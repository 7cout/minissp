// Command ssp запускает gRPC-сервер SSP (Supply-Side Platform).
//
// Хранилище выбирается env STORAGE: memory (по умолчанию) или postgres.
// Для postgres seed не заливается автоматически — используй cmd/seed.
//
// Зависимости от STORAGE:
//
//	STORAGE=memory:
//	  - репозитории: in-memory
//	  - слоты в кэше: нет (NoopSlotCache)
//	  - резервы: MemoryManager (живут в памяти процесса)
//	  - события: MemoryStore + NoopPublisher (публикации нет)
//
//	STORAGE=postgres:
//	  - репозитории: PostgreSQL
//	  - слоты в кэше: Redis (RedisSlotCache, TTL 5 минут)
//	  - резервы: RedisManager (переживают рестарт, работают при нескольких инстансах)
//	  - события: ssp.event_outbox + Kafka (franz-go)
//
// При STORAGE=postgres Redis и Kafka обязательны. Без Redis Impression
// не может быть идемпотентным, а воркер не откатывает зависшие аукционы.
// Без Kafka события Impression копятся в outbox и никогда не публикуются.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	"github.com/7cout/minissp/internal/db"
	"github.com/7cout/minissp/internal/metrics"
	"github.com/7cout/minissp/internal/ssp/bidder"
	"github.com/7cout/minissp/internal/ssp/cache"
	"github.com/7cout/minissp/internal/ssp/events"
	"github.com/7cout/minissp/internal/ssp/handler"
	"github.com/7cout/minissp/internal/ssp/repository/memory"
	ssppostgres "github.com/7cout/minissp/internal/ssp/repository/postgres"
	"github.com/7cout/minissp/internal/ssp/reserve"
	"github.com/7cout/minissp/internal/ssp/seed"
	"github.com/7cout/minissp/internal/ssp/service"
	pb "github.com/7cout/minissp/proto/gen/ssp/v1"
)

const (
	defaultAddr     = ":50051"
	defaultMetrics  = ":9100"
	defaultRedis    = "localhost:6379"
	shutdownTimeout = 10 * time.Second
	warmUpTimeout   = 3 * time.Second
	slotCacheTTL    = 5 * time.Minute
)

func main() {
	if err := run(); err != nil {
		slog.Error("server failed", "error", err)
		os.Exit(1)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// 1. Зависимости: репозитории, кэш, резервы, события.
	d, err := buildDeps(ctx)
	if err != nil {
		return fmt.Errorf("build deps: %w", err)
	}
	defer d.cleanup()

	// 2. Metrics-сервер: HTTP на отдельном порту, чтобы не мешать gRPC.
	metricsAddr := getEnv("METRICS_ADDR", defaultMetrics)
	metricsErrCh := make(chan error, 1)
	go func() {
		if err := metrics.Serve(ctx, metricsAddr); err != nil {
			metricsErrCh <- err
		}
	}()

	// 3. Клиенты к DSP.
	bidders, err := buildBidders(ctx)
	if err != nil {
		return fmt.Errorf("build bidders: %w", err)
	}
	slog.Info("dsp clients ready", "count", len(bidders))

	// 4. Сервис SSP.
	svc := service.New(service.Options{
		Slots:      d.slots,
		Publishers: d.publishers,
		Bidders:    bidders,
		SlotCache:  d.slotCache,
		Reserve:    d.reserve,
		TxManager:  d.txManager,
	})
	defer func() {
		if err := svc.Close(); err != nil {
			slog.Error("close service", "error", err)
		}
	}()

	// 5. Worker отката просроченных резервов.
	svc.StartReserveWorker(ctx)

	// 6. Worker публикации событий из outbox в Kafka.
	evtCfg := events.DefaultWorkerConfig()
	if v := getEnvInt("EVENTS_BATCH_SIZE", 0); v > 0 {
		evtCfg.BatchSize = v
	}
	if v := getEnvDuration("EVENTS_POLL_INTERVAL", 0); v > 0 {
		evtCfg.PollInterval = v
	}
	evtWorker := events.NewWorker(d.eventStore, d.publisher, evtCfg)
	go evtWorker.Run(ctx)

	// 7. gRPC-сервер с auth interceptor.
	authResolver := newPublisherResolver(d.publishers)
	grpcServer := grpc.NewServer(
		grpc.UnaryInterceptor(handler.AuthInterceptor(authResolver)),
	)
	pb.RegisterSspServiceServer(grpcServer, handler.NewSspServer(svc))
	reflection.Register(grpcServer)

	// 8. Слушаем.
	addr := getEnv("SSP_ADDR", defaultAddr)
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("listen %s: %w", addr, err)
	}

	// 9. Запуск.
	errCh := make(chan error, 1)
	go func() {
		slog.Info("SSP gRPC server listening", "addr", addr)
		if err := grpcServer.Serve(lis); err != nil {
			errCh <- err
		}
	}()

	// 10. Ждём сигнала или ошибки.
	select {
	case <-ctx.Done():
		slog.Info("shutdown signal received")
	case err := <-errCh:
		return err
	case err := <-metricsErrCh:
		// Метрики — вспомогательная функция. Если упали, сервис
		// продолжает работать. Логируем и идём дальше.
		slog.Error("metrics server failed", "error", err)
	}

	// 11. Graceful shutdown.
	done := make(chan struct{})
	go func() {
		grpcServer.GracefulStop()
		close(done)
	}()

	select {
	case <-done:
		slog.Info("server stopped gracefully")
	case <-time.After(shutdownTimeout):
		slog.Warn("graceful shutdown timeout, forcing stop")
		grpcServer.Stop()
	}

	return nil
}

