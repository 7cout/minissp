package domain

import "errors"

// Sentinel-ошибки DSP. Проверяются через errors.Is.
var (
	// ErrCampaignNotFound — кампания с таким ID не найдена.
	ErrCampaignNotFound = errors.New("campaign not found")

	// ErrCreativeNotFound — креатив с таким ID не найден.
	ErrCreativeNotFound = errors.New("creative not found")

	// ErrNoEligibleCampaign — ни одна кампания не подходит для показа.
	ErrNoEligibleCampaign = errors.New("no eligible campaign")

	// ErrInsufficientBudget — у кампании не хватает бюджета.
	ErrInsufficientBudget = errors.New("insufficient budget")

	// ErrInvalidID — ID не соответствует формату.
	ErrInvalidID = errors.New("invalid id format")

	// ErrAdvertiserNotFound — рекламодатель с таким ID не найден.
	ErrAdvertiserNotFound = errors.New("advertiser not found")

	// ErrInsufficientBalance — на балансе рекламодателя не хватает денег.
	ErrInsufficientBalance = errors.New("insufficient balance")
)
