package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/7cout/minissp/internal/dsp/domain"
)

// CreativeRepo — реализация CreativeRepository поверх PostgreSQL.
type CreativeRepo struct {
	pool *pgxpool.Pool
}

// NewCreativeRepo создаёт репозиторий.
func NewCreativeRepo(pool *pgxpool.Pool) *CreativeRepo {
	return &CreativeRepo{pool: pool}
}

// creativeColumns — колонки, которые читаем в scanCreative.
const creativeColumns = `
	id, campaign_id, type, url, click_url,
	banner_width, banner_height,
	video_width, video_height, video_duration, video_mimes
`

// Get возвращает креатив по ID.
func (r *CreativeRepo) Get(ctx context.Context, id string) (*domain.Creative, error) {
	q := `SELECT ` + creativeColumns + ` FROM dsp.creatives WHERE id = $1`

	c, err := scanCreative(r.pool.QueryRow(ctx, q, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrCreativeNotFound
		}
		return nil, fmt.Errorf("get creative %s: %w", id, err)
	}
	return c, nil
}

// ListByCampaign возвращает все креативы указанной кампании.
func (r *CreativeRepo) ListByCampaign(ctx context.Context, campaignID string) ([]domain.Creative, error) {
	q := `SELECT ` + creativeColumns + `
		FROM dsp.creatives
		WHERE campaign_id = $1
	`

	rows, err := r.pool.Query(ctx, q, campaignID)
	if err != nil {
		return nil, fmt.Errorf("list creatives for campaign %s: %w", campaignID, err)
	}
	defer rows.Close()

	var result []domain.Creative
	for rows.Next() {
		c, err := scanCreative(rows)
		if err != nil {
			return nil, fmt.Errorf("scan creative: %w", err)
		}
		result = append(result, *c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate creatives: %w", err)
	}
	return result, nil
}

// Add добавляет креатив.
//
// Не идемпотентен: повторный вызов с тем же id вернёт unique violation.
// Используется только из seed.
func (r *CreativeRepo) Add(ctx context.Context, c *domain.Creative) error {
	q := `
		INSERT INTO dsp.creatives (
			id, campaign_id, type, url, click_url,
			banner_width, banner_height,
			video_width, video_height, video_duration, video_mimes
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`

	_, err := r.pool.Exec(ctx, q, creativeArgs(c)...)
	if err != nil {
		return fmt.Errorf("insert creative %s: %w", c.ID, err)
	}
	return nil
}

// --- Внутренние конвертеры ---

// scanCreative читает одну строку dsp.creatives в domain.Creative.
func scanCreative(r row) (*domain.Creative, error) {
	var (
		c             domain.Creative
		typeStr       string
		bannerWidth   *int
		bannerHeight  *int
		videoWidth    *int
		videoHeight   *int
		videoDuration *int
		videoMimes    []string
	)

	err := r.Scan(
		&c.ID, &c.CampaignID, &typeStr, &c.URL, &c.ClickURL,
		&bannerWidth, &bannerHeight,
		&videoWidth, &videoHeight, &videoDuration, &videoMimes,
	)
	if err != nil {
		return nil, err
	}
	c.Type = domain.CreativeType(typeStr)

	switch c.Type {
	case domain.CreativeTypeBanner:
		if bannerWidth != nil && bannerHeight != nil {
			c.Banner = &domain.Banner{
				Width:  *bannerWidth,
				Height: *bannerHeight,
			}
		}
	case domain.CreativeTypeVideo:
		if videoWidth != nil && videoHeight != nil && videoDuration != nil {
			c.Video = &domain.Video{
				Width:    *videoWidth,
				Height:   *videoHeight,
				Duration: *videoDuration,
				MIMEs:    videoMimes,
			}
		}
	}

	return &c, nil
}

// creativeArgs раскладывает domain.Creative в плоские параметры для INSERT.
func creativeArgs(c *domain.Creative) []any {
	var (
		bannerWidth   *int
		bannerHeight  *int
		videoWidth    *int
		videoHeight   *int
		videoDuration *int
		videoMimes    []string
	)

	switch c.Type {
	case domain.CreativeTypeBanner:
		if c.Banner != nil {
			bannerWidth = &c.Banner.Width
			bannerHeight = &c.Banner.Height
		}
	case domain.CreativeTypeVideo:
		if c.Video != nil {
			videoWidth = &c.Video.Width
			videoHeight = &c.Video.Height
			videoDuration = &c.Video.Duration
			videoMimes = c.Video.MIMEs
		}
	}

	return []any{
		c.ID, c.CampaignID, string(c.Type), c.URL, c.ClickURL,
		bannerWidth, bannerHeight,
		videoWidth, videoHeight, videoDuration, videoMimes,
	}
}

// row — общий интерфейс QueryRow и Rows для scan-функций.
type row interface {
	Scan(dest ...any) error
}
