package domain

import (
	"errors"
	"fmt"
	"strings"
)

// Slot — рекламное место в приложении издателя.
type Slot struct {
	ID          string
	PublisherID string
	Name        string
	Banner      Banner
	Geo         string
	MinPrice    int64
}

// Validate проверяет инварианты слота.
func (s Slot) Validate() error {
	if strings.TrimSpace(s.ID) == "" {
		return errors.New("slot id is required")
	}
	if strings.TrimSpace(s.PublisherID) == "" {
		return errors.New("slot publisher id is required")
	}
	if strings.TrimSpace(s.Name) == "" {
		return errors.New("slot name is required")
	}

	if err := s.Banner.Validate(); err != nil {
		return fmt.Errorf("slot banner: %w", err)
	}

	if err := validateGeoCode(s.Geo); err != nil {
		return fmt.Errorf("slot geo: %w", err)
	}

	if s.MinPrice < 0 {
		return fmt.Errorf("slot min price cannot be negative, got %d", s.MinPrice)
	}

	return nil
}