// deps — собранные зависимости SSP.
type deps struct {
	slots      service.SlotRepository
	publishers service.PublisherRepository
	slotCache  cache.SlotCache
	reserve    reserve.Manager
	txManager  service.TxManager
	eventStore events.Store
	publisher  events.Publisher
	cleanup    func()
}

// buildDeps выбирает реализацию репозиториев, кэша, резервов
// и событий по env STORAGE.
func buildDeps(ctx context.Context) (deps, error) {
	switch getEnv("STORAGE", "memory") {
	case "postgres":
		return buildPostgresDeps(ctx)
	default:
		return buildMemoryDeps(ctx)
	}
}

// buildPostgresDeps собирает продакшен-конфигурацию: Postgres + Redis + Kafka.
//
// Redis и Kafka обязательны. Если что-то недоступно — возвращаем ошибку,
// чтобы сервис не стартовал в полурабочем состоянии. Иначе получится
// SSP, который принимает Impression, но не может откатить просроченный
// резерв (Redis) или теряет все события аналитики (Kafka).
func buildPostgresDeps(ctx context.Context) (deps, error) {
	pgCfg := db.PostgresConfig{
		Host:     getEnv("POSTGRES_HOST", "localhost"),
		Port:     getEnv("POSTGRES_PORT", "5432"),
		User:     getEnv("POSTGRES_USER", "minissp"),
		Password: getEnv("POSTGRES_PASSWORD", ""),
		Database: getEnv("POSTGRES_DB", "minissp"),
	}
	pool, err := db.NewPostgresPool(ctx, pgCfg)
	if err != nil {
		return deps{}, fmt.Errorf("postgres pool: %w", err)
	}
	slog.Info("storage: postgres")

	redisAddr := getEnv("REDIS_ADDR", defaultRedis)
	redisClient, err := db.NewRedisClient(ctx, db.RedisConfig{Addr: redisAddr})
	if err != nil {
		pool.Close()
		return deps{}, fmt.Errorf("redis %s: %w", redisAddr, err)
	}
	slog.Info("redis ready", "addr", redisAddr)

	// Kafka обязательна для STORAGE=postgres: без неё события
	// копятся в outbox и никогда не публикуются.
	brokers := splitEnvList("KAFKA_BROKERS")
	if len(brokers) == 0 {
		_ = redisClient.Close()
		pool.Close()
		return deps{}, fmt.Errorf("KAFKA_BROKERS is required for STORAGE=postgres")
	}
	publisher, err := events.NewKafka(events.KafkaOptions{
		Brokers:  brokers,
		ClientID: "ssp",
	})
	if err != nil {
		_ = redisClient.Close()
		pool.Close()
		return deps{}, fmt.Errorf("kafka: %w", err)
	}
	slog.Info("kafka publisher ready", "brokers", brokers)

	// Проверяем доступность брокера. Не блокируем запуск:
	// если брокер поднимется позже, первая публикация подождёт.
	if err := publisher.Ping(ctx); err != nil {
		slog.Warn("kafka ping failed — first publish may be slow",
			"error", err,
		)
	} else {
		slog.Info("kafka broker reachable")
	}

	eventStore := events.NewPostgresStore(pool)
	txManager := ssppostgres.NewTxManager(pool)

	return deps{
		slots:      ssppostgres.NewSlotRepo(pool),
		publishers: ssppostgres.NewPublisherRepo(pool),
		slotCache:  cache.NewRedisSlotCache(redisClient, slotCacheTTL),
		reserve:    reserve.NewRedis(redisClient, reserve.DefaultRedisOptions()),
		txManager:  txManager,
		eventStore: eventStore,
		publisher:  publisher,
		cleanup: func() {
			_ = publisher.Close()
			_ = redisClient.Close()
			pool.Close()
		},
	}, nil
}

