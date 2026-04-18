package app

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"booking-service/internal/api"
	"booking-service/internal/config"
	"booking-service/internal/handler"

	"github.com/labstack/echo/v4/middleware"
	echoSwagger "github.com/swaggo/echo-swagger"
	"go.uber.org/zap"

	_ "booking-service/internal/docs"
)

type App struct {
	cfg *config.Config
	di  *diContainer
	log zap.Logger
}

func NewApp(ctx context.Context, cfg *config.Config, log *zap.Logger) *App {
	di := newDIContainer(cfg, log)

	strictHandler := api.NewStrictHandler(
		di.Handler(),
		[]api.StrictMiddlewareFunc{},
	)

	r := di.EchoServer()

	r.Use(middleware.RequestID())
	r.Use(handler.LoggerMiddleware(log))
	r.Use(handler.JWTMiddleware(di.AuthService()))

	r.GET("/swagger/*", echoSwagger.WrapHandler)

	api.RegisterHandlers(r, strictHandler)

	r.GET("/_info", handler.Info)

	return &App{
		cfg: cfg,
		di:  di,
		log: *log,
	}
}

func (a *App) Run(ctx context.Context) error {
	go a.di.SlotGenerator().Run(ctx)

	go func() {
		if err := a.di.EchoServer().Start(fmt.Sprintf(":%d", a.cfg.App.Port)); err != nil && !errors.Is(err, http.ErrServerClosed) {
			a.log.Fatal("failed to start server", zap.Error(err))
		}
	}()

	<-ctx.Done()

	return a.Shutdown()
}

func (a *App) Shutdown() error {
	a.log.Info("shutting down")
	ctx, cancel := context.WithTimeout(context.Background(), a.cfg.App.ShutdownTimeout)
	defer cancel()

	if err := a.di.Close(ctx); err != nil {
		a.log.Error("failed to close resources", zap.Error(err))
		return err
	}

	a.log.Info("shutdown complete")

	return nil
}
