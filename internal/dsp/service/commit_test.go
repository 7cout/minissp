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
		advertisers := &fakeAdvertiserRepo{}
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
	})

	t.Run("campaign not found", func(t *testing.T) {
		campaigns := &fakeCampaignRepo{
			campaigns: map[string]*domain.Campaign{},
		}
		advertisers := &fakeAdvertiserRepo{}
		svc := New(campaigns, nil, advertisers, 150)

		err := svc.Commit(context.Background(), "missing", 100)
		if !errors.Is(err, domain.ErrCampaignNotFound) {
			t.Errorf("want ErrCampaignNotFound, got %v", err)
		}

		if len(advertisers.spent) != 0 {
			t.Error("advertiser spend should not be called")
		}
	})

	t.Run("campaign commit fails — advertiser spend not called", func(t *testing.T) {
		campaigns := &fakeCampaignRepo{
			campaigns: map[string]*domain.Campaign{
				"camp_1": testCampaignPtr("camp_1", "adv_1", "RU"),
			},
			commitErr: domain.ErrInsufficientBudget,
		}
		advertisers := &fakeAdvertiserRepo{}
		svc := New(campaigns, nil, advertisers, 150)

		err := svc.Commit(context.Background(), "camp_1", 1_500_000)
		if !errors.Is(err, domain.ErrInsufficientBudget) {
			t.Errorf("want ErrInsufficientBudget, got %v", err)
		}
		if len(advertisers.spent) != 0 {
			t.Error("advertiser spend should not be called if campaign commit failed")
		}
	})

	t.Run("advertiser spend fails", func(t *testing.T) {
		campaigns := &fakeCampaignRepo{
			campaigns: map[string]*domain.Campaign{
				"camp_1": testCampaignPtr("camp_1", "adv_1", "RU"),
			},
		}
		advertisers := &fakeAdvertiserRepo{
			spendErr: domain.ErrInsufficientBalance,
		}
		svc := New(campaigns, nil, advertisers, 150)

		err := svc.Commit(context.Background(), "camp_1", 1_500_000)
		if !errors.Is(err, domain.ErrInsufficientBalance) {
			t.Errorf("want ErrInsufficientBalance, got %v", err)
		}
	})
}
