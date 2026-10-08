package service

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/7cout/minissp/internal/ssp/cache"
	"github.com/7cout/minissp/internal/ssp/domain"
)

// --- fakeSlotRepo ---

type fakeSlotRepo struct {
	mu    sync.RWMutex
	slots map[string]*domain.Slot

	// ошибки
	getErr       error
	getByNameErr error
}

func newFakeSlotRepo() *fakeSlotRepo {
	return &fakeSlotRepo{slots: make(map[string]*domain.Slot)}
}

func (f *fakeSlotRepo) Add(_ context.Context, slot *domain.Slot) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.slots[slot.ID] = slot
	return nil
}

func (f *fakeSlotRepo) AddIfAbsent(_ context.Context, slot *domain.Slot) (*domain.Slot, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	for _, s := range f.slots {
		if s.PublisherID == slot.PublisherID && s.Name == slot.Name {
			cp := s.Clone()
			return &cp, false, nil
		}
	}

	f.slots[slot.ID] = slot
	cp := slot.Clone()
	return &cp, true, nil
}

func (f *fakeSlotRepo) Get(_ context.Context, id string) (*domain.Slot, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	f.mu.RLock()
	defer f.mu.RUnlock()

	s, ok := f.slots[id]
	if !ok {
		return nil, domain.ErrSlotNotFound
	}
	cp := s.Clone()
	return &cp, nil
}

func (f *fakeSlotRepo) GetByName(_ context.Context, publisherID, name string) (*domain.Slot, error) {
	if f.getByNameErr != nil {
		return nil, f.getByNameErr
	}
	f.mu.RLock()
	defer f.mu.RUnlock()

	for _, s := range f.slots {
		if s.PublisherID == publisherID && s.Name == name {
			cp := s.Clone()
			return &cp, nil
		}
	}
	return nil, domain.ErrSlotNotFound
}

// --- fakeSlotCache ---

// fakeSlotCache — минимальный in-memory кэш без TTL.
//
// Позволяет тестировать cache-aside и сценарий «кэш упал»:
// getErr/putErr возвращают ошибку без ErrCacheMiss.
type fakeSlotCache struct {
	mu      sync.Mutex
	entries map[string]domain.Slot

	getErr error
	putErr error
}

func newFakeSlotCache() *fakeSlotCache {
	return &fakeSlotCache{entries: make(map[string]domain.Slot)}
}

func (c *fakeSlotCache) Get(_ context.Context, publisherID, name string) (*domain.Slot, error) {
	if c.getErr != nil {
		return nil, c.getErr
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	slot, ok := c.entries[publisherID+":"+name]
	if !ok {
		return nil, cache.ErrCacheMiss
	}
	cp := slot.Clone()
	return &cp, nil
}

func (c *fakeSlotCache) Put(_ context.Context, slot *domain.Slot) error {
	if c.putErr != nil {
		return c.putErr
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[slot.PublisherID+":"+slot.Name] = slot.Clone()
	return nil
}

func (c *fakeSlotCache) Invalidate(_ context.Context, publisherID, name string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.entries, publisherID+":"+name)
	return nil
}

// --- fakePublisherRepo ---

type fakePublisherRepo struct {
	mu       sync.RWMutex
	byID     map[string]*domain.Publisher
	byAPIKey map[string]*domain.Publisher

	getErr        error
	getByKeyErr   error
	addBalanceErr error

	addedBalances []balanceAddition
}

type balanceAddition struct {
	publisherID string
	amount      int64
}

func newFakePublisherRepo() *fakePublisherRepo {
	return &fakePublisherRepo{
		byID:     make(map[string]*domain.Publisher),
		byAPIKey: make(map[string]*domain.Publisher),
	}
}

func (f *fakePublisherRepo) Add(_ context.Context, p *domain.Publisher) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.byID[p.ID] = p
	f.byAPIKey[p.APIKey] = p
	return nil
}

func (f *fakePublisherRepo) Get(_ context.Context, id string) (*domain.Publisher, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	f.mu.RLock()
	defer f.mu.RUnlock()

	p, ok := f.byID[id]
	if !ok {
		return nil, domain.ErrPublisherNotFound
	}
	cp := *p
	return &cp, nil
}

func (f *fakePublisherRepo) GetByAPIKey(_ context.Context, key string) (*domain.Publisher, error) {
	if f.getByKeyErr != nil {
		return nil, f.getByKeyErr
	}
	f.mu.RLock()
	defer f.mu.RUnlock()

	p, ok := f.byAPIKey[key]
	if !ok {
		return nil, domain.ErrPublisherNotFound
	}
	cp := *p
	return &cp, nil
}

