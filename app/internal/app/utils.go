package app

import (
	"fmt"

	"booking-service/internal/service"
)

func slotCacheKeyFormater(k service.SlotCacheKey) string {
	return fmt.Sprintf("slots:%s:%s", k.LaneID, k.Date)
}
