package reserve

import (
	"context"
	"errors"

	"github.com/7cout/minissp/internal/ssp/domain"
)

// ErrNotFound — записи нет в резерве: TTL истёк, Impression уже
// обработан, или ID некорректный.
var ErrNotFound = errors.New("reserve: not found")

// ConsumeStatus — результат Consume.
type ConsumeStatus int

const (
	// ConsumeFresh — запись только что взята из резерва, можно обрабатывать.
	ConsumeFresh ConsumeStatus = iota
	// ConsumeAlready — запись уже была обработана (идемпотентный повтор).
	ConsumeAlready
)

// ExpiredHandler обрабатывает запись, у которой истёк TTL.
//
// Возвращает ошибку, если откат не удался — Manager вернёт запись
// в резерв для повторной попытки.
type ExpiredHandler func(record domain.AuctionRecord) error

// Manager управляет активными резервами (записями аукционов).
//
// Инвариант: для одного auction_id Consume успешно срабатывает
// ровно один раз. Параллельные вызовы Consume и Tick (обработка TTL)
// не могут «украсть» запись друг у друга.
type Manager interface {
	// Reserve сохраняет запись до Impression или TTL.
	Reserve(ctx context.Context, record domain.AuctionRecord) error

	// Consume атомарно забирает запись из резерва.
	//
	// Возвращает:
	//   - (record, ConsumeFresh, nil)  — запись взята;
	//   - (_, ConsumeAlready, nil)     — идемпотентный повтор;
	//   - (_, 0, ErrNotFound)          — записи нет.
	Consume(ctx context.Context, auctionID string) (domain.AuctionRecord, ConsumeStatus, error)

	// Restore возвращает запись в резерв — используется, если после
	// Consume обработка не удалась и запись надо повторить.
	Restore(ctx context.Context, record domain.AuctionRecord) error

	// Subscribe запускает фоновую обработку просроченных резервов.
	//
	// Не блокирует: запускает воркер в отдельной горутине и
	// возвращает управление. Воркер останавливается при отмене ctx.
	Subscribe(ctx context.Context, onExpired ExpiredHandler)

	// Tick обрабатывает просроченные резервы синхронно, один проход.
	//
	// Используется в тестах и для ручной отладки. В проде воркер
	// из Subscribe вызывает эту же логику по тикеру (для memory)
	// или по событию (для Redis — там Tick может быть no-op).
	Tick(ctx context.Context, onExpired ExpiredHandler)

	// Peek возвращает запись из резерва без её изъятия.
	//
	// Если записи нет — ErrNotFound.
	//
	// Диагностический метод: в бизнес-логике не используется,
	// нужен тестам и ручной отладке.
	Peek(ctx context.Context, auctionID string) (domain.AuctionRecord, error)
}
