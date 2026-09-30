// Package service содержит бизнес-логику аукциона: проведение
// аукциона, сбор ставок, выбор победителя.
//
// Сервис не знает про транспорт (gRPC, HTTP) и хранилища
// (PostgreSQL, Redis). Он работает через интерфейсы
package service

import (
	"context"

	"github.com/7cout/minissp/internal/auction/domain"
)

// SlotRepository - доступ к слотам
type SlotRepository interface {
	// Get возвращает слот по ID.
	// Если слот не найден — возвращает domain.ErrSlotNotFound
	Get(ctx context.Context, id string) (*domain.Slot, error)
}

// CampaignRepository - доступ к кампаниям.
type CampaignRepository interface {
	// ListByGeo возвращает активные кампании, подходящие по гео
	ListByGeo(ctx context.Context, geo string) ([]domain.Campaign, error)

	// Reserve резервирует amount на бюджете кампании
	// Если бюджета не хватает — возвращает domain.ErrInsufficientBudget
	Reserve(ctx context.Context, campaignID string, amount int64) error
}

// CreativeRepository - доступ к креативам
type CreativeRepository interface {
	// ListByCampaign возвращает все креативы кампании
	ListByCampaign(ctx context.Context, campaignID string) ([]domain.Creative, error)
}

// BidderClient - клиент биддера
type BidderClient interface {
	// GetBid запрашивает ставку у биддера
	// Может вернуть ошибку, если биддер не ответил или отказался
	GetBid(ctx context.Context, req domain.BidRequest) (*domain.Bid, error)
}

// Service - сервис аукциона
type Service struct {
	slots     SlotRepository
	campaigns CampaignRepository
	creatives CreativeRepository
	bidders   []BidderClient
}

// New создаёт сервис аукциона
func New(
	slots SlotRepository,
	campaigns CampaignRepository,
	creatives CreativeRepository,
	bidders []BidderClient,
) *Service {
	return &Service{
		slots:     slots,
		campaigns: campaigns,
		creatives: creatives,
		bidders:   bidders,
	}
}
