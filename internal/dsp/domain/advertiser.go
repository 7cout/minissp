package domain

import (
	"errors"
	"fmt"
	"strings"
)

// Advertiser — рекламодатель, владелец кампаний.
// Хранит баланс, который тратится на показы.
type Advertiser struct {
	ID      string
	Name    string
	Balance int64 // в микроединицах
}

// Validate проверяет инварианты рекламодателя.
func (a Advertiser) Validate() error {
	if strings.TrimSpace(a.ID) == "" {
		return errors.New("advertiser id is required")
	}
	if strings.TrimSpace(a.Name) == "" {
		return errors.New("advertiser name is required")
	}
	if a.Balance < 0 {
		return fmt.Errorf("advertiser balance cannot be negative, got %d", a.Balance)
	}
	return nil
}
