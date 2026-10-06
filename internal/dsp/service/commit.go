package service

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/7cout/minissp/internal/dsp/domain"
)

// Commit подтверждает списание после показа.
//
// Выполняет два действия:
//  1. Снимает резерв с бюджета кампании.
//  2. Списывает сумму с баланса рекламодателя.
//
// Проверяет баланс рекламодателя заранее и компенсирует Commit
// кампании, если Spend упал.
//
// ВАЖНО: это не транзакция. При переходе на PostgreSQL операция
// станет одной транзакцией BEGIN; UPDATE campaigns; UPDATE advertisers; COMMIT,
// и весь код с компенсацией ниже будет не нужен.
func (s *Service) Commit(ctx context.Context, campaignID string, price int64) error {
	campaign, err := s.campaigns.Get(ctx, campaignID)
	if err != nil {
		return fmt.Errorf("get campaign %s: %w", campaignID, err)
	}

	// Проверяем баланс рекламодателя заранее — отсекаем большинство
	// ошибок Spend до мутаций. Не закрывает race, но в сочетании
	// с компенсацией ниже даёт приемлемую гарантию для in-memory.
	advertiser, err := s.advertisers.Get(ctx, campaign.AdvertiserID)
	if err != nil {
		return fmt.Errorf("get advertiser %s: %w", campaign.AdvertiserID, err)
	}
	if advertiser.Balance < price {
		return fmt.Errorf("advertiser %s: %w", campaign.AdvertiserID, domain.ErrInsufficientBalance)
	}

	if err := s.campaigns.Commit(ctx, campaignID, price); err != nil {
		return fmt.Errorf("commit campaign %s: %w", campaignID, err)
	}

	if err := s.advertisers.Spend(ctx, campaign.AdvertiserID, price); err != nil {
		// Компенсация: возвращаем деньги в reserved кампании.
		if unErr := s.campaigns.Uncommit(ctx, campaignID, price); unErr != nil {
			slog.ErrorContext(ctx, "CRITICAL: reserved lost",
				"campaign_id", campaignID,
				"advertiser_id", campaign.AdvertiserID,
				"price", price,
				"commit_error", err,
				"uncommit_error", unErr,
			)
		}
		return fmt.Errorf("spend advertiser %s: %w", campaign.AdvertiserID, err)
	}

	return nil
}
