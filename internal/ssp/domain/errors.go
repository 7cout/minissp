package domain

import "errors"

// Sentinel-ошибки SSP. Проверяются через errors.Is.
var (
	// ErrSlotNotFound — слот с таким ID или именем не найден.
	ErrSlotNotFound = errors.New("slot not found")

	// ErrSlotAlreadyExists — слот с таким именем уже существует.
	ErrSlotAlreadyExists = errors.New("slot already exists")

	// ErrPublisherNotFound — издатель не найден.
	ErrPublisherNotFound = errors.New("publisher not found")

	// ErrNoBids — ни один биддер не ответил или все отказались.
	ErrNoBids = errors.New("no bids received")

	// ErrAuctionNotFound — аукцион с таким ID не найден.
	ErrAuctionNotFound = errors.New("auction not found")

	// ErrInvalidID — ID не соответствует формату.
	ErrInvalidID = errors.New("invalid id format")

	// ErrUnauthenticated — api-key невалиден или отсутствует.
	ErrUnauthenticated = errors.New("unauthenticated")
)
