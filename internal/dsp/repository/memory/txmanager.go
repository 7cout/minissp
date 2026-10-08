package memory

import (
	"context"
)

// TxManager выполняет составные операции над memory-репозиториями
// под одной критической секцией.
type TxManager struct {
	campaigns   *CampaignRepo
	advertisers *AdvertiserRepo
}

// NewTxManager создаёт менеджер транзакций.
func NewTxManager(campaigns *CampaignRepo, advertisers *AdvertiserRepo) *TxManager {
	return &TxManager{
		campaigns:   campaigns,
		advertisers: advertisers,
	}
}

// CommitWithSpend атомарно снимает резерв с кампании и списывает
// деньги с её advertiser'а.
//
// Захватывает мьютексы в фиксированном порядке (campaigns → advertisers),
// чтобы избежать deadlock между параллельными вызовами.
func (m *TxManager) CommitWithSpend(_ context.Context, campaignID string, price int64) error {
	m.campaigns.mu.Lock()
	defer m.campaigns.mu.Unlock()

	m.advertisers.mu.Lock()
	defer m.advertisers.mu.Unlock()

	advertiserID, err := m.campaigns.commitLocked(campaignID, price)
	if err != nil {
		return err
	}

	if err := m.advertisers.spendLocked(advertiserID, price); err != nil {
		// Резерв уже снят. Возвращаем его на место.
		if c, ok := m.campaigns.campaigns[campaignID]; ok {
			c.BudgetReserved += price
		}
		return err
	}
	return nil
}
