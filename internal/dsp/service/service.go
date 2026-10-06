package service

import (
	"context"

	"github.com/7cout/minissp/internal/dsp/domain"
)

// CampaignRepository — доступ к кампаниям.
type CampaignRepository interface {
	Get(ctx context.Context, id string) (*domain.Campaign, error)
	ListByGeo(ctx context.Context, geo string) ([]domain.Campaign, error)
	Reserve(ctx context.Context, campaignID string, amount int64) error
	Commit(ctx context.Context, campaignID string, amount int64) error
	Rollback(ctx context.Context, campaignID string, amount int64) error
	Uncommit(ctx context.Context, campaignID string, amount int64) error
}

// CreativeRepository — доступ к креативам.
type CreativeRepository interface {
	ListByCampaign(ctx context.Context, campaignID string) ([]domain.Creative, error)
}

// AdvertiserRepository — доступ к рекламодателям.
type AdvertiserRepository interface {
	Get(ctx context.Context, id string) (*domain.Advertiser, error)
	Spend(ctx context.Context, id string, amount int64) error
}

// Service — сервис DSP.
type Service struct {
	campaigns   CampaignRepository
	creatives   CreativeRepository
	advertisers AdvertiserRepository

	// bidMultiplierPercent — насколько DSP готов поставить больше floor.
	// Например, 150 = готов платить в 1.5 раза больше минимальной цены.
	// Разные значения — разные стратегии у разных DSP.
	bidMultiplierPercent int64
}

// New создаёт сервис DSP.
func New(
	campaigns CampaignRepository,
	creatives CreativeRepository,
	advertisers AdvertiserRepository,
	bidMultiplierPercent int64,
) *Service {
	return &Service{
		campaigns:            campaigns,
		creatives:            creatives,
		advertisers:          advertisers,
		bidMultiplierPercent: bidMultiplierPercent,
	}
}
