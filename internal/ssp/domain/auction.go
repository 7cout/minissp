package domain

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// BidRequest — запрос от Publisher на проведение аукциона.
//
// Publisher НЕ присылает параметры слота — SSP знает их из БД.
type BidRequest struct {
	RequestID string
	SlotID    string
	UserID    string
}

// Validate проверяет инварианты запроса.
func (r BidRequest) Validate() error {
	if strings.TrimSpace(r.RequestID) == "" {
		return errors.New("request id is required")
	}
	if strings.TrimSpace(r.SlotID) == "" {
		return errors.New("slot id is required")
	}
	return nil
}

// AuctionResult — результат аукциона для Publisher.
//
// Publisher получает только то, что нужно для показа баннера.
// Внутренние детали (campaign_id, price) хранятся в AuctionRecord.
type AuctionResult struct {
	AuctionID   string
	CreativeURL string
	ClickURL    string
}

// AuctionRecord — внутренняя запись аукциона, которую SSP хранит
// до момента Impression или истечения TTL.
type AuctionRecord struct {
	AuctionID   string
	ImpID       string
	CampaignID  string
	CreativeID  string
	PublisherID string
	SlotID      string
	Price       int64
	CreatedAt   time.Time
}

// Validate проверяет инварианты записи.
func (r AuctionRecord) Validate() error {
	if strings.TrimSpace(r.AuctionID) == "" {
		return errors.New("auction id is required")
	}
	if strings.TrimSpace(r.CampaignID) == "" {
		return errors.New("campaign id is required")
	}
	if strings.TrimSpace(r.PublisherID) == "" {
		return errors.New("publisher id is required")
	}
	if r.Price <= 0 {
		return fmt.Errorf("price must be positive, got %d", r.Price)
	}
	return nil
}
