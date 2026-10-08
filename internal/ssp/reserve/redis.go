package reserve

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/7cout/minissp/internal/ssp/domain"
)

// RedisOptions — параметры RedisManager.
//
// Значения по умолчанию совпадают с MemoryOptions, чтобы при переходе
// с STORAGE=memory на STORAGE=postgres поведение не менялось.
type RedisOptions struct {
	// TTL — сколько живёт запись до автоотката.
	TTL time.Duration

	// ProcessedTTL — TTL для метки обработанного аукциона.
	//
	// Хранится как отдельный ключ, чтобы идемпотентность Impression
	// пережила рестарт сервиса (а не жила только в памяти процесса).
	ProcessedTTL time.Duration

	// ScanInterval — частота прохода воркера по просроченным записям.
	ScanInterval time.Duration

	// TickBatchSize — максимум id за один ZRANGEBYSCORE.
	//
	// Ограничивает работу одного прохода, чтобы не выгрести разом
	// десятки тысяч записей и не заблокировать Redis.
	TickBatchSize int64

	// KeyPrefix — префикс всех ключей резервов, по умолчанию "reserve".
	KeyPrefix string

	// ShardTag — hash-тег внутри фигурных скобок ключей.
	//
	// Все ключи получают один и тот же тег ({ssp}), чтобы в Redis Cluster
	// Lua-скрипты с несколькими ключами не падали с CROSSSLOT.
	// Для standalone-режима значение не играет роли.
	ShardTag string
}

// DefaultRedisOptions — значения по умолчанию.
func DefaultRedisOptions() RedisOptions {
	return RedisOptions{
		TTL:           30 * time.Second,
		ProcessedTTL:  10 * time.Minute,
		ScanInterval:  5 * time.Second,
		TickBatchSize: 100,
		KeyPrefix:     "reserve",
		ShardTag:      "ssp",
	}
}

// RedisManager — реализация Manager поверх Redis.
//
// Схема хранения:
//
//	<prefix>:{<tag>}:records:<id>     STRING  json AuctionRecord
//	<prefix>:{<tag>}:deadlines        ZSET    score = expires_at_ms, member = id
//	<prefix>:{<tag>}:processed:<id>   STRING  "1", TTL = ProcessedTTL
//
// Атомарность Reserve/Consume/Restore/Take — через Lua-скрипты.
// Истечение TTL — через воркер (Subscribe/Tick), который опрашивает
// deadlines sorted set. Keyspace notifications не используем: они
// fire-and-forget и теряются при рестарте подписчика, а для откатов
// денег это недопустимо.
type RedisManager struct {
	client       *redis.Client
	ttl          time.Duration
	processedTTL time.Duration
	scanInterval time.Duration
	tickBatch    int64
	keyPrefix    string
	shardTag     string
}

// NewRedis создаёт Redis-менеджер.
//
// Нулевые поля Options заменяются дефолтными значениями.
// Клиент Redis должен быть уже создан и проверен (см. db.NewRedisClient).
func NewRedis(client *redis.Client, opts RedisOptions) *RedisManager {
	def := DefaultRedisOptions()
	if opts.TTL <= 0 {
		opts.TTL = def.TTL
	}
	if opts.ProcessedTTL <= 0 {
		opts.ProcessedTTL = def.ProcessedTTL
	}
	if opts.ScanInterval <= 0 {
		opts.ScanInterval = def.ScanInterval
	}
	if opts.TickBatchSize <= 0 {
		opts.TickBatchSize = def.TickBatchSize
	}
	if opts.KeyPrefix == "" {
		opts.KeyPrefix = def.KeyPrefix
	}
	if opts.ShardTag == "" {
		opts.ShardTag = def.ShardTag
	}
	return &RedisManager{
		client:       client,
		ttl:          opts.TTL,
		processedTTL: opts.ProcessedTTL,
		scanInterval: opts.ScanInterval,
		tickBatch:    opts.TickBatchSize,
		keyPrefix:    opts.KeyPrefix,
		shardTag:     opts.ShardTag,
	}
}

// --- Ключи ---

func (m *RedisManager) recordsKey(id string) string {
	return fmt.Sprintf("%s:{%s}:records:%s", m.keyPrefix, m.shardTag, id)
}

func (m *RedisManager) deadlinesKey() string {
	return fmt.Sprintf("%s:{%s}:deadlines", m.keyPrefix, m.shardTag)
}

func (m *RedisManager) processedKey(id string) string {
	return fmt.Sprintf("%s:{%s}:processed:%s", m.keyPrefix, m.shardTag, id)
}

