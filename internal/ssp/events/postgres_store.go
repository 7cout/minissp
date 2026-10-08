package events

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresStore — реализация Store поверх PostgreSQL.
type PostgresStore struct {
	pool *pgxpool.Pool
}

// NewPostgresStore создаёт store.
func NewPostgresStore(pool *pgxpool.Pool) *PostgresStore {
	return &PostgresStore{pool: pool}
}

// Enqueue сохраняет событие через pool. Для атомарности с бизнес-операцией
// используйте EnqueueTx.
func (s *PostgresStore) Enqueue(ctx context.Context, event ImpressionEvent) error {
	payload, err := marshalEvent(event)
	if err != nil {
		return err
	}

	const q = `
		INSERT INTO ssp.event_outbox (id, topic, event_key, payload)
		VALUES ($1, $2, $3, $4)
	`
	if _, err := s.pool.Exec(ctx, q, uuid.NewString(), event.Topic(), event.Key(), payload); err != nil {
		return fmt.Errorf("insert event: %w", err)
	}
	return nil
}

// EnqueueTx сохраняет событие внутри переданной транзакции.
// Используется TxManager'ом для атомарности с начислением баланса.
func EnqueueTx(ctx context.Context, tx pgx.Tx, event ImpressionEvent) error {
	payload, err := marshalEvent(event)
	if err != nil {
		return err
	}

	const q = `
		INSERT INTO ssp.event_outbox (id, topic, event_key, payload)
		VALUES ($1, $2, $3, $4)
	`
	if _, err := tx.Exec(ctx, q, uuid.NewString(), event.Topic(), event.Key(), payload); err != nil {
		return fmt.Errorf("insert event: %w", err)
	}
	return nil
}

func marshalEvent(event ImpressionEvent) ([]byte, error) {
	payload, err := event.Marshal()
	if err != nil {
		return nil, fmt.Errorf("marshal event: %w", err)
	}
	return payload, nil
}

// FetchUnpublished возвращает неопубликованные записи.
func (s *PostgresStore) FetchUnpublished(ctx context.Context, limit int) ([]Record, error) {
	const q = `
		SELECT id, topic, event_key, payload, created_at, attempts
		FROM ssp.event_outbox
		WHERE published_at IS NULL
		ORDER BY created_at
		LIMIT $1
	`
	rows, err := s.pool.Query(ctx, q, limit)
	if err != nil {
		return nil, fmt.Errorf("fetch unpublished: %w", err)
	}
	defer rows.Close()

	var out []Record
	for rows.Next() {
		var r Record
		if err := rows.Scan(
			&r.ID, &r.Topic, &r.Key, &r.Payload, &r.CreatedAt, &r.Attempts,
		); err != nil {
			return nil, fmt.Errorf("scan event: %w", err)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// MarkPublished помечает запись опубликованной.
func (s *PostgresStore) MarkPublished(ctx context.Context, id string) error {
	const q = `UPDATE ssp.event_outbox SET published_at = now() WHERE id = $1`
	if _, err := s.pool.Exec(ctx, q, id); err != nil {
		return fmt.Errorf("mark published: %w", err)
	}
	return nil
}

// MarkFailed регистрирует неудачную попытку.
func (s *PostgresStore) MarkFailed(ctx context.Context, id string, lastErr error) error {
	const q = `
		UPDATE ssp.event_outbox
		SET attempts = attempts + 1,
		    last_error = $2
		WHERE id = $1
	`
	msg := ""
	if lastErr != nil {
		msg = lastErr.Error()
	}
	if _, err := s.pool.Exec(ctx, q, id, msg); err != nil {
		return fmt.Errorf("mark failed: %w", err)
	}
	return nil
}

// CountUnpublished возвращает количество неопубликованных записей.
func (s *PostgresStore) CountUnpublished(ctx context.Context) (int, error) {
	const q = `SELECT count(*) FROM ssp.event_outbox WHERE published_at IS NULL`

	var n int
	if err := s.pool.QueryRow(ctx, q).Scan(&n); err != nil {
		return 0, fmt.Errorf("count unpublished: %w", err)
	}
	return n, nil
}
