package handler

import (
	"context"
	"errors"
	"log/slog"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/7cout/minissp/internal/dsp/domain"
)

// toGRPCError превращает доменную ошибку в gRPC-статус.
// Логирует только неожиданные ошибки.
func toGRPCError(ctx context.Context, err error, attrs ...any) error {
	switch {
	// --- Ожидаемые бизнес-ошибки ---
	case errors.Is(err, domain.ErrNoEligibleCampaign):
		return status.Error(codes.NotFound, "no eligible campaign")

	case errors.Is(err, domain.ErrCampaignNotFound):
		return status.Error(codes.NotFound, "campaign not found")

	case errors.Is(err, domain.ErrCreativeNotFound):
		return status.Error(codes.NotFound, "creative not found")

	case errors.Is(err, domain.ErrAdvertiserNotFound):
		return status.Error(codes.NotFound, "advertiser not found")

	case errors.Is(err, domain.ErrInsufficientBudget):
		return status.Error(codes.FailedPrecondition, "insufficient budget")

	case errors.Is(err, domain.ErrInsufficientBalance):
		return status.Error(codes.FailedPrecondition, "insufficient balance")

	case errors.Is(err, domain.ErrInvalidID):
		return status.Error(codes.InvalidArgument, "invalid id format")

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
