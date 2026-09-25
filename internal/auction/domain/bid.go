package domain

import (
	"errors"
	"fmt"
	"strings"
)

// Bid — ставка, которую биддер присылает в ответ на запрос аукциона.
type Bid struct {
	CampaignID string
	CreativeID string
	Price      int64
}

// Validate проверяет инварианты ставки.
func (b Bid) Validate() error {
	if strings.TrimSpace(b.CampaignID) == "" {
		return errors.New("bid campaign id is required")
	}
	if strings.TrimSpace(b.CreativeID) == "" {
		return errors.New("bid creative id is required")
	}
	if b.Price <= 0 {
		return fmt.Errorf("bid price must be positive, got %d", b.Price)
	}
	return nil
}