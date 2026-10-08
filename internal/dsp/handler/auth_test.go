package handler

import (
	"context"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
)

// fakeHandler — заглушка, которая просто возвращает маркер успеха.
func fakeHandler(_ context.Context, _ any) (any, error) {
	return "ok", nil
}

func TestAuthInterceptor(t *testing.T) {
	t.Run("valid key — passes", func(t *testing.T) {
		validator := NewStaticAPIKeyValidator([]string{"secret_key"})
		interceptor := AuthInterceptor(validator)

		ctx := metadata.NewIncomingContext(
			context.Background(),
			metadata.Pairs("api-key", "secret_key"),
		)

		resp, err := interceptor(ctx, nil, &grpc.UnaryServerInfo{FullMethod: "/test"}, fakeHandler)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp != "ok" {
			t.Errorf("resp = %v, want ok", resp)
		}
	})

	t.Run("invalid key — Unauthenticated", func(t *testing.T) {
		validator := NewStaticAPIKeyValidator([]string{"secret_key"})
		interceptor := AuthInterceptor(validator)

		ctx := metadata.NewIncomingContext(
			context.Background(),
			metadata.Pairs("api-key", "wrong_key"),
		)

		_, err := interceptor(ctx, nil, &grpc.UnaryServerInfo{FullMethod: "/test"}, fakeHandler)
		assertGRPCCode(t, err, codes.Unauthenticated)
	})

	t.Run("missing api-key — Unauthenticated", func(t *testing.T) {
		validator := NewStaticAPIKeyValidator([]string{"secret_key"})
		interceptor := AuthInterceptor(validator)

		ctx := metadata.NewIncomingContext(
			context.Background(),
			metadata.Pairs("other-header", "value"),
		)

		_, err := interceptor(ctx, nil, &grpc.UnaryServerInfo{FullMethod: "/test"}, fakeHandler)
		assertGRPCCode(t, err, codes.Unauthenticated)
	})

	t.Run("no metadata — Unauthenticated", func(t *testing.T) {
		validator := NewStaticAPIKeyValidator([]string{"secret_key"})
		interceptor := AuthInterceptor(validator)

		_, err := interceptor(context.Background(), nil,
			&grpc.UnaryServerInfo{FullMethod: "/test"}, fakeHandler)
		assertGRPCCode(t, err, codes.Unauthenticated)
	})

	t.Run("empty validator — passes without key", func(t *testing.T) {
		validator := NewStaticAPIKeyValidator(nil) // auth отключён
		interceptor := AuthInterceptor(validator)

		ctx := metadata.NewIncomingContext(
			context.Background(),
			metadata.Pairs("api-key", "any_key"),
		)

		resp, err := interceptor(ctx, nil, &grpc.UnaryServerInfo{FullMethod: "/test"}, fakeHandler)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp != "ok" {
			t.Errorf("resp = %v, want ok", resp)
		}
	})

	t.Run("multiple keys — all valid", func(t *testing.T) {
		validator := NewStaticAPIKeyValidator([]string{"key_a", "key_b"})
		interceptor := AuthInterceptor(validator)

		for _, key := range []string{"key_a", "key_b"} {
			ctx := metadata.NewIncomingContext(
				context.Background(),
				metadata.Pairs("api-key", key),
			)
			_, err := interceptor(ctx, nil, &grpc.UnaryServerInfo{FullMethod: "/test"}, fakeHandler)
			if err != nil {
				t.Errorf("key %q rejected: %v", key, err)
			}
		}
	})
}
