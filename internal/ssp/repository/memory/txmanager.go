package memory

import (
	"context"

	"github.com/7cout/minissp/internal/ssp/domain"
	"github.com/7cout/minissp/internal/ssp/events"
)

// TxManager выполняет составные операции над memory-репозиториями.
//
// В памяти нет настоящей транзакционности — но под одним мьютексом
// и AddBalance, и Enqueue попадут в один критический участок.
// Для pet-проекта этого достаточно. Настоящая атомарность
// достигается в postgres-версии.
type TxManager struct {
	publishers *PublisherRepo
	eventStore events.Store
}

// NewTxManager создаёт менеджер транзакций.
func NewTxManager(publishers *PublisherRepo, eventStore events.Store) *TxManager {
	return &TxManager{publishers: publishers, eventStore: eventStore}
}

// RecordImpression атомарно начисляет publisher'у долю
// и кладёт событие в outbox.
func (m *TxManager) RecordImpression(
	ctx context.Context,
	publisherID string,
	amount int64,
	event events.ImpressionEvent,
) error {
	m.publishers.mu.Lock()
	defer m.publishers.mu.Unlock()

	p, ok := m.publishers.byID[publisherID]
	if !ok {
		return domain.ErrPublisherNotFound
	}
	p.Balance += amount

	// Enqueue берёт свой mutex (внутри MemoryStore). Это ок:
	// порядок локов фиксирован (publishers → eventStore), deadlock'а
	// не будет, потому что никто не берёт их в обратном порядке.
	return m.eventStore.Enqueue(ctx, event)
}
