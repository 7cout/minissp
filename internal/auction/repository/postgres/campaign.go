package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/7cout/minissp/internal/auction/domain"
)

// CampaignRepo - PostgreSQL-реализация CampaignRepository
type CampaignRepo struct {
	db *pgxpool.Pool
}

// NewCampaignRepo создаёт репозиторий кампаний.
func NewCampaignRepo(db *pgxpool.Pool) *CampaignRepo {
	return &CampaignRepo{db: db}
}

// Get возвращает кампанию по ID.
//
// Возвращает domain.ErrCampaignNotFound, если кампания не найдена.
func (r *CampaignRepo) Get(ctx context.Context, id string) (*domain.Campaign, error) {
	const query = `
		SELECT id, advertiser_id, name,
		       budget_total, budget_remaining, budget_reserved,
		       geo_target
		FROM campaigns
		WHERE id = $1
	`

	var c domain.Campaign
	err := r.db.QueryRow(ctx, query, id).Scan(
		&c.ID,
		&c.AdvertiserID,
		&c.Name,
		&c.BudgetTotal,
		&c.BudgetRemaining,
		&c.BudgetReserved,
		&c.GeoTarget,
	)
	if err != nil {
		if mapped := mapPgError(err, domain.ErrCampaignNotFound); mapped != err {
			return nil, mapped
		}
		return nil, fmt.Errorf("query campaign %s: %w", id, err)
	}

	return &c, nil
}

// ListByGeo возвращает кампании, подходящие по гео и имеющие
// доступный бюджет (budget_remaining - budget_reserved > 0)
func (r *CampaignRepo) ListByGeo(ctx context.Context, geo string) ([]domain.Campaign, error) {
	const query = `
		SELECT id, advertiser_id, name,
		       budget_total, budget_remaining, budget_reserved,
		       geo_target
		FROM campaigns
		WHERE geo_target = $1
		  AND budget_remaining - budget_reserved > 0
	`

	rows, err := r.db.Query(ctx, query, geo)
	if err != nil {
		if mapped := mapPgError(err, nil); mapped != err {
			return nil, mapped
		}
		return nil, fmt.Errorf("query campaigns by geo %s: %w", geo, err)
	}
	defer rows.Close()

	var campaigns []domain.Campaign
	for rows.Next() {
		var c domain.Campaign
		if err := rows.Scan(
			&c.ID,
			&c.AdvertiserID,
			&c.Name,
			&c.BudgetTotal,
			&c.BudgetRemaining,
			&c.BudgetReserved,
			&c.GeoTarget,
		); err != nil {
			return nil, fmt.Errorf("scan campaign: %w", err)
		}
		campaigns = append(campaigns, c)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate campaigns: %w", err)
	}

	return campaigns, nil
}

// Reserve резервирует amount на бюджете кампании
//
// Использует атомарный UPDATE: проверка и изменение бюджета
// происходят в одной SQL-операции. Не требует мьютекса,
// безопасен при параллельных вызовах
//
// Возвращает domain.ErrInsufficientBudget, если бюджета не хватает
func (r *CampaignRepo) Reserve(ctx context.Context, campaignID string, amount int64) error {
	const query = `
		WITH updated AS (
			UPDATE campaigns
			SET budget_remaining = budget_remaining - $1,
				budget_reserved = budget_reserved + $1
			WHERE id = $2
				AND budget_remaining >= $1
			RETURNING id
		)
		SELECT
			EXISTS(SELECT 1 FROM updated) AS was_updated,
			EXISTS(SELECT 1 FROM campaigns WHERE id = $2) AS campaign_exists
	`

	var wasUpdated, campaignExists bool
	err := r.db.QueryRow(ctx, query, amount, campaignID).Scan(&wasUpdated, &campaignExists)
	if err != nil {
		if mapped := mapPgError(err, nil); mapped != err {
			return mapped
		}
		return fmt.Errorf("reserve budget for campaign %s: %w", campaignID, err)
	}

	switch {
	case wasUpdated:
		return nil
	case !campaignExists:
		return domain.ErrCampaignNotFound
	default:
		return domain.ErrInsufficientBudget
	}
}
