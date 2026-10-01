package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/7cout/minissp/internal/auction/domain"
)

// CreativeRepo - PostgreSQL-реализация CreativeRepository
type CreativeRepo struct {
	db *pgxpool.Pool
}

// NewCreativeRepo создаёт репозиторий креативов
func NewCreativeRepo(db *pgxpool.Pool) *CreativeRepo {
	return &CreativeRepo{db: db}
}

// ListByCampaign возвращает все креативы указанной кампании
func (r *CreativeRepo) ListByCampaign(ctx context.Context, campaignID string) ([]domain.Creative, error) {
	const query = `
		SELECT id, campaign_id, width, height, url, click_url
		FROM creatives
		WHERE campaign_id = $1
	`

	rows, err := r.db.Query(ctx, query, campaignID)
	if err != nil {
		if mapped := mapPgError(err, nil); mapped != err {
			return nil, mapped
		}
		return nil, fmt.Errorf("query creatives for campaign %s: %w", campaignID, err)
	}
	defer rows.Close()

	var creatives []domain.Creative
	for rows.Next() {
		var c domain.Creative
		if err := rows.Scan(
			&c.ID,
			&c.CampaignID,
			&c.Banner.Width,
			&c.Banner.Height,
			&c.URL,
			&c.ClickURL,
		); err != nil {
			return nil, fmt.Errorf("scan creative: %w", err)
		}
		creatives = append(creatives, c)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate creatives: %w", err)
	}

	return creatives, nil
}
