package app

import (
	"context"
	"fmt"

	"booking-service/internal/cache"
	"booking-service/internal/closer"
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
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

type diContainer struct {
	cfg *config.Config

	closer *closer.Closer

	db  *pgxpool.Pool
	rdb *redis.Client
	r   *echo.Echo

	userRepo     *repository.UserRepository
	laneRepo     *repository.LaneRepository
	scheduleRepo *repository.ScheduleRepository
	slotRepo     *repository.SlotRepository
	bookingRepo  *repository.BookingRepository

	slotsCache service.Cache[service.SlotCacheKey, []models.Slot]

	authService    *service.AuthService
	userService    *service.UserService
	laneService    *service.LaneService
	bookingService *service.BookingService
	sg             *service.SlotGenerator

	handler *handler.Handler

	log *zap.Logger
}

func newDIContainer(cfg *config.Config, log *zap.Logger) *diContainer {
	return &diContainer{
		cfg:    cfg,
		closer: closer.New(),
		log:    log,
	}
}

func (c *diContainer) Database() *pgxpool.Pool {
	if c.db == nil {
		db, err := database.ConnectWithConfig(context.Background(), c.cfg.Database)
		if err != nil {
			c.log.Fatal("failed to connect to database", zap.Error(err))
		}
		c.db = db

		c.closer.Add("Database", func(ctx context.Context) error {
			c.db.Close()
			return nil
		})
	}
	return c.db
}

func (c *diContainer) UserRepository() *repository.UserRepository {
	if c.userRepo == nil {
		c.userRepo = repository.NewUserRepository(c.Database(), trmpgx.DefaultCtxGetter)
	}
	return c.userRepo
}

func (c *diContainer) LaneRepository() *repository.LaneRepository {
	if c.laneRepo == nil {
		c.laneRepo = repository.NewLaneRepository(c.Database(), trmpgx.DefaultCtxGetter)
	}
	return c.laneRepo
}

func (c *diContainer) ScheduleRepository() *repository.ScheduleRepository {
	if c.scheduleRepo == nil {
		c.scheduleRepo = repository.NewScheduleRepository(c.Database(), trmpgx.DefaultCtxGetter)
	}
	return c.scheduleRepo
}

func (c *diContainer) SlotRepository() *repository.SlotRepository {
	if c.slotRepo == nil {
		c.slotRepo = repository.NewSlotRepository(c.Database(), trmpgx.DefaultCtxGetter)
	}
	return c.slotRepo
}

func (c *diContainer) BookingRepository() *repository.BookingRepository {
	if c.bookingRepo == nil {
		c.bookingRepo = repository.NewBookingRepository(c.Database(), trmpgx.DefaultCtxGetter)
	}
	return c.bookingRepo
}

func (c *diContainer) Redis() *redis.Client {
	if c.rdb == nil {
		c.rdb = redis.NewClient(&redis.Options{
			Addr:     fmt.Sprintf("%s:%d", c.cfg.Redis.Host, c.cfg.Redis.Port),
			Password: c.cfg.Redis.Password,
			DB:       c.cfg.Redis.DB,
		})
		c.closer.Add("Redis", func(ctx context.Context) error {
			return c.rdb.Close()
		})
	}
	return c.rdb
}

func (c *diContainer) Cache() service.Cache[service.SlotCacheKey, []models.Slot] {
	if c.slotsCache == nil {
		c.slotsCache = cache.NewRedisCache[service.SlotCacheKey, []models.Slot](
			c.Redis(),
			c.cfg.Redis.Duration,
			func(k service.SlotCacheKey) string {
				return fmt.Sprintf("slots:%s:%s", k.LaneID, k.Date)
			},
		)
	}
	return c.slotsCache
}

func (c *diContainer) AuthService() *service.AuthService {
	if c.authService == nil {
		c.authService = service.NewAuthService(
			c.cfg.App.Services.Auth.SecretKey,
			c.cfg.App.Services.Auth.TokenDuration,
		)
	}
	return c.authService
}

func (c *diContainer) UserService() *service.UserService {
	if c.userService == nil {
		c.userService = service.NewUserService(
			c.UserRepository(),
			c.cfg.App.Services.PasswordCost,
		)
	}
	return c.userService
}

func (c *diContainer) LaneService() *service.LaneService {
	if c.laneService == nil {
		c.laneService = service.NewLaneService(
			c.LaneRepository(),
			c.SlotRepository(),
			c.ScheduleRepository(),
			c.cfg.App.Services.SlotDuration,
			manager.Must(trmpgx.NewDefaultFactory(c.Database())),
			c.Cache(),
			14,
		)
	}
	return c.laneService
}

func (c *diContainer) BookingService() *service.BookingService {
	if c.bookingService == nil {
		c.bookingService = service.NewBookingService(
			c.BookingRepository(),
			c.SlotRepository(),
			manager.Must(trmpgx.NewDefaultFactory(c.Database())),
			c.LaneService(),
			c.cfg.App.Services.Conference.RequestTimeout,
		)
	}
	return c.bookingService
}

func (c *diContainer) SlotGenerator() *service.SlotGenerator {
	if c.sg == nil {
		c.sg = service.NewSlotGenerator(
			c.LaneRepository(),
			c.ScheduleRepository(),
			c.SlotRepository(),
			c.cfg.App.Services.SlotDuration,
			c.cfg.App.Services.SlotGenerator.Interval,
			c.cfg.App.Services.SlotGenerator.LookAhead,
		)
	}
	return c.sg
}

func (c *diContainer) EchoServer() *echo.Echo {
	if c.r == nil {
		c.r = echo.New()
		c.closer.Add("Echo server", func(ctx context.Context) error {
			ctx, cancel := context.WithTimeout(ctx, c.cfg.App.ShutdownTimeout)
			defer cancel()
			return c.r.Shutdown(ctx)
		})
	}
	return c.r
}

func (c *diContainer) Handler() *handler.Handler {
	if c.handler == nil {
		c.handler = handler.NewHandler(
			c.BookingService(),
			c.LaneService(),
			c.UserService(),
			c.AuthService(),
			c.cfg.App.Page.Size.Default,
			c.cfg.App.Page.Size.Max,
			c.cfg.App.Page.Size.Min,
			c.cfg.App.Page.Default,
			c.cfg.App.Page.Min,
		)
	}
	return c.handler
}

func (c *diContainer) Close(ctx context.Context) error {
	return c.closer.CloseAll(ctx)
}
