package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"booking-service/internal/app"
	"booking-service/internal/config"
	"booking-service/internal/database"
	"booking-service/pkg/logger"

	"go.uber.org/zap"

	_ "booking-service/internal/docs"
)

// @title Room Booking Service API
// @version 1.0.0
// @description Сервис бронирования переговорок
// @host localhost:8080
// @BasePath /

// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
func main() {
	configFilePath := os.Getenv("CONFIG_PATH")
	if configFilePath == "" {
		panic("env CONFIG_PATH is empty")
	}

	cfg, err := config.Load(configFilePath)
	if err != nil {
		panic("error on loading config: " + err.Error())
	}

	log := logger.NewLogger(cfg.App.LogLevel, cfg.App.IsProd)
	defer log.Sync()

	err = database.Migrate(cfg.App.MigrationDir, cfg.Database.Dsn)
	if err != nil {
		log.Sync()
		log.Fatal("error on migrating database", zap.Error(err))
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	bookingApp := app.NewApp(ctx, cfg, log)

	log.Info("strarting app")
	if err := bookingApp.Run(ctx); err != nil {
		if ctx.Err() != nil {
			log.Info("app stopped by context")
		} else {
			log.Error("app exited with error", zap.Error(err))
		}
	}
}
