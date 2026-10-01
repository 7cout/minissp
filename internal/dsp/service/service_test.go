package service

import (
	"context"

	"github.com/7cout/minissp/internal/dsp/domain"
)

// --- Fake: CampaignRepository ---

type fakeCampaignRepo struct {
	campaigns map[string]*domain.Campaign
	byGeo     map[string][]domain.Campaign

	getErr      error
	listErr     error
	commitErr   error
	rollbackErr error

	// reserveErrByID — ошибка для конкретной кампании.
	// Если ID нет в map — Reserve успешен.
	reserveErrByID map[string]error

	reserved   []call
	committed  []call
	rolledBack []call
}

type call struct {
	campaignID string
	amount     int64
}

func (f *fakeCampaignRepo) Get(_ context.Context, id string) (*domain.Campaign, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	c, ok := f.campaigns[id]
	if !ok {
		return nil, domain.ErrCampaignNotFound
	}
	return c, nil
}

func (f *fakeCampaignRepo) ListByGeo(_ context.Context, geo string) ([]domain.Campaign, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	return f.byGeo[geo], nil
}

func (f *fakeCampaignRepo) Reserve(_ context.Context, campaignID string, amount int64) error {
	if f.reserveErrByID != nil {
		if err, ok := f.reserveErrByID[campaignID]; ok {
			return err
		}
	}
	f.reserved = append(f.reserved, call{campaignID, amount})
	return nil
}

func (f *fakeCampaignRepo) Commit(_ context.Context, campaignID string, amount int64) error {
	if f.commitErr != nil {
		return f.commitErr
	}
	f.committed = append(f.committed, call{campaignID, amount})
	return nil
}

func (f *fakeCampaignRepo) Rollback(_ context.Context, campaignID string, amount int64) error {
	if f.rollbackErr != nil {
		return f.rollbackErr
	}
	f.rolledBack = append(f.rolledBack, call{campaignID, amount})
	return nil
}

// --- Fake: CreativeRepository ---

type fakeCreativeRepo struct {
	creatives map[string][]domain.Creative
	err       error
}

func (f *fakeCreativeRepo) ListByCampaign(_ context.Context, campaignID string) ([]domain.Creative, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.creatives[campaignID], nil
}

// --- Fake: AdvertiserRepository ---

type fakeAdvertiserRepo struct {
	advertisers map[string]*domain.Advertiser
	spendErr    error
	spent       []call
}

func (f *fakeAdvertiserRepo) Get(_ context.Context, id string) (*domain.Advertiser, error) {
	a, ok := f.advertisers[id]
	if !ok {
		return nil, domain.ErrAdvertiserNotFound
	}
	return a, nil
}

func (f *fakeAdvertiserRepo) Spend(_ context.Context, id string, amount int64) error {
	if f.spendErr != nil {
		return f.spendErr
	}
	f.spent = append(f.spent, call{id, amount})
	return nil
}

// --- Хелперы ---

func testBidRequest() domain.BidRequest {
	return domain.BidRequest{
		RequestID: "req_1",
		ImpID:     "imp_1",
		SlotID:    "slot_1",
		Width:     320,
		Height:    50,
		Geo:       "RU",
		BidFloor:  1_000_000,
	}
}

func testCampaign(id, advertiserID, geo string) domain.Campaign {
	return domain.Campaign{
		ID:              id,
		AdvertiserID:    advertiserID,
		Name:            "Test Campaign",
		BudgetTotal:     10_000_000,
		BudgetRemaining: 10_000_000,
		BudgetReserved:  0,
		GeoTarget:       geo,
	}
}

func testBannerCreative(id, campaignID string, w, h int) domain.Creative {
	return domain.Creative{
		ID:         id,
		CampaignID: campaignID,
		Type:       domain.CreativeTypeBanner,
		Banner:     &domain.Banner{Width: w, Height: h},
		URL:        "https://cdn.example.com/banner.jpg",
		ClickURL:   "https://example.com/click",
	}
}

func testCampaignPtr(id, advertiserID, geo string) *domain.Campaign {
	c := testCampaign(id, advertiserID, geo)
	return &c
}
