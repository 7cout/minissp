package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/7cout/minissp/internal/ssp/domain"
	"github.com/7cout/minissp/internal/ssp/events"
)

// TxManager выполняет составные операции над SSP-таблицами
// в одной транзакции.
type TxManager struct {
	pool *pgxpool.Pool
}

// NewTxManager создаёт менеджер транзакций.
func NewTxManager(pool *pgxpool.Pool) *TxManager {
	return &TxManager{pool: pool}
}

// RecordImpression атомарно начисляет publisher'у долю
// и кладёт событие в outbox.
//
// Одна транзакция: либо оба изменения применятся, либо ни одно.
// Если Enqueue упадёт — баланс не увеличится, что даёт нам
// точную консистентность между движением денег и журналом событий.
func (m *TxManager) RecordImpression(
	ctx context.Context,
	publisherID string,
	amount int64,
	event events.ImpressionEvent,
) error {
	tx, err := m.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// 1. Начислить баланс publisher'у.
	const creditQ = `
		UPDATE ssp.publishers
		SET balance    = balance + $2,
		    updated_at = now()
		WHERE id = $1
	`
	tag, err := tx.Exec(ctx, creditQ, publisherID, amount)
	if err != nil {
		return fmt.Errorf("credit publisher: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrPublisherNotFound
	}

	// 2. Записать событие в outbox (та же транзакция).
	if err := events.EnqueueTx(ctx, tx, event); err != nil {
		return fmt.Errorf("enqueue event: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	return nil
}
