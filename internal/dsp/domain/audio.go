package domain

import "errors"

// Audio — параметры аудиокреатива.
type Audio struct {
	Duration int
	MIMEs    []string
}

// Validate - проверяет инварианты аудио креатива
func (a Audio) Validate() error {
	return errors.New("audio creative not implemented yet")
}
