package handler

import (
	"context"
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/7cout/minissp/internal/auction/domain"
	pb "github.com/7cout/minissp/proto/gen/auction/v1"
)

// AuctionService - сервис аукциона
type AuctionService interface {
	RunAuction(ctx context.Context, req domain.BidRequest) (*domain.AuctionResult, error)
}

// AuctionServer — реализация gRPC-сервиса AuctionService
type AuctionServer struct {
	pb.UnimplementedAuctionServiceServer
	service AuctionService
}

// NewAuctionServer создаёт gRPC-сервер аукциона.
func NewAuctionServer(svc AuctionService) *AuctionServer {
	return &AuctionServer{service: svc}
}

// RunAuction обрабатывает gRPC-запрос на проведение аукциона
func (h *AuctionServer) RunAuction(
	ctx context.Context,
	req *pb.BidRequest,
) (*pb.BidResponse, error) {
	attrs := []any{"request_id", req.GetId()}

	// Транспортная валидация.
	if req.GetId() == "" {
		return nil, status.Error(codes.InvalidArgument, "id is required")
	}
	if req.GetImp() == nil {
		return nil, status.Error(codes.InvalidArgument, "imp is required")
	}

	// Конвертация в domain.
	domainReq, err := toDomainRequest(req)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid request: %v", err)
	}

	attrs = append(attrs, "slot_id", domainReq.Imp.SlotID)

	// Вызов сервиса.
	result, err := h.service.RunAuction(ctx, domainReq)
	if err != nil {
		return nil, toGRPCError(ctx, err, attrs...)
	}

	// Конвертация ответа.
	return toProtoResponse(result), nil
}

// toDomainRequest конвертирует protobuf-запрос в domain-структуру
func toDomainRequest(req *pb.BidRequest) (domain.BidRequest, error) {
	if req.GetImp() == nil {
		return domain.BidRequest{}, errors.New("imp is required")
	}

	imp := req.GetImp()
	var banner domain.Banner
	if imp.GetBanner() != nil {
		banner = domain.Banner{
			Width:  int(imp.GetBanner().GetWidth()),
			Height: int(imp.GetBanner().GetHeight()),
		}
	}

	domainReq := domain.BidRequest{
		ID:     req.GetId(),
		UserID: req.GetUserId(),
		Imp: domain.Imp{
			ID:       imp.GetId(),
			SlotID:   imp.GetSlotId(),
			Banner:   banner,
			BidFloor: imp.GetBidFloor(),
		},
	}

	// Валидация domain-инвариантов
	if err := domainReq.Validate(); err != nil {
		return domain.BidRequest{}, err
	}

	return domainReq, nil
}

// toProtoResponse конвертирует результат аукциона в protobuf-ответ
func toProtoResponse(result *domain.AuctionResult) *pb.BidResponse {
	return &pb.BidResponse{
		AuctionId:  result.AuctionID,
		ImpId:      result.ImpID,
		BidId:      result.BidID,
		CampaignId: result.CampaignID,
		CreativeId: result.CreativeID,
		Price:      result.Price,
	}
}
