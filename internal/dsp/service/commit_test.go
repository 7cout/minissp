package service

import (
	"context"
	"errors"
	"testing"

	"github.com/7cout/minissp/internal/dsp/domain"
)

func TestService_Commit(t *testing.T) {
	t.Run("happy path — campaign commit and advertiser spend", func(t *testing.T) {
		campaigns := &fakeCampaignRepo{
			campaigns: map[string]*domain.Campaign{
				"camp_1": testCampaignPtr("camp_1", "adv_1", "RU"),
			},
		}
		advertisers := &fakeAdvertiserRepo{
			advertisers: map[string]*domain.Advertiser{
				"adv_1": testAdvertiser("adv_1", 10_000_000),
			},
		}
		svc := New(campaigns, nil, advertisers, 150)

		err := svc.Commit(context.Background(), "camp_1", 1_500_000)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(campaigns.committed) != 1 {
			t.Fatalf("want 1 campaign commit, got %d", len(campaigns.committed))
		}
		if campaigns.committed[0].campaignID != "camp_1" {
			t.Errorf("committed campaign = %q", campaigns.committed[0].campaignID)
		}
		if campaigns.committed[0].amount != 1_500_000 {
			t.Errorf("committed amount = %d, want 1500000", campaigns.committed[0].amount)
		}

		if len(advertisers.spent) != 1 {
			t.Fatalf("want 1 advertiser spend, got %d", len(advertisers.spent))
		}
		if advertisers.spent[0].campaignID != "adv_1" {
			t.Errorf("advertiser = %q, want adv_1", advertisers.spent[0].campaignID)
		}
		if advertisers.spent[0].amount != 1_500_000 {
			t.Errorf("advertiser spend = %d, want 1500000", advertisers.spent[0].amount)
		}

		if len(campaigns.uncommitted) != 0 {
			t.Error("Uncommit should not be called on happy path")
		}
	})

	t.Run("campaign not found", func(t *testing.T) {
		campaigns := &fakeCampaignRepo{
			campaigns: map[string]*domain.Campaign{},
		}
		advertisers := &fakeAdvertiserRepo{
			advertisers: map[string]*domain.Advertiser{},
		}
		svc := New(campaigns, nil, advertisers, 150)

		err := svc.Commit(context.Background(), "missing", 100)
		if !errors.Is(err, domain.ErrCampaignNotFound) {
			t.Errorf("want ErrCampaignNotFound, got %v", err)
		}

		if len(advertisers.spent) != 0 {
			t.Error("advertiser spend should not be called")
		}
		if len(campaigns.uncommitted) != 0 {
			t.Error("Uncommit should not be called")
		}
	})

	t.Run("campaign commit fails — advertiser spend not called", func(t *testing.T) {
		campaigns := &fakeCampaignRepo{
			campaigns: map[string]*domain.Campaign{
				"camp_1": testCampaignPtr("camp_1", "adv_1", "RU"),
			},
			commitErr: domain.ErrInsufficientBudget,
		}
		advertisers := &fakeAdvertiserRepo{
			advertisers: map[string]*domain.Advertiser{
				"adv_1": testAdvertiser("adv_1", 10_000_000),
			},
		}
		svc := New(campaigns, nil, advertisers, 150)

		err := svc.Commit(context.Background(), "camp_1", 1_500_000)
		if !errors.Is(err, domain.ErrInsufficientBudget) {
			t.Errorf("want ErrInsufficientBudget, got %v", err)
		}
		if len(advertisers.spent) != 0 {
			t.Error("advertiser spend should not be called if campaign commit failed")
		}
		if len(campaigns.uncommitted) != 0 {
			t.Error("Uncommit should not be called if campaign commit failed")
		}
	})

	t.Run("advertiser spend fails — Uncommit called (compensation)", func(t *testing.T) {
		campaigns := &fakeCampaignRepo{
			campaigns: map[string]*domain.Campaign{
				"camp_1": testCampaignPtr("camp_1", "adv_1", "RU"),
			},
		}
		advertisers := &fakeAdvertiserRepo{
			advertisers: map[string]*domain.Advertiser{
				"adv_1": testAdvertiser("adv_1", 10_000_000),
			},
			spendErr: errors.New("unexpected db error"),
		}
		svc := New(campaigns, nil, advertisers, 150)

		err := svc.Commit(context.Background(), "camp_1", 1_500_000)
		if err == nil {
			t.Fatal("want error, got nil")
		}

		// Компенсация должна быть вызвана ровно один раз с теми же аргументами.
		if len(campaigns.uncommitted) != 1 {
			t.Fatalf("want 1 Uncommit call, got %d", len(campaigns.uncommitted))
		}
		if campaigns.uncommitted[0].campaignID != "camp_1" {
			t.Errorf("uncommitted campaign = %q, want camp_1", campaigns.uncommitted[0].campaignID)
		}
		if campaigns.uncommitted[0].amount != 1_500_000 {
			t.Errorf("uncommitted amount = %d, want 1500000", campaigns.uncommitted[0].amount)
		}
	})

	t.Run("insufficient balance — Spend not called", func(t *testing.T) {
		campaigns := &fakeCampaignRepo{
			campaigns: map[string]*domain.Campaign{
				"camp_1": testCampaignPtr("camp_1", "adv_1", "RU"),
			},
		}
		advertisers := &fakeAdvertiserRepo{
			advertisers: map[string]*domain.Advertiser{
				"adv_1": testAdvertiser("adv_1", 100), // меньше, чем 1_500_000
			},
		}
		svc := New(campaigns, nil, advertisers, 150)

		err := svc.Commit(context.Background(), "camp_1", 1_500_000)
		if !errors.Is(err, domain.ErrInsufficientBalance) {
			t.Errorf("want ErrInsufficientBalance, got %v", err)
		}

		// Предварительная проверка должна отсечь до мутаций.
		if len(campaigns.committed) != 0 {
			t.Error("campaign Commit should not be called when balance is insufficient")
		}
		if len(advertisers.spent) != 0 {
			t.Error("advertiser Spend should not be called")
		}
		if len(campaigns.uncommitted) != 0 {
			t.Error("Uncommit should not be called — nothing to compensate")
		}
	})

	t.Run("advertiser not found — campaign commit not called", func(t *testing.T) {
		campaigns := &fakeCampaignRepo{
			campaigns: map[string]*domain.Campaign{
				"camp_1": testCampaignPtr("camp_1", "adv_1", "RU"),
			},
		}
		advertisers := &fakeAdvertiserRepo{
			advertisers: map[string]*domain.Advertiser{}, // adv_1 отсутствует
		}
		svc := New(campaigns, nil, advertisers, 150)

		err := svc.Commit(context.Background(), "camp_1", 1_500_000)
		if !errors.Is(err, domain.ErrAdvertiserNotFound) {
			t.Errorf("want ErrAdvertiserNotFound, got %v", err)
		}

		if len(campaigns.committed) != 0 {
			t.Error("campaign Commit should not be called when advertiser lookup failed")
		}
	})
}
