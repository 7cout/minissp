package handler

import (
	"context"
	"errors"
	"log/slog"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/7cout/minissp/internal/ssp/domain"
)

// toGRPCError превращает доменную ошибку в gRPC-статус.
// Логирует только неожиданные ошибки.
func toGRPCError(ctx context.Context, err error, attrs ...any) error {
	switch {
	// --- Domain: ожидаемые бизнес-ошибки ---
	case errors.Is(err, domain.ErrSlotNotFound):
		return status.Error(codes.NotFound, "slot not found")

	case errors.Is(err, domain.ErrSlotAlreadyExists):
		return status.Error(codes.AlreadyExists, "slot already exists")

	case errors.Is(err, domain.ErrPublisherNotFound):
		return status.Error(codes.NotFound, "publisher not found")

	case errors.Is(err, domain.ErrNoBids):
		return status.Error(codes.NotFound, "no bids received")

	case errors.Is(err, domain.ErrAuctionNotFound):
		return status.Error(codes.NotFound, "auction not found")

	case errors.Is(err, domain.ErrInvalidID):
		return status.Error(codes.InvalidArgument, "invalid id format")

	case errors.Is(err, domain.ErrUnauthenticated):
		return status.Error(codes.Unauthenticated, "unauthenticated")

	// --- Контекст ---
	case errors.Is(err, context.Canceled):
		return status.Error(codes.Canceled, "request canceled")

	case errors.Is(err, context.DeadlineExceeded):
		return status.Error(codes.DeadlineExceeded, "request timeout")

	// --- Неожиданное ---
	default:
		slog.ErrorContext(ctx, "internal error",
			append(attrs, "error", err)...)
		return status.Error(codes.Internal, "internal error")
	}
}
