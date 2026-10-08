package service

import (
	"context"
	"errors"
	"testing"

	"github.com/7cout/minissp/internal/dsp/domain"
)

func TestService_GetBid(t *testing.T) {
	t.Run("happy path — finds campaign and creative", func(t *testing.T) {
		campaigns := &fakeCampaignRepo{
			campaigns: map[string]*domain.Campaign{
				"camp_1": testCampaignPtr("camp_1", "adv_1", "RU"),
			},
			byGeo: map[string][]domain.Campaign{
				"RU": {testCampaign("camp_1", "adv_1", "RU")},
			},
		}
		creatives := &fakeCreativeRepo{
			creatives: map[string][]domain.Creative{
				"camp_1": {testBannerCreative("cr_1", "camp_1", 320, 50)},
			},
		}

		svc := New(campaigns, creatives, &fakeTxManager{}, 150)

		bid, err := svc.GetBid(context.Background(), testBidRequest())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if bid.CampaignID != "camp_1" {
			t.Errorf("campaign = %q, want camp_1", bid.CampaignID)
		}
		if bid.CreativeID != "cr_1" {
			t.Errorf("creative = %q, want cr_1", bid.CreativeID)
		}
		if bid.Price != 1_500_000 {
			t.Errorf("price = %d, want 1500000", bid.Price)
		}
		if bid.ID == "" {
			t.Error("bid id is empty")
		}
		if bid.ImpID != "imp_1" {
			t.Errorf("imp id = %q, want imp_1", bid.ImpID)
		}

		if len(campaigns.reserved) != 1 {
			t.Fatalf("want 1 reserve call, got %d", len(campaigns.reserved))
		}
		if campaigns.reserved[0].campaignID != "camp_1" {
			t.Errorf("reserved campaign = %q", campaigns.reserved[0].campaignID)
		}
		if campaigns.reserved[0].amount != 1_500_000 {
			t.Errorf("reserved amount = %d, want 1500000", campaigns.reserved[0].amount)
		}
	})

	t.Run("no campaigns by geo — ErrNoEligibleCampaign", func(t *testing.T) {
		campaigns := &fakeCampaignRepo{
			campaigns: map[string]*domain.Campaign{},
			byGeo:     map[string][]domain.Campaign{},
		}
		creatives := &fakeCreativeRepo{creatives: map[string][]domain.Creative{}}

		svc := New(campaigns, creatives, &fakeTxManager{}, 150)

		_, err := svc.GetBid(context.Background(), testBidRequest())
		if !errors.Is(err, domain.ErrNoEligibleCampaign) {
			t.Errorf("want ErrNoEligibleCampaign, got %v", err)
		}
	})

	t.Run("no matching creative — ErrNoEligibleCampaign", func(t *testing.T) {
		campaigns := &fakeCampaignRepo{
			campaigns: map[string]*domain.Campaign{
				"camp_1": testCampaignPtr("camp_1", "adv_1", "RU"),
			},
			byGeo: map[string][]domain.Campaign{
				"RU": {testCampaign("camp_1", "adv_1", "RU")},
			},
		}
		creatives := &fakeCreativeRepo{
			creatives: map[string][]domain.Creative{
				"camp_1": {testBannerCreative("cr_1", "camp_1", 640, 100)},
			},
		}

		svc := New(campaigns, creatives, &fakeTxManager{}, 150)

		_, err := svc.GetBid(context.Background(), testBidRequest())
		if !errors.Is(err, domain.ErrNoEligibleCampaign) {
			t.Errorf("want ErrNoEligibleCampaign, got %v", err)
		}
		if len(campaigns.reserved) != 0 {
			t.Error("Reserve should not be called for mismatched creative")
		}
	})

	t.Run("campaign has only video creative — skipped", func(t *testing.T) {
		campaigns := &fakeCampaignRepo{
			campaigns: map[string]*domain.Campaign{
				"camp_1": testCampaignPtr("camp_1", "adv_1", "RU"),
			},
			byGeo: map[string][]domain.Campaign{
				"RU": {testCampaign("camp_1", "adv_1", "RU")},
			},
		}
		creatives := &fakeCreativeRepo{
			creatives: map[string][]domain.Creative{
				"camp_1": {
					{
						ID:         "cr_video",
						CampaignID: "camp_1",
						Type:       domain.CreativeTypeVideo,
						Video: &domain.Video{
							Width:    320,
							Height:   50,
							Duration: 15,
							MIMEs:    []string{"video/mp4"},
						},
						URL:      "https://cdn.example.com/video.mp4",
						ClickURL: "https://example.com/click",
					},
				},
			},
		}
		svc := New(campaigns, creatives, &fakeTxManager{}, 150)

		_, err := svc.GetBid(context.Background(), testBidRequest())
		if !errors.Is(err, domain.ErrNoEligibleCampaign) {
			t.Errorf("want ErrNoEligibleCampaign, got %v", err)
		}
	})

	t.Run("banner creative without banner params — skipped", func(t *testing.T) {
		campaigns := &fakeCampaignRepo{
			campaigns: map[string]*domain.Campaign{
				"camp_1": testCampaignPtr("camp_1", "adv_1", "RU"),
			},
			byGeo: map[string][]domain.Campaign{
				"RU": {testCampaign("camp_1", "adv_1", "RU")},
			},
		}
		creatives := &fakeCreativeRepo{
			creatives: map[string][]domain.Creative{
				"camp_1": {
					{
						ID:         "cr_1",
						CampaignID: "camp_1",
						Type:       domain.CreativeTypeBanner,
						Banner:     nil,
						URL:        "https://cdn.example.com/banner.jpg",
						ClickURL:   "https://example.com/click",
					},
				},
			},
		}
		svc := New(campaigns, creatives, &fakeTxManager{}, 150)

		_, err := svc.GetBid(context.Background(), testBidRequest())
		if !errors.Is(err, domain.ErrNoEligibleCampaign) {
			t.Errorf("want ErrNoEligibleCampaign, got %v", err)
		}
	})

	t.Run("list campaigns fails", func(t *testing.T) {
		campaigns := &fakeCampaignRepo{listErr: errors.New("db error")}
		svc := New(campaigns, &fakeCreativeRepo{}, &fakeTxManager{}, 150)

		_, err := svc.GetBid(context.Background(), testBidRequest())
		if err == nil {
			t.Fatal("want error, got nil")
		}
	})

	t.Run("list creatives fails", func(t *testing.T) {
		campaigns := &fakeCampaignRepo{
			campaigns: map[string]*domain.Campaign{
				"camp_1": testCampaignPtr("camp_1", "adv_1", "RU"),
			},
			byGeo: map[string][]domain.Campaign{
				"RU": {testCampaign("camp_1", "adv_1", "RU")},
			},
		}
		creatives := &fakeCreativeRepo{err: errors.New("db error")}
		svc := New(campaigns, creatives, &fakeTxManager{}, 150)

		_, err := svc.GetBid(context.Background(), testBidRequest())
		if err == nil {
			t.Fatal("want error, got nil")
		}
	})

	t.Run("first campaign insufficient budget — tries second", func(t *testing.T) {
		campaigns := &fakeCampaignRepo{
			campaigns: map[string]*domain.Campaign{
				"camp_1": testCampaignPtr("camp_1", "adv_1", "RU"),
				"camp_2": testCampaignPtr("camp_2", "adv_1", "RU"),
			},
			byGeo: map[string][]domain.Campaign{
				"RU": {
					testCampaign("camp_1", "adv_1", "RU"),
					testCampaign("camp_2", "adv_1", "RU"),
				},
			},
			reserveErrByID: map[string]error{
				"camp_1": domain.ErrInsufficientBudget,
			},
		}
		creatives := &fakeCreativeRepo{
			creatives: map[string][]domain.Creative{
				"camp_1": {testBannerCreative("cr_1", "camp_1", 320, 50)},
				"camp_2": {testBannerCreative("cr_2", "camp_2", 320, 50)},
			},
		}
		svc := New(campaigns, creatives, &fakeTxManager{}, 150)

		bid, err := svc.GetBid(context.Background(), testBidRequest())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if bid.CampaignID != "camp_2" {
			t.Errorf("campaign = %q, want camp_2", bid.CampaignID)
		}
		if bid.CreativeID != "cr_2" {
			t.Errorf("creative = %q, want cr_2", bid.CreativeID)
		}
	})

	t.Run("reserve fails with unexpected error", func(t *testing.T) {
		campaigns := &fakeCampaignRepo{
			campaigns: map[string]*domain.Campaign{
				"camp_1": testCampaignPtr("camp_1", "adv_1", "RU"),
			},
			byGeo: map[string][]domain.Campaign{
				"RU": {testCampaign("camp_1", "adv_1", "RU")},
			},
			reserveErrByID: map[string]error{
				"camp_1": errors.New("unexpected db error"),
			},
		}
		creatives := &fakeCreativeRepo{
			creatives: map[string][]domain.Creative{
				"camp_1": {testBannerCreative("cr_1", "camp_1", 320, 50)},
			},
		}
		svc := New(campaigns, creatives, &fakeTxManager{}, 150)

		_, err := svc.GetBid(context.Background(), testBidRequest())
		if err == nil {
			t.Fatal("want error, got nil")
		}
		if errors.Is(err, domain.ErrNoEligibleCampaign) {
			t.Error("want unexpected error, got ErrNoEligibleCampaign")
		}
	})

	t.Run("zero bid floor — ErrNoEligibleCampaign", func(t *testing.T) {
		campaigns := &fakeCampaignRepo{
			campaigns: map[string]*domain.Campaign{
				"camp_1": testCampaignPtr("camp_1", "adv_1", "RU"),
			},
			byGeo: map[string][]domain.Campaign{
				"RU": {testCampaign("camp_1", "adv_1", "RU")},
			},
		}
		creatives := &fakeCreativeRepo{
			creatives: map[string][]domain.Creative{
				"camp_1": {testBannerCreative("cr_1", "camp_1", 320, 50)},
			},
		}
		svc := New(campaigns, creatives, &fakeTxManager{}, 150)

		req := testBidRequest()
		req.BidFloor = 0

		_, err := svc.GetBid(context.Background(), req)
		if !errors.Is(err, domain.ErrNoEligibleCampaign) {
			t.Errorf("want ErrNoEligibleCampaign, got %v", err)
		}
	})
}
