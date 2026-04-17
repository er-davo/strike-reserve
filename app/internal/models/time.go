package models

import (
	"database/sql"
	"database/sql/driver"
	"fmt"
	"time"
)

// HMSTime is a custom type for storing time in HH:MM:SS format
type HMSTime string

// FromHMToHMS converts HH:MM to HH:MM:SS format
func FromHMToHMS(hm string) HMSTime {
	return HMSTime(hm + ":00")
}

var _ sql.Scanner = (*HMSTime)(nil)
var _ driver.Valuer = (*HMSTime)(nil)

// Time returns the time.Time representation of the HMSTime
func (t HMSTime) Time() (time.Time, error) {
	return time.Parse("15:04:05", string(t))
}

func (t HMSTime) HMTime() string {
	return string(t)[:5]
}

// sql.Scanner implementation
func (h *HMSTime) Scan(value interface{}) error {
	if value == nil {
		*h = ""
		return nil
	}

	switch v := value.(type) {
	case time.Time:
		*h = HMSTime(v.Format("15:04:05"))
		return nil
	case string:
		if len(v) > 8 {
			*h = HMSTime(v[:8])
		} else {
			*h = HMSTime(v)
		}
		return nil
	case []byte:
		vStr := string(v)
		if len(vStr) > 8 {
			*h = HMSTime(vStr[:8])
		} else {
			*h = HMSTime(vStr)
		}
		return nil
	default:
		return fmt.Errorf("cannot scan %T into HMSTime", value)
	}
}

// driver.Valuer implementation
func (ct HMSTime) Value() (driver.Value, error) {
	if ct == "" {
		return nil, nil
	}
	return string(ct), nil
}
