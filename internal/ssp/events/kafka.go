package events

import (
	"context"
	"fmt"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"
)

// KafkaOptions — параметры Kafka-клиента.
type KafkaOptions struct {
	Brokers  []string
	ClientID string
}

// KafkaPublisher — Publisher поверх franz-go.
//
// franz-go сам управляет переподключениями: если брокер недоступен
// при старте, Create не падает, а первая публикация вернёт ошибку
// после таймаута. Это то, что нам нужно: outbox подождёт.
type KafkaPublisher struct {
	client *kgo.Client
}

// NewKafka создаёт publisher.
//
// Не пытается подключиться к брокеру сразу — это ленивая операция.
// Проверить доступность можно методом Ping.
func NewKafka(opts KafkaOptions) (*KafkaPublisher, error) {
	if len(opts.Brokers) == 0 {
		return nil, fmt.Errorf("kafka: no brokers configured")
	}
	if opts.ClientID == "" {
		opts.ClientID = "ssp"
	}

	client, err := kgo.NewClient(
		kgo.SeedBrokers(opts.Brokers...),
		kgo.ClientID(opts.ClientID),
		// Ждём подтверждения от всех синхронных реплик.
		kgo.RequiredAcks(kgo.AllISRAcks()),
		// Сжатие — snappy: разумный компромисс скорость/размер.
		kgo.ProducerBatchCompression(kgo.SnappyCompression()),
		// Не копим батчи: у нас по одной записи за раз.
		kgo.ProducerLinger(0),
		// Идемпотентный producer включён по умолчанию в franz-go:
		// защищает от дублей при retry внутри клиента.
	)
	if err != nil {
		return nil, fmt.Errorf("create kafka client: %w", err)
	}
	return &KafkaPublisher{client: client}, nil
}

// Ping проверяет доступность брокера. Используется при старте сервиса
// для лога — не блокирует запуск.
func (p *KafkaPublisher) Ping(ctx context.Context) error {
	pingCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	return p.client.Ping(pingCtx)
}

// Publish отправляет запись и ждёт подтверждения.
func (p *KafkaPublisher) Publish(ctx context.Context, topic, key string, payload []byte) error {
	rec := &kgo.Record{
		Topic: topic,
		Key:   []byte(key),
		Value: payload,
	}
	if err := p.client.ProduceSync(ctx, rec).FirstErr(); err != nil {
		return fmt.Errorf("produce %s: %w", topic, err)
	}
	return nil
}

// Close закрывает клиент, дожидаясь отправки буферизованных сообщений.
func (p *KafkaPublisher) Close() error {
	p.client.Close()
	return nil
}
