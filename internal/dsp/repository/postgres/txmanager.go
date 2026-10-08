package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/7cout/minissp/internal/dsp/domain"
)

// TxManager выполняет составные операции над DSP-таблицами
// в одной транзакции.
type TxManager struct {
	pool *pgxpool.Pool
}

// NewTxManager создаёт менеджер транзакций.
func NewTxManager(pool *pgxpool.Pool) *TxManager {
	return &TxManager{pool: pool}
}

// CommitWithSpend атомарно снимает резерв с кампании и списывает
// деньги с её advertiser'а.
//
// Одна транзакция: BEGIN; UPDATE campaigns RETURNING advertiser_id;
// UPDATE advertisers; COMMIT. Если второй UPDATE не сработал —
// ROLLBACK, первое изменение откатится.
//
// Возможные ошибки:
//   - ErrCampaignNotFound
//   - ErrInsufficientBudget
//   - ErrAdvertiserNotFound
//   - ErrInsufficientBalance
func (m *TxManager) CommitWithSpend(ctx context.Context, campaignID string, price int64) error {
	tx, err := m.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	// No-op, если Commit уже прошёл.
	defer func() { _ = tx.Rollback(ctx) }()

	// 1. Снять резерв, вернуть advertiser_id.
	const commitQ = `
		UPDATE dsp.campaigns
		SET budget_reserved = budget_reserved - $2,
		    updated_at      = now()
		WHERE id = $1 AND budget_reserved >= $2
		RETURNING advertiser_id
	`

	var advertiserID string
	err = tx.QueryRow(ctx, commitQ, campaignID, price).Scan(&advertiserID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return distinguishCampaignError(ctx, tx, campaignID)
		}
		return fmt.Errorf("commit campaign %s: %w", campaignID, err)
	}

	// 2. Списать с advertiser'а.
	const spendQ = `
		UPDATE dsp.advertisers
		SET balance    = balance - $2,
		    updated_at = now()
		WHERE id = $1 AND balance >= $2
	`

	tag, err := tx.Exec(ctx, spendQ, advertiserID, price)
	if err != nil {
		return fmt.Errorf("spend advertiser %s: %w", advertiserID, err)
	}
	if tag.RowsAffected() == 0 {
		return distinguishAdvertiserError(ctx, tx, advertiserID)
	}

	// 3. Фиксируем.
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	return nil
}

// distinguishCampaignError различает «кампании нет» и «reserved мало».
//
// Вызывается внутри транзакции, второй SELECT на редком пути ошибки.
func distinguishCampaignError(ctx context.Context, tx pgx.Tx, campaignID string) error {
	const q = `SELECT 1 FROM dsp.campaigns WHERE id = $1`

	var exists int
	err := tx.QueryRow(ctx, q, campaignID).Scan(&exists)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrCampaignNotFound
	}
	if err != nil {
		return fmt.Errorf("check campaign %s: %w", campaignID, err)
	}
	return domain.ErrInsufficientBudget
}

// distinguishAdvertiserError различает «advertiser'а нет» и «баланса мало».
func distinguishAdvertiserError(ctx context.Context, tx pgx.Tx, advertiserID string) error {
	const q = `SELECT 1 FROM dsp.advertisers WHERE id = $1`

	var exists int
	err := tx.QueryRow(ctx, q, advertiserID).Scan(&exists)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrAdvertiserNotFound
	}
	if err != nil {
		return fmt.Errorf("check advertiser %s: %w", advertiserID, err)
	}
	return domain.ErrInsufficientBalance
}
