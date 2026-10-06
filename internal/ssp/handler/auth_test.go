package handler

import (
	"context"
	"errors"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/7cout/minissp/internal/ssp/domain"
)

// fakeResolver — простой резолвер api-key → publisher_id.
type fakeResolver struct {
	keys map[string]string // api-key → publisher_id
	err  error
}

func (f *fakeResolver) Resolve(_ context.Context, apiKey string) (string, error) {
	if f.err != nil {
		return "", f.err
	}
	id, ok := f.keys[apiKey]
	if !ok {
		return "", domain.ErrPublisherNotFound
	}
	return id, nil
}

func fakeHandler(_ context.Context, _ any) (any, error) {
	return "ok", nil
}

func TestAuthInterceptor(t *testing.T) {
	t.Run("valid key — passes and sets publisher_id", func(t *testing.T) {
		resolver := &fakeResolver{keys: map[string]string{"key_1": "pub_1"}}
		interceptor := AuthInterceptor(resolver)

		var capturedPublisherID string
		next := func(ctx context.Context, _ any) (any, error) {
			id, _ := ctx.Value(PublisherContextKey{}).(string)
			capturedPublisherID = id
			return "ok", nil
		}

		ctx := metadata.NewIncomingContext(
			context.Background(),
			metadata.Pairs("api-key", "key_1"),
		)

		_, err := interceptor(ctx, nil, &grpc.UnaryServerInfo{FullMethod: "/test"}, next)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if capturedPublisherID != "pub_1" {
			t.Errorf("publisher_id = %q, want pub_1", capturedPublisherID)
		}
	})

	t.Run("invalid key — Unauthenticated", func(t *testing.T) {
		resolver := &fakeResolver{keys: map[string]string{"key_1": "pub_1"}}
		interceptor := AuthInterceptor(resolver)

		ctx := metadata.NewIncomingContext(
			context.Background(),
			metadata.Pairs("api-key", "wrong"),
		)

		_, err := interceptor(ctx, nil, &grpc.UnaryServerInfo{FullMethod: "/test"}, fakeHandler)
		assertGRPCCode(t, err, codes.Unauthenticated)
	})

	t.Run("missing api-key — Unauthenticated", func(t *testing.T) {
		resolver := &fakeResolver{keys: map[string]string{"key_1": "pub_1"}}
		interceptor := AuthInterceptor(resolver)

		ctx := metadata.NewIncomingContext(
			context.Background(),
			metadata.Pairs("other-header", "value"),
		)

		_, err := interceptor(ctx, nil, &grpc.UnaryServerInfo{FullMethod: "/test"}, fakeHandler)
		assertGRPCCode(t, err, codes.Unauthenticated)
	})

	t.Run("empty api-key value — Unauthenticated", func(t *testing.T) {
		resolver := &fakeResolver{keys: map[string]string{"key_1": "pub_1"}}
		interceptor := AuthInterceptor(resolver)

		ctx := metadata.NewIncomingContext(
			context.Background(),
			metadata.Pairs("api-key", ""),
		)

		_, err := interceptor(ctx, nil, &grpc.UnaryServerInfo{FullMethod: "/test"}, fakeHandler)
		assertGRPCCode(t, err, codes.Unauthenticated)
	})

	t.Run("no metadata — Unauthenticated", func(t *testing.T) {
		resolver := &fakeResolver{keys: map[string]string{"key_1": "pub_1"}}
		interceptor := AuthInterceptor(resolver)

		_, err := interceptor(context.Background(), nil,
			&grpc.UnaryServerInfo{FullMethod: "/test"}, fakeHandler)
		assertGRPCCode(t, err, codes.Unauthenticated)
	})

	t.Run("resolver error — Unauthenticated", func(t *testing.T) {
		resolver := &fakeResolver{err: errors.New("db error")}
		interceptor := AuthInterceptor(resolver)

		ctx := metadata.NewIncomingContext(
			context.Background(),
			metadata.Pairs("api-key", "any"),
		)

		_, err := interceptor(ctx, nil, &grpc.UnaryServerInfo{FullMethod: "/test"}, fakeHandler)
		assertGRPCCode(t, err, codes.Unauthenticated)
	})
}

func TestPublisherIDFromContext(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), PublisherContextKey{}, "pub_1")
		id, err := publisherIDFromContext(ctx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if id != "pub_1" {
			t.Errorf("id = %q, want pub_1", id)
		}
	})

	t.Run("missing", func(t *testing.T) {
		_, err := publisherIDFromContext(context.Background())
		if status.Code(err) != codes.Unauthenticated {
			t.Errorf("want Unauthenticated, got %v", err)
		}
	})

	t.Run("empty value", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), PublisherContextKey{}, "")
		_, err := publisherIDFromContext(ctx)
		if status.Code(err) != codes.Unauthenticated {
			t.Errorf("want Unauthenticated, got %v", err)
		}
	})

	t.Run("wrong type in context", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), PublisherContextKey{}, 123)
		_, err := publisherIDFromContext(ctx)
		if status.Code(err) != codes.Unauthenticated {
			t.Errorf("want Unauthenticated, got %v", err)
		}
	})
}
