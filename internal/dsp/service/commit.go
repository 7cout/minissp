package service

import (
	"context"
	"fmt"

	dspmetrics "github.com/7cout/minissp/internal/dsp/metrics"
)

// Commit подтверждает списание после показа.
func (s *Service) Commit(ctx context.Context, campaignID string, price int64) (err error) {
	defer func() {
		dspmetrics.CommitsTotal.WithLabelValues(statusLabel(err)).Inc()
	}()

	if err := s.txManager.CommitWithSpend(ctx, campaignID, price); err != nil {
		return fmt.Errorf("commit campaign %s: %w", campaignID, err)
	}
	return nil
}

// statusLabel превращает ошибку в метку "ok"/"error".
func statusLabel(err error) string {
	if err == nil {
		return "ok"
	}
	return "error"
}
