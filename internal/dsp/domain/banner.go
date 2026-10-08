package domain

import "fmt"

// Banner — параметры банннера
type Banner struct {
	Width  int
	Height int
}

// Validate проверяет инварианты баннера.
func (b Banner) Validate() error {
	if b.Width <= 0 {
		return fmt.Errorf("banner width must be positive, got %d", b.Width)
	}
	if b.Height <= 0 {
		return fmt.Errorf("banner height must be positive, got %d", b.Height)
	}
	return nil
}
