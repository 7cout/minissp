// Package metrics запускает HTTP-сервер для экспорта метрик
// в формате Prometheus.
//
// Сервисы (SSP, DSP) несут этот пакет отдельно от своей бизнес-логики:
// метрики — не часть домена, а инфраструктурная деталь.
package metrics

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// shutdownTimeout — сколько ждать завершения in-flight запросов
// к /metrics при остановке. Скрайп обычно занимает миллисекунды.
const shutdownTimeout = 3 * time.Second

// Serve запускает HTTP-сервер, отдающий /metrics.
//
// Блокирует до отмены ctx или фатальной ошибки сервера.
// net.Listen вызывается синхронно, чтобы ошибка bind (порт занят)
// всплыла сразу, а не в горутине.
func Serve(ctx context.Context, addr string) error {
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("listen metrics %s: %w", addr, err)
	}

	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())

	srv := &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		slog.Info("metrics server listening", "addr", addr)
		if err := srv.Serve(lis); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	case err := <-errCh:
		return err
	}
}
