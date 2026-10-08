package service

import (
	"context"
	"fmt"

	dspmetrics "github.com/7cout/minissp/internal/dsp/metrics"
)

// Rollback отменяет резерв, возвращая деньги в бюджет кампании.
func (s *Service) Rollback(ctx context.Context, campaignID string, price int64) (err error) {
	defer func() {
		dspmetrics.RollbacksTotal.WithLabelValues(statusLabel(err)).Inc()
	}()

	if err := s.campaigns.Rollback(ctx, campaignID, price); err != nil {
		return fmt.Errorf("rollback campaign %s: %w", campaignID, err)
	}
	return nil
}
