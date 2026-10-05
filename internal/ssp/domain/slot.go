package domain

import (
	"errors"
	"fmt"
	"strings"
)

// Slot — рекламное место в приложении издателя.
//
// Тип слота определяет, какое из вложенных полей заполнено:
// Banner, Video, Native или Audio.
type Slot struct {
	ID          string
	PublisherID string
	Name        string
	Geo         string
	MinPrice    int64

	Type   CreativeType
	Banner *Banner
	Video  *Video
	Native *Native
	Audio  *Audio
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
	if err := validateGeoCode(s.Geo); err != nil {
		return fmt.Errorf("slot geo: %w", err)
	}
	if s.MinPrice < 0 {
		return fmt.Errorf("slot min price cannot be negative, got %d", s.MinPrice)
	}
	if err := s.validateParams(); err != nil {
		return err
	}
	return nil
}

// validateParams проверяет, что для указанного типа заполнены
// соответствующие параметры.
func (s Slot) validateParams() error {
	switch s.Type {
	case CreativeTypeBanner:
		if s.Banner == nil {
			return errors.New("banner type requires banner params")
		}
		if err := s.Banner.Validate(); err != nil {
			return fmt.Errorf("slot banner: %w", err)
		}
	case CreativeTypeVideo:
		if s.Video == nil {
			return errors.New("video type requires video params")
		}
		if err := s.Video.Validate(); err != nil {
			return fmt.Errorf("slot video: %w", err)
		}
	case CreativeTypeNative:
		if s.Native == nil {
			return errors.New("native type requires native params")
		}
		if err := s.Native.Validate(); err != nil {
			return fmt.Errorf("slot native: %w", err)
		}
	case CreativeTypeAudio:
		if s.Audio == nil {
			return errors.New("audio type requires audio params")
		}
		if err := s.Audio.Validate(); err != nil {
			return fmt.Errorf("slot audio: %w", err)
		}
	default:
		return fmt.Errorf("unsupported creative type: %q", s.Type)
	}
	return nil
}
