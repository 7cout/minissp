package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/7cout/minissp/internal/dsp/domain"
)

// AdvertiserRepo — реализация AdvertiserRepository поверх PostgreSQL.
type AdvertiserRepo struct {
	pool *pgxpool.Pool
}

// NewAdvertiserRepo создаёт репозиторий.
func NewAdvertiserRepo(pool *pgxpool.Pool) *AdvertiserRepo {
	return &AdvertiserRepo{pool: pool}
}

// Get возвращает рекламодателя по ID.
func (r *AdvertiserRepo) Get(ctx context.Context, id string) (*domain.Advertiser, error) {
	const q = `
		SELECT id, name, balance
		FROM dsp.advertisers
		WHERE id = $1
	`

	var a domain.Advertiser
	err := r.pool.QueryRow(ctx, q, id).Scan(&a.ID, &a.Name, &a.Balance)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrAdvertiserNotFound
		}
		return nil, fmt.Errorf("get advertiser %s: %w", id, err)
	}
	return &a, nil
}

// Add добавляет рекламодателя.
//
// Не идемпотентен: повторный вызов с тем же id вернёт unique violation.
// Используется только из seed.
func (r *AdvertiserRepo) Add(ctx context.Context, a *domain.Advertiser) error {
	const q = `
		INSERT INTO dsp.advertisers (id, name, balance)
		VALUES ($1, $2, $3)
	`

	_, err := r.pool.Exec(ctx, q, a.ID, a.Name, a.Balance)
	if err != nil {
		return fmt.Errorf("insert advertiser %s: %w", a.ID, err)
	}
	return nil
}

// Spend атомарно списывает amount с баланса рекламодателя.
//
// Условие balance >= amount проверяется в WHERE — один round-trip,
// никакой гонки между SELECT и UPDATE.
//
// Возвращает:
//   - ErrAdvertiserNotFound, если строки с таким id нет;
//   - ErrInsufficientBalance, если баланса не хватает.
func (r *AdvertiserRepo) Spend(ctx context.Context, id string, amount int64) error {
	const q = `
		UPDATE dsp.advertisers
		SET balance    = balance - $2,
		    updated_at = now()
		WHERE id = $1 AND balance >= $2
	`

	tag, err := r.pool.Exec(ctx, q, id, amount)
	if err != nil {
		return fmt.Errorf("spend advertiser %s: %w", id, err)
	}
	if tag.RowsAffected() == 1 {
		return nil
	}

	// Строка не обновилась: либо advertiser'а нет, либо денег не хватило.
	// Различаем отдельным запросом — этот путь редкий, второй round-trip ок.
	const existsQ = `SELECT balance FROM dsp.advertisers WHERE id = $1`

	var balance int64
	err = r.pool.QueryRow(ctx, existsQ, id).Scan(&balance)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrAdvertiserNotFound
		}
		return fmt.Errorf("check advertiser %s: %w", id, err)
	}
	return domain.ErrInsufficientBalance
}
