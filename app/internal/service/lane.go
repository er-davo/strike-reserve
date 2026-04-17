package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"booking-service/internal/models"
	"booking-service/internal/repository"
	"booking-service/pkg/logger"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

type SlotCacheKey struct {
	LaneID uuid.UUID
	Date   string
}

type LaneService struct {
	laneRepo     LaneRepository
	scheduleRepo ScheduleRepository
	slotRepo     SlotRepository
	slotDuration time.Duration

	trManager TxManager

	cache              Cache[SlotCacheKey, []models.Slot]
	slotGenerationDays int
}

func NewLaneService(
	laneRepo LaneRepository,
	slotRepo SlotRepository,
	scheduleRepo ScheduleRepository,
	slotDuration time.Duration,
	trm TxManager,
	cache Cache[SlotCacheKey, []models.Slot],
	slotGenerationDays int,
) *LaneService {
	return &LaneService{
		laneRepo:           laneRepo,
		slotRepo:           slotRepo,
		scheduleRepo:       scheduleRepo,
		slotDuration:       slotDuration,
		trManager:          trm,
		cache:              cache,
		slotGenerationDays: slotGenerationDays,
	}
}

func (s *LaneService) CreateLane(ctx context.Context, lCreate models.LaneCreate) (*models.Lane, error) {
	l := logger.FromContext(ctx)
	l.Info("creating new lane",
		zap.String("name", lCreate.Name),
		zap.String("type", string(lCreate.Type)),
	)

	lane := &models.Lane{
		ID:          uuid.New(),
		Name:        lCreate.Name,
		Description: lCreate.Description,
		Type:        lCreate.Type,
	}

	if err := s.laneRepo.Create(ctx, lane); err != nil {
		l.Error("failed to create lane", zap.Error(err), zap.String("lane_name", lCreate.Name))
		return nil, err
	}

	l.Info("lane created successfully", zap.String("lane_id", lane.ID.String()))
	return lane, nil
}

func (s *LaneService) GetAllLanes(ctx context.Context) ([]models.Lane, error) {
	logger.FromContext(ctx).Debug("fetching all lanes")

	lanes, err := s.laneRepo.GetAll(ctx)
	if err != nil {
		logger.FromContext(ctx).Error("failed to get all lanes", zap.Error(err))
		return nil, err
	}
	return lanes, nil
}

func (s *LaneService) GetAvailableSlots(ctx context.Context, laneID uuid.UUID, date time.Time) ([]models.Slot, error) {
	l := logger.FromContext(ctx)
	l.Debug("fetching available slots",
		zap.String("lane_id", laneID.String()),
		zap.Time("date", date),
	)

	cacheKey := SlotCacheKey{
		LaneID: laneID,
		Date:   date.Format("2006-01-02"),
	}

	if val, ok := s.cache.Get(ctx, cacheKey); ok {
		return val, nil
	}

	_, err := s.laneRepo.Get(ctx, laneID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			l.Warn("get slots failed: lane not found", zap.String("lane_id", laneID.String()))
			return nil, ErrLaneNotFound
		}
		l.Error("failed to check lane existence", zap.Error(err), zap.String("lane_id", laneID.String()))
		return nil, err
	}

	slots, err := s.slotRepo.GetAvailableByLaneAndDate(ctx, laneID, date)
	if err != nil {
		l.Error("failed to get available slots",
			zap.Error(err),
			zap.String("lane_id", laneID.String()),
			zap.Time("date", date),
		)
		return nil, err
	}

	s.cache.Set(ctx, cacheKey, slots)
	return slots, nil
}

func (s *LaneService) CreateSchedule(ctx context.Context, sch *models.Schedule) error {
	l := logger.FromContext(ctx)
	l.Info("attempting to create schedule", zap.String("lane_id", sch.LaneID.String()))

	return s.trManager.Do(ctx, func(ctx context.Context) error {
		txLog := logger.FromContext(ctx)

		_, err := s.laneRepo.Get(ctx, sch.LaneID)
		if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				txLog.Warn("schedule creation failed: lane not found", zap.String("lane_id", sch.LaneID.String()))
				return ErrLaneNotFound
			}
			return err
		}

		existingSchedules, err := s.scheduleRepo.GetByLaneID(ctx, sch.LaneID)
		if err != nil && !errors.Is(err, repository.ErrNotFound) {
			return err
		}
		if existingSchedules != nil {
			txLog.Warn("schedule already exists for lane", zap.String("lane_id", sch.LaneID.String()))
			return ErrAlreadyExists
		}

		if err := s.scheduleRepo.Create(ctx, sch); err != nil {
			txLog.Error("failed to save schedule record", zap.Error(err))
			return err
		}

		txLog.Debug("starting slot generation for schedule")

		startTime, _ := sch.StartTime.Time()
		endTime, _ := sch.EndTime.Time()

		allowedDays := [7]bool{}
		for _, day := range sch.DaysOfWeek {
			allowedDays[day%7] = true
		}

		now := time.Now().UTC()
		endDate := now.AddDate(0, 0, s.slotGenerationDays)
		var slots []models.Slot

		for day := now; day.Before(endDate); day = day.AddDate(0, 0, 1) {
			if allowedDays[int(day.Weekday())] {
				curStart := time.Date(day.Year(), day.Month(), day.Day(), startTime.Hour(), startTime.Minute(), 0, 0, time.UTC)
				curEnd := time.Date(day.Year(), day.Month(), day.Day(), endTime.Hour(), endTime.Minute(), 0, 0, time.UTC)

				for t := curStart; !t.Add(s.slotDuration).After(curEnd); t = t.Add(s.slotDuration) {
					slots = append(slots, models.Slot{
						ID:        uuid.New(),
						LaneID:    sch.LaneID,
						StartTime: t,
						EndTime:   t.Add(s.slotDuration),
					})
				}
			}
		}

		if len(slots) == 0 {
			txLog.Error("no slots generated",
				zap.String("lane_id", sch.LaneID.String()),
				zap.Any("days", sch.DaysOfWeek),
			)
			return fmt.Errorf("no slots generated for schedule")
		}

		if err := s.slotRepo.CreateSlots(ctx, slots); err != nil {
			txLog.Error("failed to bulk insert slots", zap.Error(err), zap.Int("count", len(slots)))
			return err
		}

		txLog.Info("schedule and slots created successfully",
			zap.String("lane_id", sch.LaneID.String()),
			zap.Int("slots_count", len(slots)),
		)
		return nil
	})
}

func (s *LaneService) InvalidateSlotsCache(ctx context.Context, laneID uuid.UUID, date time.Time) {
	s.cache.Delete(ctx, SlotCacheKey{
		LaneID: laneID,
		Date:   date.Format("2006-01-02"),
	})
}
