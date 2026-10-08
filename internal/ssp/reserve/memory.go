package reserve

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/7cout/minissp/internal/ssp/domain"
)

// MemoryOptions — параметры MemoryManager.
type MemoryOptions struct {
	// TTL — сколько живёт запись до автоотката.
	TTL time.Duration

	// ProcessedTTL — сколько храним отметки обработанных impression.
	ProcessedTTL time.Duration

	// ScanInterval — частота проверки просроченных записей.
	ScanInterval time.Duration
}

// DefaultMemoryOptions — значения по умолчанию.
func DefaultMemoryOptions() MemoryOptions {
	return MemoryOptions{
		TTL:          30 * time.Second,
		ProcessedTTL: 10 * time.Minute,
		ScanInterval: 5 * time.Second,
	}
}

// MemoryManager — in-memory реализация Manager.
type MemoryManager struct {
	ttl          time.Duration
	processedTTL time.Duration
	scanInterval time.Duration

	mu        sync.RWMutex
	auctions  map[string]domain.AuctionRecord
	processed map[string]time.Time
}

// NewMemory создаёт in-memory менеджер.
//
// Нулевые поля Options заменяются дефолтными значениями.
func NewMemory(opts MemoryOptions) *MemoryManager {
	def := DefaultMemoryOptions()
	if opts.TTL <= 0 {
		opts.TTL = def.TTL
	}
	if opts.ProcessedTTL <= 0 {
		opts.ProcessedTTL = def.ProcessedTTL
	}
	if opts.ScanInterval <= 0 {
		opts.ScanInterval = def.ScanInterval
	}
	return &MemoryManager{
		ttl:          opts.TTL,
		processedTTL: opts.ProcessedTTL,
		scanInterval: opts.ScanInterval,
		auctions:     make(map[string]domain.AuctionRecord),
		processed:    make(map[string]time.Time),
	}
}

// Reserve сохраняет запись.
func (m *MemoryManager) Reserve(_ context.Context, record domain.AuctionRecord) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.auctions[record.AuctionID] = record
	return nil
}

// Consume забирает запись.
//
// Если запись уже обработана ранее — возвращает ConsumeAlready.
// Если записи нет — ErrNotFound.
func (m *MemoryManager) Consume(_ context.Context, auctionID string) (domain.AuctionRecord, ConsumeStatus, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.processed[auctionID]; ok {
		return domain.AuctionRecord{}, ConsumeAlready, nil
	}

	record, ok := m.auctions[auctionID]
	if !ok {
		return domain.AuctionRecord{}, 0, ErrNotFound
	}

	delete(m.auctions, auctionID)
	m.processed[auctionID] = time.Now()
	return record, ConsumeFresh, nil
}

// Restore возвращает запись в резерв.
//
// Снимает отметку processed и кладёт запись обратно в auctions.
func (m *MemoryManager) Restore(_ context.Context, record domain.AuctionRecord) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.processed, record.AuctionID)
	m.auctions[record.AuctionID] = record
	return nil
}

// Subscribe запускает воркер, который по тикеру обрабатывает
// просроченные резервы.
func (m *MemoryManager) Subscribe(ctx context.Context, onExpired ExpiredHandler) {
	ticker := time.NewTicker(m.scanInterval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				slog.Info("reserve: worker stopped")
				return
			case <-ticker.C:
				m.Tick(ctx, onExpired)
			}
		}
	}()
}

// Tick обрабатывает просроченные резервы синхронно.
func (m *MemoryManager) Tick(ctx context.Context, onExpired ExpiredHandler) {
	expired := m.listExpired(time.Now())
	if len(expired) == 0 {
		return
	}

	slog.InfoContext(ctx, "reserve: rolling back expired", "count", len(expired))

	for _, rec := range expired {
		taken, ok := m.takeRecord(rec.AuctionID)
		if !ok {
			// Запись уже забрана — Impression успел раньше, либо
			// другой тик воркера её обработал. Пропускаем.
			continue
		}
		if err := onExpired(taken); err != nil {
			slog.WarnContext(ctx, "reserve: rollback failed, restoring",
				"auction_id", taken.AuctionID,
				"error", err,
			)
			_ = m.Restore(ctx, taken)
		}
	}
}

// Peek возвращает запись без её изъятия.
//
// Если записи нет — ErrNotFound. Диагностический метод.
func (m *MemoryManager) Peek(_ context.Context, auctionID string) (domain.AuctionRecord, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	rec, ok := m.auctions[auctionID]
	if !ok {
		return domain.AuctionRecord{}, ErrNotFound
	}
	return rec, nil
}

// Contains возвращает true, если запись с таким ID ещё в резерве.
//
// Диагностический метод. Не учитывает обработанные записи —
// используйте IsProcessed для этого случая.
func (m *MemoryManager) Contains(auctionID string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, ok := m.auctions[auctionID]
	return ok
}

// IsProcessed возвращает true, если запись уже была забрана Consume.
func (m *MemoryManager) IsProcessed(auctionID string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, ok := m.processed[auctionID]
	return ok
}

// ExpireNow помечает запись как просроченную — для тестов и отладки.
//
// В проде не используется.
func (m *MemoryManager) ExpireNow(auctionID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	rec, ok := m.auctions[auctionID]
	if !ok {
		return
	}
	rec.CreatedAt = time.Now().Add(-m.ttl - time.Second)
	m.auctions[auctionID] = rec
}

// --- Внутренние методы ---

// listExpired возвращает записи старше ttl.
func (m *MemoryManager) listExpired(now time.Time) []domain.AuctionRecord {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var expired []domain.AuctionRecord
	for _, r := range m.auctions {
		if now.Sub(r.CreatedAt) > m.ttl {
			expired = append(expired, r)
		}
	}
	return expired
}

// takeRecord извлекает запись и удаляет её.
func (m *MemoryManager) takeRecord(auctionID string) (domain.AuctionRecord, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	rec, ok := m.auctions[auctionID]
	if !ok {
		return domain.AuctionRecord{}, false
	}
	delete(m.auctions, auctionID)
	return rec, true
}
