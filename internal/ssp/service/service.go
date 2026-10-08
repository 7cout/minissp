package service

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/7cout/minissp/internal/ssp/cache"
	"github.com/7cout/minissp/internal/ssp/domain"
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

const (
	// CommissionPercent — комиссия платформы.
	CommissionPercent int64 = 20

	// AuctionTTL — сколько живёт запись аукциона до автоотката.
	AuctionTTL = 30 * time.Second

	// ProcessedTTL — сколько храним ID обработанных impression
	// для идемпотентности.
	ProcessedTTL = 10 * time.Minute
)

// Options — зависимости сервиса SSP.
//
// Именованные поля вместо позиционных аргументов: конструктор
// не разрастается при добавлении новых зависимостей (метрики,
// reserve manager и т.д.), вызовы в тестах читаются понятнее.
type Options struct {
	Slots      SlotRepository
	Publishers PublisherRepository
	Bidders    []BidderClient
	SlotCache  cache.SlotCache
}

// Service — сервис SSP.
type Service struct {
	slots      SlotRepository
	publishers PublisherRepository
	bidders    []BidderClient
	biddersBy  map[string]BidderClient // name → client, для быстрого поиска
	slotCache  cache.SlotCache

	mu        sync.RWMutex
	auctions  map[string]domain.AuctionRecord
	processed map[string]time.Time // auction_id → когда обработан
}

// New создаёт сервис SSP.
//
// Если SlotCache не задан — используется NoopSlotCache,
// сервис работает без кэша.
func New(opts Options) *Service {
	by := make(map[string]BidderClient, len(opts.Bidders))
	for _, b := range opts.Bidders {
		by[b.Name()] = b
	}

	slotCache := opts.SlotCache
	if slotCache == nil {
		slotCache = cache.NoopSlotCache{}
	}

	return &Service{
		slots:      opts.Slots,
		publishers: opts.Publishers,
		bidders:    opts.Bidders,
		biddersBy:  by,
		slotCache:  slotCache,
		auctions:   make(map[string]domain.AuctionRecord),
		processed:  make(map[string]time.Time),
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
