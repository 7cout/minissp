package service

import (
	"context"
	"fmt"
)

// Commit подтверждает списание после показа.
//
// Делегирует в TransactionManager — там всё в одной транзакции:
// снять резерв с кампании и списать деньги с advertiser'а.
func (s *Service) Commit(ctx context.Context, campaignID string, price int64) error {
	if err := s.txManager.CommitWithSpend(ctx, campaignID, price); err != nil {
		return fmt.Errorf("commit campaign %s: %w", campaignID, err)
	}
	return nil
}
