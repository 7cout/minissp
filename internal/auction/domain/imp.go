package domain

import (
	"errors"
	"fmt"
	"strings"
)

// Imp — описание одного показа в рамках BidRequest.
// создаётся при формировании запроса и не хранится в БД.
//
// В отличие от Slot, Imp знает только про этот конкретный показ.
type Imp struct {
	ID       string
	SlotID   string
	Banner   Banner
	BidFloor int64
}

// Validate проверяет инварианты показа.
func (i Imp) Validate() error {
	if strings.TrimSpace(i.ID) == "" {
		return errors.New("imp id is required")
	}
	if strings.TrimSpace(i.SlotID) == "" {
		return errors.New("imp slot id is required")
	}
	if err := i.Banner.Validate(); err != nil {
		return fmt.Errorf("slot banner: %w", err)
	}
	if i.BidFloor < 0 {
		return fmt.Errorf("imp bid floor cannot be negative, got %d", i.BidFloor)
	}
	return nil
}
