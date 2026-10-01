// Package memory содержит in-memory реализации репозиториев
// аукциона. Используется для разработки и тестов — там, где
// не нужны реальные PostgreSQL и Redis
//
// # Данные живут в памяти процесса и теряются при перезапуске
//
// Возвращает те же доменные ошибки, что и postgres-репозитории:
// ErrSlotNotFound, ErrCampaignNotFound, ErrInsufficientBudget.
package memory