// --- Публичные методы ---

// Reserve сохраняет запись атомарно: SET records + ZADD deadlines.
func (m *RedisManager) Reserve(ctx context.Context, record domain.AuctionRecord) error {
	data, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("marshal record: %w", err)
	}

	expiresAt := time.Now().Add(m.ttl).UnixMilli()

	_, err = reserveScript.Run(ctx, m.client,
		[]string{m.recordsKey(record.AuctionID), m.deadlinesKey()},
		data, expiresAt, record.AuctionID,
	).Result()
	if err != nil {
		return fmt.Errorf("reserve script: %w", err)
	}
	return nil
}

// Consume атомарно забирает запись из резерва.
func (m *RedisManager) Consume(ctx context.Context, auctionID string) (domain.AuctionRecord, ConsumeStatus, error) {
	raw, err := consumeScript.Run(ctx, m.client,
		[]string{
			m.recordsKey(auctionID),
			m.deadlinesKey(),
			m.processedKey(auctionID),
		},
		auctionID,
		int(m.processedTTL.Seconds()),
	).Result()
	if err != nil {
		return domain.AuctionRecord{}, 0, fmt.Errorf("consume script: %w", err)
	}

	arr, ok := raw.([]any)
	if !ok || len(arr) == 0 {
		return domain.AuctionRecord{}, 0, fmt.Errorf("consume: unexpected result %T", raw)
	}

	status, _ := arr[0].(string)
	switch status {
	case "notfound":
		return domain.AuctionRecord{}, 0, ErrNotFound
	case "already":
		return domain.AuctionRecord{}, ConsumeAlready, nil
	case "fresh":
		if len(arr) < 2 {
			return domain.AuctionRecord{}, 0, errors.New("consume: fresh without payload")
		}
		body, _ := arr[1].(string)

		var rec domain.AuctionRecord
		if err := json.Unmarshal([]byte(body), &rec); err != nil {
			return domain.AuctionRecord{}, 0, fmt.Errorf("unmarshal record: %w", err)
		}
		return rec, ConsumeFresh, nil
	default:
		return domain.AuctionRecord{}, 0, fmt.Errorf("consume: unknown status %q", status)
	}
}

// Restore возвращает запись в резерв и снимает processed-метку.
//
// CreatedAt обновляется на текущий момент — так у Impression'а
// снова есть полные TTL секунд на ретрай (см. комментарий
// на MemoryManager.Restore).
func (m *RedisManager) Restore(ctx context.Context, record domain.AuctionRecord) error {
	record.CreatedAt = time.Now()

	data, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("marshal record: %w", err)
	}

	expiresAt := time.Now().Add(m.ttl).UnixMilli()

	_, err = restoreScript.Run(ctx, m.client,
		[]string{
			m.recordsKey(record.AuctionID),
			m.deadlinesKey(),
			m.processedKey(record.AuctionID),
		},
		data, expiresAt, record.AuctionID,
	).Result()
	if err != nil {
		return fmt.Errorf("restore script: %w", err)
	}
	return nil
}

// Subscribe запускает воркер обработки просроченных резервов по тикеру.
func (m *RedisManager) Subscribe(ctx context.Context, onExpired ExpiredHandler) {
	ticker := time.NewTicker(m.scanInterval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				slog.Info("reserve: redis worker stopped")
				return
			case <-ticker.C:
				m.Tick(ctx, onExpired)
			}
		}
	}()
}

// Tick обрабатывает просроченные резервы синхронно, один проход.
//
// Логика:
//  1. Достаём из deadlines до TickBatchSize id, у которых score <= now.
//  2. Для каждого — атомарный take (забрать из records, убрать из deadlines).
//  3. Если запись уже забрана — пропускаем.
//  4. Вызываем onExpired(record). При ошибке — Restore (даём шанс позже).
func (m *RedisManager) Tick(ctx context.Context, onExpired ExpiredHandler) {
	nowMs := time.Now().UnixMilli()

	ids, err := m.client.ZRangeArgs(ctx, redis.ZRangeArgs{
		Key:     m.deadlinesKey(),
		Start:   "-inf",
		Stop:    fmt.Sprintf("%d", nowMs),
		ByScore: true,
		Offset:  0,
		Count:   m.tickBatch,
	}).Result()
	if err != nil {
		slog.WarnContext(ctx, "reserve: redis tick zrange failed", "error", err)
		return
	}
	if len(ids) == 0 {
		return
	}

	slog.InfoContext(ctx, "reserve: rolling back expired", "count", len(ids))

	for _, id := range ids {
		raw, err := takeScript.Run(ctx, m.client,
			[]string{m.recordsKey(id), m.deadlinesKey()},
			id,
		).Result()
		if err != nil {
			if errors.Is(err, redis.Nil) {
				// Запись уже забрана Impression'ом или предыдущим тиком.
				continue
			}
			slog.WarnContext(ctx, "reserve: take script failed",
				"auction_id", id, "error", err)
			continue
		}

		body, _ := raw.(string)

		var rec domain.AuctionRecord
		if err := json.Unmarshal([]byte(body), &rec); err != nil {
			slog.WarnContext(ctx, "reserve: unmarshal failed",
				"auction_id", id, "error", err)
			continue
		}

		if err := onExpired(rec); err != nil {
			slog.WarnContext(ctx, "reserve: rollback failed, restoring",
				"auction_id", rec.AuctionID, "error", err)
			_ = m.Restore(ctx, rec)
		}
	}
}

