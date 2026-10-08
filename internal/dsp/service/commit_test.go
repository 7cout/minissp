package service

import (
	"context"
	"errors"
	"testing"

	"github.com/7cout/minissp/internal/dsp/domain"
)

func TestService_Commit(t *testing.T) {
	t.Run("happy path — delegates to tx manager", func(t *testing.T) {
		tx := &fakeTxManager{}
		svc := New(&fakeCampaignRepo{}, nil, tx, 150)

		err := svc.Commit(context.Background(), "camp_1", 1_500_000)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(tx.commitCalls) != 1 {
			t.Fatalf("want 1 commit call, got %d", len(tx.commitCalls))
		}
		if tx.commitCalls[0].campaignID != "camp_1" {
			t.Errorf("campaign = %q, want camp_1", tx.commitCalls[0].campaignID)
		}
		if tx.commitCalls[0].amount != 1_500_000 {
			t.Errorf("price = %d, want 1500000", tx.commitCalls[0].amount)
		}
	})

	t.Run("tx manager error is wrapped", func(t *testing.T) {
		tx := &fakeTxManager{commitErr: domain.ErrInsufficientBalance}
		svc := New(&fakeCampaignRepo{}, nil, tx, 150)

		err := svc.Commit(context.Background(), "camp_1", 1_500_000)
		if !errors.Is(err, domain.ErrInsufficientBalance) {
			t.Errorf("want ErrInsufficientBalance, got %v", err)
		}
	})
}