// buildMemoryDeps собирает dev-конфигурацию: всё в памяти.
//
// Redis и Kafka не нужны — репозитории in-memory, резервы
// в MemoryManager, события в MemoryStore и никуда не публикуются.
// Данные теряются при перезапуске сервиса.
func buildMemoryDeps(ctx context.Context) (deps, error) {
	slog.Info("storage: memory")

	publishers := memory.NewPublisherRepo()
	slots := memory.NewSlotRepo()
	if err := seed.Populate(ctx, publishers, slots); err != nil {
		return deps{}, fmt.Errorf("seed memory: %w", err)
	}
	slog.Info("seed data loaded", "publishers", 1, "slots", 1)

	eventStore := events.NewMemoryStore()

	return deps{
		slots:      slots,
		publishers: publishers,
		slotCache:  cache.NoopSlotCache{},
		reserve:    reserve.NewMemory(reserve.DefaultMemoryOptions()),
		txManager:  memory.NewTxManager(publishers, eventStore),
		eventStore: eventStore,
		publisher:  events.NoopPublisher{},
		cleanup:    func() {},
	}, nil
}

// buildBidders создаёт gRPC-клиентов ко всем известным DSP
// и прогревает соединения.
func buildBidders(ctx context.Context) ([]service.BidderClient, error) {
	type dspConfig struct {
		name   string
		addr   string
		apiKey string
	}

	configs := []dspConfig{
		{
			name:   "dsp-nike",
			addr:   getEnv("DSP_NIKE_ADDR", "127.0.0.1:50052"),
			apiKey: getEnv("DSP_API_KEY", ""),
		},
	}

	var clients []service.BidderClient
	for _, cfg := range configs {
		c, err := bidder.NewClient(cfg.name, cfg.addr, cfg.apiKey)
		if err != nil {
			for _, cl := range clients {
				_ = cl.Close()
			}
			return nil, fmt.Errorf("create bidder %s: %w", cfg.name, err)
		}

		warmUpCtx, cancel := context.WithTimeout(ctx, warmUpTimeout)
		err = c.WarmUp(warmUpCtx)
		cancel()

		if err != nil {
			slog.Warn("dsp warm up failed — first RPC may be slow",
				"dsp", cfg.name,
				"addr", cfg.addr,
				"error", err,
			)
		} else {
			slog.Info("dsp connection ready", "dsp", cfg.name)
		}

		clients = append(clients, c)
	}

	return clients, nil
}

// publisherResolver адаптирует PublisherRepository к интерфейсу
// handler.PublisherResolver.
type publisherResolver struct {
	repo service.PublisherRepository
}

func newPublisherResolver(repo service.PublisherRepository) *publisherResolver {
	return &publisherResolver{repo: repo}
}

func (r *publisherResolver) Resolve(ctx context.Context, apiKey string) (string, error) {
	p, err := r.repo.GetByAPIKey(ctx, apiKey)
	if err != nil {
		return "", err
	}
	return p.ID, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// splitEnvList читает переменную окружения как список значений,
// разделённых запятыми. Пустые элементы и пробелы игнорируются.
//
// Пример: KAFKA_BROKERS="kafka-1:9092,kafka-2:9092" → ["kafka-1:9092", "kafka-2:9092"].
func splitEnvList(key string) []string {
	v := os.Getenv(key)
	if v == "" {
		return nil
	}
	parts := strings.Split(v, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if trimmed := strings.TrimSpace(p); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func getEnvDuration(key string, fallback time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}
