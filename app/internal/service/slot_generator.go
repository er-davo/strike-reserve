package service

import (
	"booking-service/internal/models"
	"booking-service/pkg/logger"
	"context"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

type SlotGenerator struct {
	laneRepo     LaneRepository
	scheduleRepo ScheduleRepository
	slotRepo     SlotRepository
	slotDuration time.Duration
	interval     time.Duration
	lookahead    int
}

func NewSlotGenerator(
	rr LaneRepository,
	sr ScheduleRepository,
	slr SlotRepository,
	dur time.Duration,
	interval time.Duration,
	lookahead int,
) *SlotGenerator {
	return &SlotGenerator{
		laneRepo:     rr,
		scheduleRepo: sr,
		slotRepo:     slr,
		slotDuration: dur,
		interval:     interval,
		lookahead:    lookahead,
	}
}

func (g *SlotGenerator) Run(ctx context.Context) error {
	l := logger.FromContext(ctx)
	l.Info("starting background slot generator", zap.Duration("interval", g.interval))

	g.GenerateAll(ctx)

	ticker := time.NewTicker(g.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			l.Info("stopping slot generator")
			return ctx.Err()
		case <-ticker.C:
			g.GenerateAll(ctx)
		}
	}
}

func (g *SlotGenerator) GenerateAll(ctx context.Context) {
	l := logger.FromContext(ctx)

	lanes, err := g.laneRepo.GetAll(ctx)
	if err != nil {
		l.Error("failed to fetch rooms for generation", zap.Error(err))
		return
	}

	for _, lane := range lanes {
		sch, err := g.scheduleRepo.GetByLaneID(ctx, lane.ID)
		if err != nil {
			l.Error("failed to get schedule", zap.String("lane_id", lane.ID.String()), zap.Error(err))
			continue
		}
		if sch == nil {
			continue
		}

		var allLaneSlots []models.Slot
		start := time.Now().UTC()
		end := start.AddDate(0, 0, g.lookahead)

		for d := start; d.Before(end); d = d.AddDate(0, 0, 1) {
			daySlots := g.buildSlotsForDay(lane.ID, sch, d)
			allLaneSlots = append(allLaneSlots, daySlots...)
		}

		if len(allLaneSlots) > 0 {
			if err := g.slotRepo.CreateSlots(ctx, allLaneSlots); err != nil {
				l.Error("failed to save batch of slots",
					zap.String("lane_id", lane.ID.String()),
					zap.Int("count", len(allLaneSlots)),
					zap.Error(err),
				)
			} else {
				l.Debug("successfully generated slots",
					zap.String("lane_id", lane.ID.String()),
					zap.Int("count", len(allLaneSlots)),
				)
			}
		}
	}
}

func (g *SlotGenerator) buildSlotsForDay(roomID uuid.UUID, sch *models.Schedule, date time.Time) []models.Slot {
	startTime, _ := sch.StartTime.Time()
	endTime, _ := sch.EndTime.Time()

	allowed := false
	for _, d := range sch.DaysOfWeek {
		if d == int(date.Weekday()) {
			allowed = true
			break
		}
	}
	if !allowed {
		return nil
	}

	var slots []models.Slot
	curStart := time.Date(date.Year(), date.Month(), date.Day(), startTime.Hour(), startTime.Minute(), 0, 0, time.UTC)
	curEnd := time.Date(date.Year(), date.Month(), date.Day(), endTime.Hour(), endTime.Minute(), 0, 0, time.UTC)

	for t := curStart; !t.Add(g.slotDuration).After(curEnd); t = t.Add(g.slotDuration) {
		slots = append(slots, models.Slot{
			ID:        uuid.New(),
			LaneID:    roomID,
			StartTime: t,
			EndTime:   t.Add(g.slotDuration),
		})
	}
	return slots
}
