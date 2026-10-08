package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/7cout/minissp/internal/dsp/domain"
)

// CampaignRepo — реализация CampaignRepository поверх PostgreSQL.
type CampaignRepo struct {
	pool *pgxpool.Pool
}

// NewCampaignRepo создаёт репозиторий.
func NewCampaignRepo(pool *pgxpool.Pool) *CampaignRepo {
	return &CampaignRepo{pool: pool}
}

// Get возвращает кампанию по ID.
func (r *CampaignRepo) Get(ctx context.Context, id string) (*domain.Campaign, error) {
	const q = `
		SELECT id, advertiser_id, name,
		       budget_total, budget_remaining, budget_reserved, geo_target
		FROM dsp.campaigns
		WHERE id = $1
	`

	var c domain.Campaign
	err := r.pool.QueryRow(ctx, q, id).Scan(
		&c.ID, &c.AdvertiserID, &c.Name,
		&c.BudgetTotal, &c.BudgetRemaining, &c.BudgetReserved, &c.GeoTarget,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrCampaignNotFound
		}
		return nil, fmt.Errorf("get campaign %s: %w", id, err)
	}
	return &c, nil
}

// ListByGeo возвращает кампании, подходящие по гео с доступным бюджетом.
func (r *CampaignRepo) ListByGeo(ctx context.Context, geo string) ([]domain.Campaign, error) {
	const q = `
		SELECT id, advertiser_id, name,
		       budget_total, budget_remaining, budget_reserved, geo_target
		FROM dsp.campaigns
		WHERE geo_target = $1 AND budget_remaining > 0
	`

	rows, err := r.pool.Query(ctx, q, geo)
	if err != nil {
		return nil, fmt.Errorf("list campaigns by geo %s: %w", geo, err)
	}
	defer rows.Close()

	var result []domain.Campaign
	for rows.Next() {
		var c domain.Campaign
		if err := rows.Scan(
			&c.ID, &c.AdvertiserID, &c.Name,
			&c.BudgetTotal, &c.BudgetRemaining, &c.BudgetReserved, &c.GeoTarget,
		); err != nil {
			return nil, fmt.Errorf("scan campaign: %w", err)
		}
		result = append(result, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate campaigns: %w", err)
	}
	return result, nil
}

// Add добавляет кампанию.
func (r *CampaignRepo) Add(ctx context.Context, c *domain.Campaign) error {
	const q = `
		INSERT INTO dsp.campaigns (
			id, advertiser_id, name,
			budget_total, budget_remaining, budget_reserved, geo_target
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`

	_, err := r.pool.Exec(ctx, q,
		c.ID, c.AdvertiserID, c.Name,
		c.BudgetTotal, c.BudgetRemaining, c.BudgetReserved, c.GeoTarget,
	)
	if err != nil {
		return fmt.Errorf("insert campaign %s: %w", c.ID, err)
	}
	return nil
}

// Reserve резервирует amount на бюджете кампании.
func (r *CampaignRepo) Reserve(ctx context.Context, campaignID string, amount int64) error {
	const q = `
		UPDATE dsp.campaigns
		SET budget_remaining = budget_remaining - $2,
		    budget_reserved  = budget_reserved  + $2,
		    updated_at       = now()
		WHERE id = $1 AND budget_remaining >= $2
	`

	tag, err := r.pool.Exec(ctx, q, campaignID, amount)
	if err != nil {
		return fmt.Errorf("reserve campaign %s: %w", campaignID, err)
	}
	if tag.RowsAffected() == 1 {
		return nil
	}

	return r.distinguishBudgetError(ctx, campaignID)
}

// Rollback отменяет резерв, возвращая деньги в budget_remaining.
func (r *CampaignRepo) Rollback(ctx context.Context, campaignID string, amount int64) error {
	const q = `
		UPDATE dsp.campaigns
		SET budget_reserved  = budget_reserved  - $2,
		    budget_remaining = budget_remaining + $2,
		    updated_at       = now()
		WHERE id = $1 AND budget_reserved >= $2
	`

	tag, err := r.pool.Exec(ctx, q, campaignID, amount)
	if err != nil {
		return fmt.Errorf("rollback campaign %s: %w", campaignID, err)
	}
	if tag.RowsAffected() == 1 {
		return nil
	}

	return r.distinguishBudgetError(ctx, campaignID)
}

// distinguishBudgetError определяет, почему UPDATE не сработал:
// кампании нет или денег не хватило.
func (r *CampaignRepo) distinguishBudgetError(ctx context.Context, campaignID string) error {
	const q = `SELECT 1 FROM dsp.campaigns WHERE id = $1`

	var exists int
	err := r.pool.QueryRow(ctx, q, campaignID).Scan(&exists)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrCampaignNotFound
		}
		return fmt.Errorf("check campaign %s: %w", campaignID, err)
	}
	return domain.ErrInsufficientBudget
}
