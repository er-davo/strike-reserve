package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"time"

	"booking-service/internal/cache"
	"booking-service/internal/config"
	"booking-service/internal/database"
	"booking-service/internal/models"
	"booking-service/internal/repository"
	"booking-service/internal/service"

	trmpgx "github.com/avito-tech/go-transaction-manager/drivers/pgxv5/v2"
	"github.com/avito-tech/go-transaction-manager/trm/v2/manager"
)

func main() {
	dbURLFlag := flag.String("db-url", "", "Database connection URL")
	flag.Parse()

	ctx := context.Background()
	cfg, _ := config.Load("../config.yaml")

	dbURL := *dbURLFlag
	if dbURL == "" || dbURL == "url" {
		log.Fatal("database url is not provided")
	}

	db, err := database.Connect(ctx, dbURL)
	if err != nil {
		log.Fatalf("failed to connect: %v", err)
	}
	defer db.Close()

	trManager := manager.Must(trmpgx.NewDefaultFactory(db))
	userRepo := repository.NewUserRepository(db, trmpgx.DefaultCtxGetter)
	roomRepo := repository.NewLaneRepository(db, trmpgx.DefaultCtxGetter)
	slotRepo := repository.NewSlotRepository(db, trmpgx.DefaultCtxGetter)
	scheduleRepo := repository.NewScheduleRepository(db, trmpgx.DefaultCtxGetter)
	bookingRepo := repository.NewBookingRepository(db, trmpgx.DefaultCtxGetter)

	slotsCache := cache.NewInMemoryLRUCache[service.SlotCacheKey, []models.Slot](cfg.Redis.Size, cfg.Redis.Duration)

	userService := service.NewUserService(
		userRepo,
		cfg.App.Services.PasswordCost,
	)
	roomService := service.NewLaneService(
		roomRepo,
		slotRepo,
		scheduleRepo,
		cfg.App.Services.SlotDuration,
		trManager,
		slotsCache,
		45, // на 45 дней вперед генерация слотов
	)
	bookingService := service.NewBookingService(
		bookingRepo,
		slotRepo,
		trManager,
		roomService,
		cfg.App.Services.Conference.RequestTimeout,
	)

	// Используем фиксированный сид для воспроизводимости
	rng := rand.New(rand.NewSource(31))

	fmt.Println("Starting seed process...")

	// --- ШАГ 1: Создаем пачку юзеров ---
	fmt.Println("Step 1: Creating users...")
	userCount := 500
	var users []*models.User
	for i := 0; i < userCount; i++ {
		u, _ := userService.Register(ctx, models.UserRegister{
			Email:    fmt.Sprintf("user%d@test.com", i),
			Password: "password123",
			Role:     models.RoleUser,
		})
		users = append(users, u)
	}

	// --- ШАГ 2: Создаем дорожки и расписания ---
	fmt.Println("Step 2: Creating lanes and schedules...")
	roomToCreate := 100
	var createdLanes []*models.Lane
	for i := 1; i <= roomToCreate; i++ {
		var laneType models.LaneType

		switch rng.Intn(4) {
		case 0:
			laneType = models.LaneTypeStandard
		case 1:
			laneType = models.LaneTypeKids
		case 2:
			laneType = models.LaneTypePro
		case 3:
			laneType = models.LaneTypeVip
		default:
			laneType = models.LaneTypeStandard
		}

		lane, _ := roomService.CreateLane(ctx, models.LaneCreate{
			Name:        fmt.Sprintf("Bowling lane %d", i),
			Description: ptr(fmt.Sprintf("Bowling lane description %d", i)),
			Type:        laneType,
		})

		_ = roomService.CreateSchedule(ctx, &models.Schedule{
			LaneID:     lane.ID,
			DaysOfWeek: []int{1, 2, 3, 4, 5, 6, 7}, // Включаем выходные для теста
			StartTime:  "08:00:00",
			EndTime:    "20:00:00",
		})
		createdLanes = append(createdLanes, lane)
	}

	// --- ШАГ 3: Генерация броней ---
	fmt.Println("Step 3: Generating random bookings...")
	bookingCount := 0
	targetBookings := 100_000

	for day := 0; day < 45; day++ {
		if bookingCount >= targetBookings {
			break
		}
		date := time.Now().UTC().AddDate(0, 0, day)

		for _, r := range createdLanes {
			slots, err := roomService.GetAvailableSlots(ctx, r.ID, date)
			if err != nil || len(slots) == 0 {
				continue
			}

			for _, slot := range slots {
				// 70% вероятность, что слот будет занят.
				// Оставляем 30% пустыми для тестов k6 на "успешное бронирование"
				if rng.Float32() > 0.7 {
					continue
				}

				if bookingCount >= targetBookings {
					break
				}

				// Берем случайного юзера из пачки
				randomUser := users[rng.Intn(len(users))]

				_, err := bookingService.Create(ctx, randomUser.ID, slot.ID)
				if err == nil {
					bookingCount++
					if bookingCount%1000 == 0 {
						fmt.Printf("\rProgress: %d/%d bookings", bookingCount, targetBookings)
					}
				}
			}
		}
	}

	fmt.Printf("\nDone. Lanes: %d, Users: %d, Bookings: %d\n", len(createdLanes), len(users), bookingCount)
}

func ptr[T any](v T) *T {
	return &v
}
