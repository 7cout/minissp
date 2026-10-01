package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/7cout/minissp/internal/auction/domain"
)

// SlotRepo — PostgreSQL-реализация SlotRepository
type SlotRepo struct {
	db *pgxpool.Pool
}

// NewSlotRepo создаёт репозиторий слотов
func NewSlotRepo(db *pgxpool.Pool) *SlotRepo {
	return &SlotRepo{db: db}
}

// Get возвращает слот по ID
//
// Возвращает domain.ErrSlotNotFound, если слот не найден
func (r *SlotRepo) Get(ctx context.Context, id string) (*domain.Slot, error) {
	const query = `
		SELECT id, publisher_id, name, width, height, geo, min_price
		FROM ad_slots
		WHERE id = $1
	`

	var s domain.Slot
	err := r.db.QueryRow(ctx, query, id).Scan(
		&s.ID,
		&s.PublisherID,
		&s.Name,
		&s.Banner.Width,
		&s.Banner.Height,
		&s.Geo,
		&s.MinPrice,
	)
	if err != nil {
		if mapped := mapPgError(err, domain.ErrSlotNotFound); mapped != err {
			return nil, mapped
		}
		return nil, fmt.Errorf("query slot %s: %w", id, err)
	}

	return &s, nil
}
