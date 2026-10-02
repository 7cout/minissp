package handler

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/7cout/minissp/internal/dsp/domain"
	pb "github.com/7cout/minissp/proto/gen/dsp/v1"
)

// DspService — то, что handler'у нужно от сервиса DSP.
type DspService interface {
	GetBid(ctx context.Context, req domain.BidRequest) (*domain.Bid, error)
	Commit(ctx context.Context, campaignID string, price int64) error
	Rollback(ctx context.Context, campaignID string, price int64) error
}

// DspServer — реализация gRPC-сервиса DspService.
type DspServer struct {
	pb.UnimplementedDspServiceServer
	service DspService
}

// NewDspServer создаёт gRPC-сервер DSP.
func NewDspServer(svc DspService) *DspServer {
	return &DspServer{service: svc}
}

// GetBid обрабатывает запрос ставки от SSP.
func (h *DspServer) GetBid(
	ctx context.Context,
	req *pb.BidRequest,
) (*pb.BidResponse, error) {
	attrs := []any{"request_id", req.GetRequestId(), "slot_id", req.GetSlotId()}

	// 1. Транспортная валидация.
	if req.GetRequestId() == "" {
		return nil, status.Error(codes.InvalidArgument, "request_id is required")
	}

	// 2. Конвертация proto → domain.
	domainReq, err := toDomainBidRequest(req)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid request: %v", err)
	}

	// 3. Вызов сервиса.
	bid, err := h.service.GetBid(ctx, domainReq)
	if err != nil {
		return nil, toGRPCError(ctx, err, attrs...)
	}

	// 4. Конвертация domain → proto.
	return toProtoBidResponse(bid), nil
}

// Commit подтверждает списание после показа.
func (h *DspServer) Commit(
	ctx context.Context,
	req *pb.CommitRequest,
) (*pb.CommitResponse, error) {
	attrs := []any{"auction_id", req.GetAuctionId(), "campaign_id", req.GetCampaignId()}

	if req.GetCampaignId() == "" {
		return nil, status.Error(codes.InvalidArgument, "campaign_id is required")
	}
	if req.GetPrice() <= 0 {
		return nil, status.Error(codes.InvalidArgument, "price must be positive")
	}

	if err := h.service.Commit(ctx, req.GetCampaignId(), req.GetPrice()); err != nil {
		return nil, toGRPCError(ctx, err, attrs...)
	}

	return &pb.CommitResponse{Ok: true}, nil
}

// Rollback отменяет резерв.
func (h *DspServer) Rollback(
	ctx context.Context,
	req *pb.RollbackRequest,
) (*pb.RollbackResponse, error) {
	attrs := []any{"auction_id", req.GetAuctionId(), "campaign_id", req.GetCampaignId()}

	if req.GetCampaignId() == "" {
		return nil, status.Error(codes.InvalidArgument, "campaign_id is required")
	}
	if req.GetPrice() <= 0 {
		return nil, status.Error(codes.InvalidArgument, "price must be positive")
	}

	if err := h.service.Rollback(ctx, req.GetCampaignId(), req.GetPrice()); err != nil {
		return nil, toGRPCError(ctx, err, attrs...)
	}

	return &pb.RollbackResponse{Ok: true}, nil
}

// toDomainBidRequest конвертирует protobuf-запрос в domain-структуру.
func toDomainBidRequest(req *pb.BidRequest) (domain.BidRequest, error) {
	domainReq := domain.BidRequest{
		RequestID: req.GetRequestId(),
		ImpID:     req.GetImpId(),
		SlotID:    req.GetSlotId(),
		Width:     req.GetWidth(),
		Height:    req.GetHeight(),
		Geo:       req.GetGeo(),
		BidFloor:  req.GetBidFloor(),
		UserID:    req.GetUserId(),
	}
	if err := domainReq.Validate(); err != nil {
		return domain.BidRequest{}, err
	}
	return domainReq, nil
}

// toProtoBidResponse конвертирует domain-ставку в protobuf-ответ.
func toProtoBidResponse(bid *domain.Bid) *pb.BidResponse {
	return &pb.BidResponse{
		BidId:       bid.ID,
		CampaignId:  bid.CampaignID,
		CreativeId:  bid.CreativeID,
		CreativeUrl: bid.CreativeURL,
		ClickUrl:    bid.ClickURL,
		Price:       bid.Price,
	}
}
