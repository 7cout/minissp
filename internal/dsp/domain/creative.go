package domain

import (
	"errors"
	"fmt"
	"strings"
)

// CreativeType — тип рекламного материала.
type CreativeType string

// Поддерживаемые типы креативов.
const (
	CreativeTypeBanner CreativeType = "banner"
	CreativeTypeVideo  CreativeType = "video"
	CreativeTypeNative CreativeType = "native"
	CreativeTypeAudio  CreativeType = "audio"
)

// Creative — рекламный материал. Тип определяет, какое поле заполнено.
type Creative struct {
	ID         string
	CampaignID string
	Type       CreativeType
	URL        string
	ClickURL   string

	Banner *Banner // для Type=banner
	Video  *Video  // для Type=video
	Native *Native // для Type=native
	Audio  *Audio  // для Type=audio
}

// Validate проверяет инварианты креатива.
func (c Creative) Validate() error {
	if strings.TrimSpace(c.ID) == "" {
		return errors.New("creative id is required")
	}
	if strings.TrimSpace(c.CampaignID) == "" {
		return errors.New("creative campaign id is required")
	}
	if err := validateURL(c.URL); err != nil {
		return fmt.Errorf("creative url: %w", err)
	}
	if err := validateURL(c.ClickURL); err != nil {
		return fmt.Errorf("creative click url: %w", err)
	}

	// Тип определяет, какое вложенное поле должно быть заполнено.
	switch c.Type {
	case CreativeTypeBanner:
		if c.Banner == nil {
			return errors.New("banner creative requires banner parameters")
		}
		if err := c.Banner.Validate(); err != nil {
			return fmt.Errorf("creative banner: %w", err)
		}
	case CreativeTypeVideo:
		if c.Video == nil {
			return errors.New("video creative requires video parameters")
		}
		if err := c.Video.Validate(); err != nil {
			return fmt.Errorf("creative video: %w", err)
		}
	case CreativeTypeNative:
		if c.Native == nil {
			return errors.New("native creative requires native parameters")
		}
		if err := c.Native.Validate(); err != nil {
			return fmt.Errorf("creative native: %w", err)
		}
	case CreativeTypeAudio:
		if c.Audio == nil {
			return errors.New("audio creative requires audio parameters")
		}
		if err := c.Audio.Validate(); err != nil {
			return fmt.Errorf("creative audio: %w", err)
		}
	default:
		return fmt.Errorf("unsupported creative type: %q", c.Type)
	}
	return nil
}

// Clone возвращает глубокую копию креатива.
//
// Копирует вложенные структуры (Banner, Video, Native, Audio) и слайс
// MIMEs. Нужен, чтобы репозитории не отдавали наружу указатели на
// собственное хранилище.
func (c Creative) Clone() Creative {
	cp := c

	if c.Banner != nil {
		b := *c.Banner
		cp.Banner = &b
	}
	if c.Video != nil {
		v := *c.Video
		if c.Video.MIMEs != nil {
			v.MIMEs = append([]string(nil), c.Video.MIMEs...)
		}
		cp.Video = &v
	}
	if c.Native != nil {
		n := *c.Native
		cp.Native = &n
	}
	if c.Audio != nil {
		a := *c.Audio
		cp.Audio = &a
	}

	return cp
}