func (f *fakePublisherRepo) AddBalance(_ context.Context, id string, amount int64) error {
	if f.addBalanceErr != nil {
		return f.addBalanceErr
	}
	f.mu.Lock()
	defer f.mu.Unlock()

	p, ok := f.byID[id]
	if !ok {
		return domain.ErrPublisherNotFound
	}
	p.Balance += amount
	f.addedBalances = append(f.addedBalances, balanceAddition{id, amount})
	return nil
}

// --- fakeBidder ---

type fakeBidder struct {
	name string

	bid    *domain.Bid
	bidErr error

	commitErr   error
	rollbackErr error

	bidDelay time.Duration

	mu                  sync.Mutex
	capturedGetBidReq   domain.BidRequest
	capturedGetBidSlot  *domain.Slot
	capturedCommitID    string
	capturedCommitPrice int64
	capturedRollbackID  string
	capturedRollbackPr  int64
	closed              bool
}

func (f *fakeBidder) Name() string { return f.name }

func (f *fakeBidder) GetBid(ctx context.Context, req domain.BidRequest, slot *domain.Slot) (*domain.Bid, error) {
	if f.bidDelay > 0 {
		select {
		case <-time.After(f.bidDelay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	f.mu.Lock()
	f.capturedGetBidReq = req
	f.capturedGetBidSlot = slot
	f.mu.Unlock()

	if f.bidErr != nil {
		return nil, f.bidErr
	}
	return f.bid, nil
}

func (f *fakeBidder) Commit(_ context.Context, campaignID string, price int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.capturedCommitID = campaignID
	f.capturedCommitPrice = price
	return f.commitErr
}

func (f *fakeBidder) Rollback(_ context.Context, campaignID string, price int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.capturedRollbackID = campaignID
	f.capturedRollbackPr = price
	return f.rollbackErr
}

func (f *fakeBidder) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.closed = true
	return nil
}

// --- helpers для тестовых данных ---

func testBannerSlot(id, publisherID, name string) *domain.Slot {
	return &domain.Slot{
		ID:          id,
		PublisherID: publisherID,
		Name:        name,
		Geo:         "RU",
		MinPrice:    1_000_000,
		Type:        domain.CreativeTypeBanner,
		Banner:      &domain.Banner{Width: 320, Height: 50},
	}
}

func testPublisher(id string) *domain.Publisher {
	return &domain.Publisher{
		ID:     id,
		Name:   "Test Publisher",
		APIKey: "test-key-" + id,
	}
}

// addPublisher добавляет publisher и падает с t.Fatal при ошибке.
func addPublisher(t *testing.T, repo *fakePublisherRepo, p *domain.Publisher) {
	t.Helper()
	if err := repo.Add(context.Background(), p); err != nil {
		t.Fatalf("add publisher: %v", err)
	}
}

// addSlot добавляет слот и падает с t.Fatal при ошибке.
func addSlot(t *testing.T, repo *fakeSlotRepo, slot *domain.Slot) {
	t.Helper()
	if err := repo.Add(context.Background(), slot); err != nil {
		t.Fatalf("add slot: %v", err)
	}
}

func testBidder(name, campaignID string, price int64) *fakeBidder {
	return &fakeBidder{
		name: name,
		bid: &domain.Bid{
			ID:          "bid-" + name,
			CampaignID:  campaignID,
			CreativeID:  "creative-" + name,
			CreativeURL: "https://cdn.example.com/" + name + ".jpg",
			ClickURL:    "https://example.com/click-" + name,
			Price:       price,
		},
	}
}

// newTestService собирает сервис с моками и in-memory фейк-кэшем.
func newTestService(bidders ...*fakeBidder) (*Service, *fakeSlotRepo, *fakePublisherRepo) {
	slots := newFakeSlotRepo()
	pubs := newFakePublisherRepo()

	clientList := make([]BidderClient, 0, len(bidders))
	for _, b := range bidders {
		clientList = append(clientList, b)
	}

	return New(Options{
		Slots:      slots,
		Publishers: pubs,
		Bidders:    clientList,
		SlotCache:  newFakeSlotCache(),
	}), slots, pubs
}

func TestService_RegisterSlot_Concurrent(t *testing.T) {
	svc, _, _ := newTestService()

	slot := domain.Slot{
		Name:     "home_banner",
		Geo:      "RU",
		MinPrice: 1_000_000,
		Type:     domain.CreativeTypeBanner,
		Banner:   &domain.Banner{Width: 320, Height: 50},
	}

	const goroutines = 50
	var wg sync.WaitGroup
	ids := make([]string, goroutines)

	start := make(chan struct{})
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			<-start
			created, err := svc.RegisterSlot(context.Background(), "pub_1", slot)
			if err != nil {
				t.Errorf("goroutine %d: %v", idx, err)
				return
			}
			ids[idx] = created.ID
		}(i)
	}
	close(start)
	wg.Wait()

	for i, id := range ids {
		if id != ids[0] {
			t.Errorf("goroutine %d returned %q, want %q", i, id, ids[0])
		}
	}
}
