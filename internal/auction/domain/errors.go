package domain

import "errors"

// Sentinel-ошибки домена. Проверяются через errors.Is.
var (
	// ErrSlotNotFound — слот с таким ID не найден.
	ErrSlotNotFound = errors.New("slot not found")

	// ErrCampaignNotFound — кампания с таким ID не найдена.
	ErrCampaignNotFound = errors.New("campaign not found")

	// ErrCreativeNotFound — креатив с таким ID не найден.
	ErrCreativeNotFound = errors.New("creative not found")

	// ErrNoBids — в аукционе не участвовало ни одной ставки.
	ErrNoBids = errors.New("no bids received")

	// ErrInsufficientBudget — у кампании не хватает бюджета.
	ErrInsufficientBudget = errors.New("insufficient budget")

	// ErrUnsupportedGeo — гео не поддерживается платформой.
	ErrUnsupportedGeo = errors.New("unsupported geo")
)
