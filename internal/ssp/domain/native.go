package domain

import "errors"

// Native — параметры нативного креатива.
// Пока пустая структура — заполним, когда понадобится.
type Native struct{}

// Validate всегда возвращает ошибку — тип не реализован.
func (n Native) Validate() error {
	return errors.New("native creative not implemented yet")
}
