// Package cache содержит кэш слотов SSP.
//
// Используется как cache-aside поверх SlotRepository: сначала
// смотрим в кэш, при промахе идём в репозиторий и кладём результат
// в кэш. Падение кэша не ломает бизнес — это деградация, не отказ.
//
// Две реализации:
//   - NoopSlotCache — пустышка, для STORAGE=memory. Репозиторий
//     и так в памяти, кэш ничего не ускорит.
//   - RedisSlotCache — Redis с TTL. Для STORAGE=postgres.
package cache
