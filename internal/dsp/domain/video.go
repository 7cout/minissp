package domain

import (
	"errors"
	"fmt"
)

// Video — параметры видеокреатива.
type Video struct {
	Width    int
	Height   int
	Duration int      // в секундах
	MIMEs    []string // ["video/mp4", "video/webm"]
}

// Validate - проверяет инварианты видеокреатива
func (v Video) Validate() error {
	if v.Width <= 0 {
		return fmt.Errorf("video width must be positive, got %d", v.Width)
	}
	if v.Height <= 0 {
		return fmt.Errorf("video height must be positive, got %d", v.Height)
	}
	if v.Duration <= 0 {
		return fmt.Errorf("video duration must be positive, got %d", v.Duration)
	}
	if len(v.MIMEs) == 0 {
		return errors.New("video must have at least one MIME type")
	}
	return nil
}
