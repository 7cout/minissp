package domain

import (
	"errors"
	"fmt"
	"strings"
)

// Bid — ставка от DSP, полученная в ответ на BidRequest.
//
// Это внутренняя структура SSP: Publisher её не видит.
// SSP использует Bid для формирования AuctionRecord.
type Bid struct {
	ID          string
	CampaignID  string
	CreativeID  string
	CreativeURL string
	ClickURL    string
	Price       int64
}

// Validate проверяет инварианты ставки.
func (b Bid) Validate() error {
	if strings.TrimSpace(b.ID) == "" {
		return errors.New("bid id is required")
	}
	if strings.TrimSpace(b.CampaignID) == "" {
		return errors.New("bid campaign id is required")
	}
	if strings.TrimSpace(b.CreativeID) == "" {
		return errors.New("bid creative id is required")
	}
	if err := validateURL(b.CreativeURL); err != nil {
		return fmt.Errorf("bid creative url: %w", err)
	}
	if err := validateURL(b.ClickURL); err != nil {
		return fmt.Errorf("bid click url: %w", err)
	}
	if b.Price <= 0 {
		return fmt.Errorf("bid price must be positive, got %d", b.Price)
	}
	return nil
}