// Peek возвращает запись без её изъятия.
//
// Если записи нет — ErrNotFound. Диагностический метод.
func (m *RedisManager) Peek(ctx context.Context, auctionID string) (domain.AuctionRecord, error) {
	body, err := m.client.Get(ctx, m.recordsKey(auctionID)).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return domain.AuctionRecord{}, ErrNotFound
		}
		return domain.AuctionRecord{}, fmt.Errorf("peek: %w", err)
	}

	var rec domain.AuctionRecord
	if err := json.Unmarshal(body, &rec); err != nil {
		return domain.AuctionRecord{}, fmt.Errorf("unmarshal record: %w", err)
	}
	return rec, nil
}

// --- Lua-скрипты ---
//
// Скрипты вынесены в пакет-уровень: их можно переиспользовать между
// разными RedisManager'ами (мало ли, если поднимем несколько).

// reserveScript: SET records + ZADD deadlines — атомарно.
//
// KEYS[1] = records:<id>
// KEYS[2] = deadlines
// ARGV[1] = json
// ARGV[2] = expires_at_ms
// ARGV[3] = auction_id
var reserveScript = redis.NewScript(`
redis.call('SET', KEYS[1], ARGV[1])
redis.call('ZADD', KEYS[2], ARGV[2], ARGV[3])
return 'ok'
`)

// consumeScript: атомарный Consume.
//
// Возвращает:
//   - {"notfound"} — записи нет и не было;
//   - {"already"}  — запись уже обработана ранее;
//   - {"fresh", "<json>"} — запись взята.
//
// KEYS[1] = records:<id>
// KEYS[2] = deadlines
// KEYS[3] = processed:<id>
// ARGV[1] = auction_id
// ARGV[2] = processed_ttl_seconds
var consumeScript = redis.NewScript(`
local rec = redis.call('GET', KEYS[1])
if not rec then
	if redis.call('EXISTS', KEYS[3]) == 1 then
		return {'already'}
	end
	return {'notfound'}
end
redis.call('DEL', KEYS[1])
redis.call('ZREM', KEYS[2], ARGV[1])
redis.call('SETEX', KEYS[3], ARGV[2], '1')
return {'fresh', rec}
`)

// restoreScript: вернуть запись, снять processed, обновить deadline.
//
// KEYS[1] = records:<id>
// KEYS[2] = deadlines
// KEYS[3] = processed:<id>
// ARGV[1] = json
// ARGV[2] = expires_at_ms
// ARGV[3] = auction_id
var restoreScript = redis.NewScript(`
redis.call('SET', KEYS[1], ARGV[1])
redis.call('ZADD', KEYS[2], ARGV[2], ARGV[3])
redis.call('DEL', KEYS[3])
return 'ok'
`)

// takeScript: забрать запись в Tick.
//
// Отличие от consumeScript: не ставит processed-метку — мы не
// «обрабатываем», а «откатываем». Если onExpired упадёт, Restore
// вернёт запись.
//
// KEYS[1] = records:<id>
// KEYS[2] = deadlines
// ARGV[1] = auction_id
//
// Возвращает строку json или nil, если записи нет
// (тогда заодно чистит фантомный member из deadlines).
var takeScript = redis.NewScript(`
local rec = redis.call('GET', KEYS[1])
if not rec then
	redis.call('ZREM', KEYS[2], ARGV[1])
	return nil
end
redis.call('DEL', KEYS[1])
redis.call('ZREM', KEYS[2], ARGV[1])
return rec
`)
