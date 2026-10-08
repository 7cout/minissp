package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/7cout/minissp/internal/ssp/domain"
)

// SlotRepo — реализация SlotRepository поверх PostgreSQL.
type SlotRepo struct {
	pool *pgxpool.Pool
}

// NewSlotRepo создаёт репозиторий.
func NewSlotRepo(pool *pgxpool.Pool) *SlotRepo {
	return &SlotRepo{pool: pool}
}

// slotColumns — колонки, которые читаем в scanSlot.
const slotColumns = `
	id, publisher_id, name, geo, min_price, type,
	banner_width, banner_height,
	video_width, video_height, video_duration, video_mimes
`

// Get возвращает слот по ID.
func (r *SlotRepo) Get(ctx context.Context, id string) (*domain.Slot, error) {
	q := `SELECT ` + slotColumns + ` FROM ssp.ad_slots WHERE id = $1`

	slot, err := scanSlot(r.pool.QueryRow(ctx, q, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrSlotNotFound
		}
		return nil, fmt.Errorf("get slot %s: %w", id, err)
	}
	return slot, nil
}

// GetByName возвращает слот по (publisher_id, name).
func (r *SlotRepo) GetByName(ctx context.Context, publisherID, name string) (*domain.Slot, error) {
	q := `SELECT ` + slotColumns + `
		FROM ssp.ad_slots
		WHERE publisher_id = $1 AND name = $2`

	slot, err := scanSlot(r.pool.QueryRow(ctx, q, publisherID, name))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrSlotNotFound
		}
		return nil, fmt.Errorf("get slot by name %s/%s: %w", publisherID, name, err)
	}
	return slot, nil
}

// Add добавляет слот.
//
// Не идемпотентен: повторный вызов с тем же id вернёт unique violation.
// Для конкурентной регистрации по (publisher_id, name) — AddIfAbsent.
func (r *SlotRepo) Add(ctx context.Context, slot *domain.Slot) error {
	q := `
		INSERT INTO ssp.ad_slots (
			id, publisher_id, name, geo, min_price, type,
			banner_width, banner_height,
			video_width, video_height, video_duration, video_mimes
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
	`

	_, err := r.pool.Exec(ctx, q, slotArgs(slot)...)
	if err != nil {
		return fmt.Errorf("insert slot %s: %w", slot.ID, err)
	}
	return nil
}

// AddIfAbsent атомарно добавляет слот, если с таким (publisher_id, name)
// ещё нет.
//
// Использует INSERT ... ON CONFLICT (publisher_id, name) DO NOTHING
// RETURNING ... — если RETURNING пуст, значит слот уже существует,
// делаем SELECT и возвращаем его.
//
// Возвращает:
//   - (slot, true,  nil) — слот вставлен;
//   - (slot, false, nil) — слот уже был, возвращаем существующий.
func (r *SlotRepo) AddIfAbsent(ctx context.Context, slot *domain.Slot) (*domain.Slot, bool, error) {
	q := `
		INSERT INTO ssp.ad_slots (
			id, publisher_id, name, geo, min_price, type,
			banner_width, banner_height,
			video_width, video_height, video_duration, video_mimes
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		ON CONFLICT (publisher_id, name) DO NOTHING
		RETURNING ` + slotColumns

	inserted, err := scanSlot(r.pool.QueryRow(ctx, q, slotArgs(slot)...))
	if err == nil {
		return inserted, true, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, false, fmt.Errorf("insert slot %s: %w", slot.ID, err)
	}

	// Слот уже существует — читаем его.
	existing, err := r.GetByName(ctx, slot.PublisherID, slot.Name)
	if err != nil {
		return nil, false, fmt.Errorf("get existing slot %s/%s: %w", slot.PublisherID, slot.Name, err)
	}
	return existing, false, nil
}

// --- Внутренние конвертеры ---

// row — общий интерфейс QueryRow и Rows для scanSlot.
type row interface {
	Scan(dest ...any) error
}

// scanSlot читает одну строку ad_slots в domain.Slot.
//
// pgx сканирует NULL в *int как nil-указатель; по типу слота собираем
// либо Banner, либо Video.
func scanSlot(r row) (*domain.Slot, error) {
	var (
		s             domain.Slot
		typeStr       string
		bannerWidth   *int
		bannerHeight  *int
		videoWidth    *int
		videoHeight   *int
		videoDuration *int
		videoMimes    []string
	)

	err := r.Scan(
		&s.ID, &s.PublisherID, &s.Name, &s.Geo, &s.MinPrice, &typeStr,
		&bannerWidth, &bannerHeight,
		&videoWidth, &videoHeight, &videoDuration, &videoMimes,
	)
	if err != nil {
		return nil, err
	}
	s.Type = domain.CreativeType(typeStr)

	switch s.Type {
	case domain.CreativeTypeBanner:
		if bannerWidth != nil && bannerHeight != nil {
			s.Banner = &domain.Banner{
				Width:  *bannerWidth,
				Height: *bannerHeight,
			}
		}
	case domain.CreativeTypeVideo:
		if videoWidth != nil && videoHeight != nil && videoDuration != nil {
			s.Video = &domain.Video{
				Width:    *videoWidth,
				Height:   *videoHeight,
				Duration: *videoDuration,
				MIMEs:    videoMimes,
			}
		}
	}

	return &s, nil
}

// slotArgs раскладывает domain.Slot в плоские параметры для INSERT.
//
// Для banner-слота заполняет только banner_*, для video — только video_*.
// Остальные остаются nil → NULL в БД.
func slotArgs(s *domain.Slot) []any {
	var (
		bannerWidth   *int
		bannerHeight  *int
		videoWidth    *int
		videoHeight   *int
		videoDuration *int
		videoMimes    []string
	)

	switch s.Type {
	case domain.CreativeTypeBanner:
		if s.Banner != nil {
			bannerWidth = &s.Banner.Width
			bannerHeight = &s.Banner.Height
		}
	case domain.CreativeTypeVideo:
		if s.Video != nil {
			videoWidth = &s.Video.Width
			videoHeight = &s.Video.Height
			videoDuration = &s.Video.Duration
			videoMimes = s.Video.MIMEs
		}
	}

	return []any{
		s.ID, s.PublisherID, s.Name, s.Geo, s.MinPrice, string(s.Type),
		bannerWidth, bannerHeight,
		videoWidth, videoHeight, videoDuration, videoMimes,
	}
}
