package app

import (
	"context"
	"fmt"

	"booking-service/internal/api"
	"booking-service/internal/cache"
	"booking-service/internal/config"
	"booking-service/internal/database"
	"booking-service/internal/handler"
	"booking-service/internal/models"
	"booking-service/internal/repository"
	"booking-service/internal/service"

	trmpgx "github.com/avito-tech/go-transaction-manager/drivers/pgxv5/v2"
	"github.com/avito-tech/go-transaction-manager/trm/v2/manager"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/redis/go-redis/v9"
	echoSwagger "github.com/swaggo/echo-swagger"
	"go.uber.org/zap"

	_ "booking-service/internal/docs"
)

type App struct {
	cfg *config.Config

	db  *pgxpool.Pool
	rdb *redis.Client
	r   *echo.Echo
	sg  *service.SlotGenerator

	log zap.Logger
}

func NewApp(ctx context.Context, cfg *config.Config, log *zap.Logger) *App {
	db, err := database.ConnectWithConfig(ctx, cfg.Database)
	if err != nil {
		log.Fatal("failed to connect to database", zap.Error(err))
	}

	userRepo := repository.NewUserRepository(db, trmpgx.DefaultCtxGetter)
	roomRepo := repository.NewLaneRepository(db, trmpgx.DefaultCtxGetter)
	scheduleRepo := repository.NewScheduleRepository(db, trmpgx.DefaultCtxGetter)
	slotRepo := repository.NewSlotRepository(db, trmpgx.DefaultCtxGetter)
	bookingRepo := repository.NewBookingRepository(db, trmpgx.DefaultCtxGetter)

	rdb := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%d", cfg.Redis.Host, cfg.Redis.Port),
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})

	redisSlotsCache := cache.NewRedisCache[service.SlotCacheKey, []models.Slot](
		rdb,
		cfg.Redis.Duration,
		slotCacheKeyFormater,
	)

	authService := service.NewAuthService(
		cfg.App.Services.Auth.SecretKey,
		cfg.App.Services.Auth.TokenDuration,
	)
	userService := service.NewUserService(userRepo, cfg.App.Services.PasswordCost)
	roomService := service.NewLaneService(
		roomRepo,
		slotRepo,
		scheduleRepo,
		cfg.App.Services.SlotDuration,
		manager.Must(trmpgx.NewDefaultFactory(db)),
		redisSlotsCache,
		14,
	)
	bookingService := service.NewBookingService(
		bookingRepo,
		slotRepo,
		manager.Must(trmpgx.NewDefaultFactory(db)),
		roomService,
		cfg.App.Services.Conference.RequestTimeout,
	)

	slotGenerator := service.NewSlotGenerator(
		roomRepo,
		scheduleRepo,
		slotRepo,
		cfg.App.Services.SlotDuration,
		cfg.App.Services.SlotGenerator.Interval,
		cfg.App.Services.SlotGenerator.LookAhead,
	)

	bookingServiceHandler := handler.NewHandler(
		bookingService,
		roomService,
		userService,
		authService,
		cfg.App.Page.Size.Default,
		cfg.App.Page.Size.Max,
		cfg.App.Page.Size.Min,
		cfg.App.Page.Default,
		cfg.App.Page.Min,
	)

	r := echo.New()

	r.Use(middleware.RequestID())
	r.Use(handler.LoggerMiddleware(log))
	r.Use(handler.JWTMiddleware(authService))

	strictHandler := api.NewStrictHandler(
		bookingServiceHandler,
		[]api.StrictMiddlewareFunc{
			handler.StrictLoggerMiddleware(log),
			handler.StrictJWTMiddleware(authService),
		})

	r.GET("/swagger/*", echoSwagger.WrapHandler)

	api.RegisterHandlers(r, strictHandler)

	r.GET("/_info", handler.Info)

	return &App{
		cfg: cfg,

		r:   r,
		db:  db,
		rdb: rdb,
		sg:  slotGenerator,

		log: *log,
	}
}

func (a *App) Run(ctx context.Context) error {
	go a.sg.Run(ctx)

	go func() {
		if err := a.r.Start(fmt.Sprintf(":%d", a.cfg.App.Port)); err != nil && err.Error() != "http: Server closed" {
			a.log.Fatal("failed to start server", zap.Error(err))
		}
	}()

	<-ctx.Done()

	return a.Shutdown()
}

func (a *App) Shutdown() error {
	ctx, cancel := context.WithTimeout(context.Background(), a.cfg.App.ShutdownTimeout)
	defer cancel()

	if err := a.r.Shutdown(ctx); err != nil {
		a.log.Fatal("failed to shutdown echo server", zap.Error(err))
		return err
	}

	if err := a.rdb.Close(); err != nil {
		a.log.Fatal("failed to close redis client", zap.Error(err))
		return err
	}

	a.db.Close()

	return nil
}
