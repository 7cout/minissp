package handler

import (
	"context"
	"log/slog"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// apiKeyMetadata — имя поля в gRPC metadata, где передаётся ключ.
const apiKeyMetadata = "api-key"

// APIKeyValidator проверяет, что api-key принадлежит известному клиенту.
type APIKeyValidator interface {
	Validate(key string) bool
}

// staticAPIKeyValidator — валидатор с одним ключом.
// Подходит, когда у сервиса один-два клиента (например, только SSP).
type staticAPIKeyValidator struct {
	validKeys map[string]struct{}
}

// NewStaticAPIKeyValidator создаёт валидатор с набором валидных ключей.
//
// Если keys пустой — валидатор считает любой ключ валидным.
// Для локальной разработки без аутентификации.
func NewStaticAPIKeyValidator(keys []string) APIKeyValidator {
	set := make(map[string]struct{}, len(keys))
	for _, k := range keys {
		if k != "" {
			set[k] = struct{}{}
		}
	}
	return &staticAPIKeyValidator{validKeys: set}
}

func (v *staticAPIKeyValidator) Validate(key string) bool {
	// Пустой набор ключей — auth отключён.
	if len(v.validKeys) == 0 {
		return true
	}
	_, ok := v.validKeys[key]
	return ok
}

// AuthInterceptor возвращает gRPC unary interceptor, который проверяет
// api-key в metadata. Если ключ невалиден или отсутствует — возвращает
// codes.Unauthenticated.
//
// Если валидатор пуст (auth отключён) — пропускает все запросы,
// но пишет warning в лог один раз.
func AuthInterceptor(validator APIKeyValidator) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (any, error) {
		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			return nil, status.Error(codes.Unauthenticated, "missing metadata")
		}

		keys := md.Get(apiKeyMetadata)
		if len(keys) == 0 {
			return nil, status.Errorf(codes.Unauthenticated, "missing %s", apiKeyMetadata)
		}

		if !validator.Validate(keys[0]) {
			slog.WarnContext(ctx, "invalid api-key",
				"method", info.FullMethod,
			)
			return nil, status.Error(codes.Unauthenticated, "invalid api-key")
		}

		return handler(ctx, req)
	}
}
