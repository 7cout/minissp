package domain

import (
	"errors"
	"fmt"
	"strings"
)

// BidRequest — контекст показа, который SSP передаёт DSP.
type BidRequest struct {
	RequestID string
	ImpID     string
	SlotID    string
	Geo       string
	BidFloor  int64
	UserID    string

	// Тип контента и его параметры.
	Type   CreativeType
	Banner *Banner
	Video  *Video
	Native *Native
	Audio  *Audio
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
	if err := validateGeoCode(r.Geo); err != nil {
		return fmt.Errorf("geo: %w", err)
	}
	if r.BidFloor < 0 {
		return fmt.Errorf("bid floor cannot be negative, got %d", r.BidFloor)
	}
	if err := r.validateParams(); err != nil {
		return err
	}
	return nil
}

// validateParams проверяет, что для указанного типа заполнены
// соответствующие параметры.
func (r BidRequest) validateParams() error {
	switch r.Type {
	case CreativeTypeBanner:
		if r.Banner == nil {
			return errors.New("banner type requires banner params")
		}
		return r.Banner.Validate()
	case CreativeTypeVideo:
		if r.Video == nil {
			return errors.New("video type requires video params")
		}
		return r.Video.Validate()
	case CreativeTypeNative:
		if r.Native == nil {
			return errors.New("native type requires native params")
		}
		return r.Native.Validate()
	case CreativeTypeAudio:
		if r.Audio == nil {
			return errors.New("audio type requires audio params")
		}
		return r.Audio.Validate()
	default:
		return fmt.Errorf("unsupported creative type: %q", r.Type)
	}
}

// Bid — ставка DSP. Возвращается в ответ на BidRequest.
type Bid struct {
	ID          string
	ImpID       string
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
	if strings.TrimSpace(b.ImpID) == "" {
		return errors.New("bid imp id is required")
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
