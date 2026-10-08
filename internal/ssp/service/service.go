package service

import (
	"context"
	"errors"

	"github.com/7cout/minissp/internal/ssp/cache"
	"github.com/7cout/minissp/internal/ssp/domain"
	"github.com/7cout/minissp/internal/ssp/events"
	"github.com/7cout/minissp/internal/ssp/reserve"
)

// SlotRepository — доступ к слотам.
type SlotRepository interface {
	Get(ctx context.Context, id string) (*domain.Slot, error)
	GetByName(ctx context.Context, publisherID, name string) (*domain.Slot, error)
	Add(ctx context.Context, slot *domain.Slot) error
	AddIfAbsent(ctx context.Context, slot *domain.Slot) (*domain.Slot, bool, error)
}

// PublisherRepository — доступ к издателям.
type PublisherRepository interface {
	Get(ctx context.Context, id string) (*domain.Publisher, error)
	GetByAPIKey(ctx context.Context, apiKey string) (*domain.Publisher, error)
	Add(ctx context.Context, p *domain.Publisher) error
	AddBalance(ctx context.Context, id string, amount int64) error
}

// BidderClient — клиент одного DSP.
type BidderClient interface {
	Name() string
	GetBid(ctx context.Context, req domain.BidRequest, slot *domain.Slot) (*domain.Bid, error)
	Commit(ctx context.Context, campaignID string, price int64) error
	Rollback(ctx context.Context, campaignID string, price int64) error
	Close() error
}

// TxManager выполняет составные операции над хранилищами SSP
// в одной транзакции.
type TxManager interface {
	// RecordImpression атомарно начисляет publisher'у долю
	// и кладёт событие в outbox.
	RecordImpression(
		ctx context.Context,
		publisherID string,
		amount int64,
		event events.ImpressionEvent,
	) error
}

// CommissionPercent — комиссия платформы.
const CommissionPercent int64 = 20

// Options — зависимости сервиса SSP.
type Options struct {
	Slots      SlotRepository
	Publishers PublisherRepository
	Bidders    []BidderClient
	SlotCache  cache.SlotCache
	Reserve    reserve.Manager
	TxManager  TxManager
}

// Service — сервис SSP.
type Service struct {
	slots      SlotRepository
	publishers PublisherRepository
	bidders    []BidderClient
	biddersBy  map[string]BidderClient
	slotCache  cache.SlotCache
	reserve    reserve.Manager
	txManager  TxManager
}

// New создаёт сервис SSP.
//
// Если SlotCache не задан — используется NoopSlotCache.
// Если TxManager не задан — используется fallback, который просто
// начисляет баланс без outbox. Это для unit-тестов; в проде всегда
// должен быть настоящий TxManager.
//
// Reserve обязателен: без него RunAuction/Impression не смогут
// управлять резервами, и мы хотим узнать об этом сразу при старте,
// а не в первом запросе.
func New(opts Options) *Service {
	if opts.Reserve == nil {
		panic("ssp/service: Options.Reserve is required")
	}

	by := make(map[string]BidderClient, len(opts.Bidders))
	for _, b := range opts.Bidders {
		by[b.Name()] = b
	}

	slotCache := opts.SlotCache
	if slotCache == nil {
		slotCache = cache.NoopSlotCache{}
	}

	txManager := opts.TxManager
	if txManager == nil {
		txManager = fallbackTxManager{publishers: opts.Publishers}
	}

	return &Service{
		slots:      opts.Slots,
		publishers: opts.Publishers,
		bidders:    opts.Bidders,
		biddersBy:  by,
		slotCache:  slotCache,
		reserve:    opts.Reserve,
		txManager:  txManager,
	}
}

// Close закрывает всех биддеров, объединяя ошибки.
func (s *Service) Close() error {
	var errs []error
	for _, b := range s.bidders {
		if err := b.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// fallbackTxManager используется в unit-тестах: просто начисляет
// баланс, событие никуда не пишется.
//
// В проде всегда должен использоваться настоящий TxManager —
// memory или postgres. Fallback существует для тестов, где событие
// в Kafka не проверяется.
type fallbackTxManager struct {
	publishers PublisherRepository
}

// RecordImpression начисляет баланс без записи в outbox.
func (f fallbackTxManager) RecordImpression(
	ctx context.Context,
	publisherID string,
	amount int64,
	_ events.ImpressionEvent,
) error {
	return f.publishers.AddBalance(ctx, publisherID, amount)
}
