package handler

import (
	"context"
	"log/slog"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// apiKeyMetadata — имя поля в gRPC metadata для api-key.
const apiKeyMetadata = "api-key"

// PublisherContextKey — ключ для publisher_id в context.
type PublisherContextKey struct{}

// PublisherResolver — то, что нужно для аутентификации Publisher'а.
//
// Resolve возвращает publisher_id по api-key.
// Возвращает domain.ErrPublisherNotFound, если ключ невалиден.
type PublisherResolver interface {
	Resolve(ctx context.Context, apiKey string) (string, error)
}

// AuthInterceptor проверяет api-key в metadata и кладёт publisher_id
// в context.
//
// api-key обязателен для всех методов: если ключ отсутствует или
// невалиден — codes.Unauthenticated.
func AuthInterceptor(resolver PublisherResolver) grpc.UnaryServerInterceptor {
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
		if len(keys) == 0 || keys[0] == "" {
			return nil, status.Errorf(codes.Unauthenticated, "missing %s", apiKeyMetadata)
		}

		publisherID, err := resolver.Resolve(ctx, keys[0])
		if err != nil {
			slog.WarnContext(ctx, "invalid api-key",
				"method", info.FullMethod,
			)
			return nil, status.Error(codes.Unauthenticated, "invalid api-key")
		}

		ctx = context.WithValue(ctx, PublisherContextKey{}, publisherID)
		return handler(ctx, req)
	}
}

// publisherIDFromContext возвращает publisher_id, положенный интерсептором.
func publisherIDFromContext(ctx context.Context) (string, error) {
	id, ok := ctx.Value(PublisherContextKey{}).(string)
	if !ok || id == "" {
		return "", status.Error(codes.Unauthenticated, "publisher not authenticated")
	}
	return id, nil
}
