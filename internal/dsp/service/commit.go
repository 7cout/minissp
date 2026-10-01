package service

import (
	"context"
	"fmt"
)

// Commit подтверждает списание после показа.
//
// Выполняет два действия:
//  1. Снимает резерв с бюджета кампании.
//  2. Списывает сумму с баланса рекламодателя.
//
// Возвращает ошибку, если кампания не найдена, резерва не хватает
// или у рекламодателя недостаточно средств.
func (s *Service) Commit(ctx context.Context, campaignID string, price int64) error {
	// 1. Достаём кампанию, чтобы знать advertiser_id.
	campaign, err := s.campaigns.Get(ctx, campaignID)
	if err != nil {
		return fmt.Errorf("get campaign %s: %w", campaignID, err)
	}

	// 2. Снимаем резерв у кампании.
	if err := s.campaigns.Commit(ctx, campaignID, price); err != nil {
		return fmt.Errorf("commit campaign %s: %w", campaignID, err)
	}

	// 3. Списываем с баланса рекламодателя.
	if err := s.advertisers.Spend(ctx, campaign.AdvertiserID, price); err != nil {
		return fmt.Errorf("spend advertiser %s: %w", campaign.AdvertiserID, err)
	}

	return nil
}
