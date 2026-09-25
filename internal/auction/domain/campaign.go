package domain

import (
	"errors"
	"fmt"
	"strings"
)

// Campaign — рекламная кампания...
type Campaign struct {
	ID              string
	AdvertiserID    string
	Name            string
	BudgetTotal     int64
	BudgetRemaining int64
	BudgetReserved  int64
	GeoTarget       string
}

// Validate проверяет инварианты кампании
func (c Campaign) Validate() error {
	// Идентификация
	if strings.TrimSpace(c.ID) == "" {
		return errors.New("campaign id is required")
	}
	if strings.TrimSpace(c.AdvertiserID) == "" {
		return errors.New("campaign advertiser id is required")
	}
	if strings.TrimSpace(c.Name) == "" {
		return errors.New("campaign name is required")
	}

	// Бюджет
	if c.BudgetTotal < 0 {
		return fmt.Errorf("campaign budget total cannot be negative, got %d", c.BudgetTotal)
	}
	if c.BudgetRemaining < 0 {
		return fmt.Errorf("campaign budget remaining cannot be negative, got %d", c.BudgetRemaining)
	}
	if c.BudgetReserved < 0 {
		return fmt.Errorf("campaign budget reserved cannot be negative, got %d", c.BudgetReserved)
	}
	if c.BudgetRemaining+c.BudgetReserved > c.BudgetTotal {
		return fmt.Errorf("campaign budget exceeded: remaining=%d + reserved=%d > total=%d",
			c.BudgetRemaining, c.BudgetReserved, c.BudgetTotal)
	}

	// Таргетинг и креатив
	if err := validateGeoCode(c.GeoTarget); err != nil {
		return fmt.Errorf("campaign geo target: %w", err)
	}

	return nil
}
