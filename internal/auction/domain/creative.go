package domain

import (
	"errors"
	"fmt"
	"strings"
)

// CreativeType тип рекламного материала
type CreativeType string

// Поддерживаемые типы креативов.
const (
	CreativeTypeImage CreativeType = "image"
)

// Creative рекламный материал. Картинка, видео,
// текст, аудио, ссылка на сайт
type Creative struct {
	ID         string
	CampaignID string
	Type       CreativeType
	Width      int
	Height     int
	URL        string
	ClickURL   string
}

// Validate проверяет инварианты
func (c Creative) Validate() error {
	if strings.TrimSpace(c.ID) == "" {
		return errors.New("creative id is required")
	}
	if strings.TrimSpace(c.CampaignID) == "" {
		return errors.New("creative campaign id is required")
	}

	if c.Type != CreativeTypeImage {
		return fmt.Errorf("unsupported creative type: %q", c.Type)
	}

	if c.Width <= 0 {
		return fmt.Errorf("creative width must be positive, got %d", c.Width)
	}
	if c.Height <= 0 {
		return fmt.Errorf("creative height must be positive, got %d", c.Height)
	}

	if err := validateURL(c.URL); err != nil {
		return fmt.Errorf("creative url: %w", err)
	}
	if err := validateURL(c.ClickURL); err != nil {
		return fmt.Errorf("creative click url: %w", err)
	}

	return nil
}
