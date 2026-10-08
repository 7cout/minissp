package service

import (
	"context"

	"github.com/7cout/minissp/internal/dsp/domain"
)

// CampaignRepository — доступ к кампаниям.
//
// Commit и Uncommit убраны: атомарная связка «снять резерв +
// списать с advertiser'а» вынесена в TransactionManager.
type CampaignRepository interface {
	Get(ctx context.Context, id string) (*domain.Campaign, error)
	ListByGeo(ctx context.Context, geo string) ([]domain.Campaign, error)
	Reserve(ctx context.Context, campaignID string, amount int64) error
	Rollback(ctx context.Context, campaignID string, amount int64) error
}

// CreativeRepository — доступ к креативам.
type CreativeRepository interface {
	ListByCampaign(ctx context.Context, campaignID string) ([]domain.Creative, error)
}

// TransactionManager выполняет составные операции в одной транзакции.
type TransactionManager interface {
	// CommitWithSpend атомарно снимает резерв с кампании и списывает
	// деньги с её advertiser'а.
	//
	// Возможные ошибки: ErrCampaignNotFound, ErrInsufficientBudget,
	// ErrAdvertiserNotFound, ErrInsufficientBalance.
	CommitWithSpend(ctx context.Context, campaignID string, price int64) error
}

// Service — сервис DSP.
type Service struct {
	campaigns CampaignRepository
	creatives CreativeRepository
	txManager TransactionManager

	// bidMultiplierPercent — насколько DSP готов поставить больше floor.
	bidMultiplierPercent int64
}

// New создаёт сервис DSP.
func New(
	campaigns CampaignRepository,
	creatives CreativeRepository,
	txManager TransactionManager,
	bidMultiplierPercent int64,
) *Service {
	return &Service{
		campaigns:            campaigns,
		creatives:            creatives,
		txManager:            txManager,
		bidMultiplierPercent: bidMultiplierPercent,
	}
}
