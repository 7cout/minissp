package domain

import (
	"errors"
	"fmt"
	"strings"
)

// Imp — описание одного показа в рамках аукциона.
// Формируется SSP из слота и передаётся DSP.
type Imp struct {
	ID       string
	SlotID   string
	BidFloor int64

	Type   CreativeType
	Banner *Banner
	Video  *Video
	Native *Native
	Audio  *Audio
}

// Validate проверяет инварианты показа.
func (i Imp) Validate() error {
	if strings.TrimSpace(i.ID) == "" {
		return errors.New("imp id is required")
	}
	if strings.TrimSpace(i.SlotID) == "" {
		return errors.New("imp slot id is required")
	}
	if i.BidFloor < 0 {
		return fmt.Errorf("imp bid floor cannot be negative, got %d", i.BidFloor)
	}
	if err := i.validateParams(); err != nil {
		return err
	}
	return nil
}

// validateParams проверяет, что для указанного типа заполнены
// соответствующие параметры.
func (i Imp) validateParams() error {
	switch i.Type {
	case CreativeTypeBanner:
		if i.Banner == nil {
			return errors.New("banner type requires banner params")
		}
		if err := i.Banner.Validate(); err != nil {
			return fmt.Errorf("imp banner: %w", err)
		}
	case CreativeTypeVideo:
		if i.Video == nil {
			return errors.New("video type requires video params")
		}
		if err := i.Video.Validate(); err != nil {
			return fmt.Errorf("imp video: %w", err)
		}
	case CreativeTypeNative:
		if i.Native == nil {
			return errors.New("native type requires native params")
		}
		if err := i.Native.Validate(); err != nil {
			return fmt.Errorf("imp native: %w", err)
		}
	case CreativeTypeAudio:
		if i.Audio == nil {
			return errors.New("audio type requires audio params")
		}
		if err := i.Audio.Validate(); err != nil {
			return fmt.Errorf("imp audio: %w", err)
		}
	default:
		return fmt.Errorf("unsupported creative type: %q", i.Type)
	}
	return nil
}
