// Package reserve управляет активными резервами аукционов SSP.
//
// Между RunAuction и Impression (или истечением TTL) SSP держит
// AuctionRecord — кто победил, сколько должен заплатить, какому
// биддеру отправлять Commit.
//
// Жизненный цикл записи:
//
//	RunAuction:  Reserve(record)             → в резерве
//	Impression:  Consume(id) = ConsumeFresh  → в обработке
//	             Commit+AddBalance успешно   → processed (идемпотентный повтор безопасен)
//	             Commit/AddBalance упал      → Restore(record)
//	TTL истёк:   Tick/Subscribe → onExpired  → Rollback в биддер
//
// Гарантия: для одного auction_id Consume успешно срабатывает ровно
// один раз, даже при гонке Impression и TTL-воркера.
//
// Две реализации: MemoryManager (для STORAGE=memory и тестов) и
// RedisManager (для STORAGE=postgres, переживает рестарт сервиса).
package reserve
