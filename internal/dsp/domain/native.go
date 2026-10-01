package domain

import "errors"

// Native — параметры нативного креатива.
// Пока пустая структура — заполним, когда понадобится.
type Native struct {
	// Поля появятся позже: Title, Description, IconURL, ImageURL.
}

// Validate - проверяет инварианты нативного креатива
func (n Native) Validate() error {
	return errors.New("native creative not implemented yet")
}
