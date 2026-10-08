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
//	             Commit+AddBalance успешно   → processed (повтор безопасен)
//	             Commit/AddBalance упал      → Restore(record)
//	TTL истёк:   Tick/Subscribe → onExpired  → Rollback в биддер
//
// Гарантия: для одного auction_id Consume успешно срабатывает ровно
// один раз, даже при гонке Impression и TTL-воркера.
//
// Реализации:
//   - MemoryManager — in-memory, для STORAGE=memory и тестов;
//   - RedisManager  — Redis, для STORAGE=postgres; резервы переживают
//     рестарт сервиса и корректно работают при нескольких инстансах SSP.
//
// RedisManager использует Lua-скрипты для атомарности и sorted set
// для отслеживания дедлайнов. См. комментарии в redis.go.
package reserve
