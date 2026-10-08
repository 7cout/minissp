package service

import (
	"context"
	"errors"
	"testing"

	"github.com/7cout/minissp/internal/dsp/domain"
)

func TestService_Rollback(t *testing.T) {
	t.Run("happy path", func(t *testing.T) {
		campaigns := &fakeCampaignRepo{}
		svc := New(campaigns, nil, nil, 150)

		err := svc.Rollback(context.Background(), "camp_1", 1_500_000)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(campaigns.rolledBack) != 1 {
			t.Fatalf("want 1 rollback call, got %d", len(campaigns.rolledBack))
		}
		if campaigns.rolledBack[0].campaignID != "camp_1" {
			t.Errorf("rolled back campaign = %q", campaigns.rolledBack[0].campaignID)
		}
		if campaigns.rolledBack[0].amount != 1_500_000 {
			t.Errorf("rolled back amount = %d, want 1500000", campaigns.rolledBack[0].amount)
		}
	})

	t.Run("campaign not found", func(t *testing.T) {
		campaigns := &fakeCampaignRepo{
			rollbackErr: domain.ErrCampaignNotFound,
		}
		svc := New(campaigns, nil, nil, 150)

		err := svc.Rollback(context.Background(), "missing", 100)
		if !errors.Is(err, domain.ErrCampaignNotFound) {
			t.Errorf("want ErrCampaignNotFound, got %v", err)
		}
	})

	t.Run("insufficient reserved", func(t *testing.T) {
		campaigns := &fakeCampaignRepo{
			rollbackErr: domain.ErrInsufficientBudget,
		}
		svc := New(campaigns, nil, nil, 150)

		err := svc.Rollback(context.Background(), "camp_1", 100)
		if !errors.Is(err, domain.ErrInsufficientBudget) {
			t.Errorf("want ErrInsufficientBudget, got %v", err)
		}
	})
}
