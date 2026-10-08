package domain

import (
	"errors"
	"fmt"
	"strings"
)

// Publisher — издатель, владелец рекламных слотов.
//
// APIKey — секрет, по которому SSP узнаёт publisher'а.
// В БД позже будем хранить хеш; для локального запуска — открыто.
type Publisher struct {
	ID      string
	Name    string
	APIKey  string
	Balance int64 // накопленный доход в микроединицах
}

// Validate проверяет инварианты издателя.
func (p Publisher) Validate() error {
	if strings.TrimSpace(p.ID) == "" {
		return errors.New("publisher id is required")
	}
	if strings.TrimSpace(p.Name) == "" {
		return errors.New("publisher name is required")
	}
	if strings.TrimSpace(p.APIKey) == "" {
		return errors.New("publisher api key is required")
	}
	if p.Balance < 0 {
		return fmt.Errorf("publisher balance cannot be negative, got %d", p.Balance)
	}
	return nil
}
