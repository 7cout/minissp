package domain

import (
	"errors"
	"fmt"
	"strings"
)

// BidRequest — контекст показа, который SSP передаёт DSP.
// DSP решает, участвовать ли, и по какой цене.
type BidRequest struct {
	RequestID string // ID запроса от Publisher
	ImpID     string // ID показа
	SlotID    string // ID слота в SSP
	Width     int32  // ширина слота
	Height    int32  // высота слота
	Geo       string // гео пользователя
	BidFloor  int64  // минимальная цена в микроединицах
	UserID    string // опционально
}

// Validate проверяет инварианты запроса.
func (r BidRequest) Validate() error {
	if strings.TrimSpace(r.RequestID) == "" {
		return errors.New("request id is required")
	}
	if strings.TrimSpace(r.ImpID) == "" {
		return errors.New("imp id is required")
	}
	if strings.TrimSpace(r.SlotID) == "" {
		return errors.New("slot id is required")
	}
	if r.Width <= 0 {
		return fmt.Errorf("width must be positive, got %d", r.Width)
	}
	if r.Height <= 0 {
		return fmt.Errorf("height must be positive, got %d", r.Height)
	}
	if err := validateGeoCode(r.Geo); err != nil {
		return fmt.Errorf("geo: %w", err)
	}
	if r.BidFloor < 0 {
		return fmt.Errorf("bid floor cannot be negative, got %d", r.BidFloor)
	}
	return nil
}

// Bid — ставка DSP. Возвращается в ответ на BidRequest.
type Bid struct {
	ID          string // ID ставки
	ImpID       string // ID показа
	CampaignID  string // какая кампания
	CreativeID  string // какой креатив
	CreativeURL string // URL креатива
	ClickURL    string // URL клика
	Price       int64  // цена в микроединицах
}

// Validate проверяет инварианты ставки.
func (b Bid) Validate() error {
	if strings.TrimSpace(b.ID) == "" {
		return errors.New("bid id is required")
	}
	if strings.TrimSpace(b.ImpID) == "" {
		return errors.New("bid imp id is required")
	}
	if strings.TrimSpace(b.CampaignID) == "" {
		return errors.New("bid campaign id is required")
	}
	if strings.TrimSpace(b.CreativeID) == "" {
		return errors.New("bid creative id is required")
	}
	if b.Price <= 0 {
		return fmt.Errorf("bid price must be positive, got %d", b.Price)
	}
	if err := validateURL(b.CreativeURL); err != nil {
		return fmt.Errorf("creative url: %w", err)
	}
	if err := validateURL(b.ClickURL); err != nil {
		return fmt.Errorf("click url: %w", err)
	}
	return nil
}
