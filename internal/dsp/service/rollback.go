package service

import (
	"context"
	"fmt"
)

// Rollback отменяет резерв, возвращая деньги в бюджет кампании.
//
// Используется, когда impression не пришёл в срок.
func (s *Service) Rollback(ctx context.Context, campaignID string, price int64) error {
	if err := s.campaigns.Rollback(ctx, campaignID, price); err != nil {
		return fmt.Errorf("rollback campaign %s: %w", campaignID, err)
	}
	return nil
}
