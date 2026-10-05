package domain

import "errors"

// Audio — параметры аудиокреатива.
// Пока пустая структура — заполним, когда понадобится.
type Audio struct{}

// Validate всегда возвращает ошибку — тип не реализован.
func (a Audio) Validate() error {
	return errors.New("audio creative not implemented yet")
}
