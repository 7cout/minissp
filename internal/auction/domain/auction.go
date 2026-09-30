package domain

import (
	"errors"
	"fmt"
	"strings"
)

// BidRequest — запрос на проведение аукциона
type BidRequest struct {
	ID     string
	Imp    Imp
	UserID string
}

// Validate проверяет инварианты запроса
func (r BidRequest) Validate() error {
	if strings.TrimSpace(r.ID) == "" {
		return errors.New("bid request id is required")
	}
	if err := r.Imp.Validate(); err != nil {
		return fmt.Errorf("bid request imp: %w", err)
	}
	return nil
}

// AuctionResult — результат аукциона: кто победил и по какой цене
type AuctionResult struct {
	AuctionID  string
	ImpID      string
	BidID      string
	CampaignID string
	CreativeID string
	Price      int64
}
