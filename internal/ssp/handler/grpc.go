package handler

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/7cout/minissp/internal/ssp/domain"
	pb "github.com/7cout/minissp/proto/gen/ssp/v1"
)

// SspService — то, что handler'у нужно от сервиса SSP.
type SspService interface {
	RegisterSlot(ctx context.Context, publisherID string, req domain.Slot) (*domain.Slot, error)
	GetSlotByName(ctx context.Context, publisherID, name string) (*domain.Slot, error)
	RunAuction(ctx context.Context, req domain.BidRequest) (*domain.AuctionResult, error)
	Impression(ctx context.Context, auctionID string) error
	GetPublisherBalance(ctx context.Context, publisherID string) (int64, error)
}

// SspServer — реализация gRPC-сервиса SspService.
type SspServer struct {
	pb.UnimplementedSspServiceServer
	service SspService
}

// NewSspServer создаёт gRPC-сервер SSP.
func NewSspServer(svc SspService) *SspServer {
	return &SspServer{service: svc}
}

// RegisterSlot регистрирует слот.
func (h *SspServer) RegisterSlot(
	ctx context.Context,
	req *pb.RegisterSlotRequest,
) (*pb.Slot, error) {
	publisherID, err := publisherIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	attrs := []any{"publisher_id", publisherID, "name", req.GetName()}

	domainReq, err := toDomainRegisterSlot(req)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid request: %v", err)
	}

	slot, err := h.service.RegisterSlot(ctx, publisherID, domainReq)
	if err != nil {
		return nil, toGRPCError(ctx, err, attrs...)
	}

	return toProtoSlot(slot), nil
}

// GetSlotByName возвращает слот по имени.
func (h *SspServer) GetSlotByName(
	ctx context.Context,
	req *pb.GetSlotByNameRequest,
) (*pb.Slot, error) {
	publisherID, err := publisherIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	attrs := []any{"publisher_id", publisherID, "name", req.GetName()}

	slot, err := h.service.GetSlotByName(ctx, publisherID, req.GetName())
	if err != nil {
		return nil, toGRPCError(ctx, err, attrs...)
	}

	return toProtoSlot(slot), nil
}

// RunAuction проводит аукцион.
func (h *SspServer) RunAuction(
	ctx context.Context,
	req *pb.BidRequest,
) (*pb.BidResponse, error) {
	publisherID, err := publisherIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	attrs := []any{
		"publisher_id", publisherID,
		"request_id", req.GetRequestId(),
		"slot_id", req.GetSlotId(),
	}

	if req.GetRequestId() == "" {
		return nil, status.Error(codes.InvalidArgument, "request_id is required")
	}
	if req.GetSlotId() == "" {
		return nil, status.Error(codes.InvalidArgument, "slot_id is required")
	}

	domainReq := domain.BidRequest{
		RequestID: req.GetRequestId(),
		SlotID:    req.GetSlotId(),
		UserID:    req.GetUserId(),
	}

	result, err := h.service.RunAuction(ctx, domainReq)
	if err != nil {
		return nil, toGRPCError(ctx, err, attrs...)
	}

	return &pb.BidResponse{
		AuctionId:   result.AuctionID,
		CreativeUrl: result.CreativeURL,
		ClickUrl:    result.ClickURL,
	}, nil
}

// Impression обрабатывает подтверждение показа.
func (h *SspServer) Impression(
	ctx context.Context,
	req *pb.ImpressionRequest,
) (*pb.ImpressionResponse, error) {
	publisherID, err := publisherIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	attrs := []any{"publisher_id", publisherID, "auction_id", req.GetAuctionId()}

	if req.GetAuctionId() == "" {
		return nil, status.Error(codes.InvalidArgument, "auction_id is required")
	}

	if err := h.service.Impression(ctx, req.GetAuctionId()); err != nil {
		return nil, toGRPCError(ctx, err, attrs...)
	}

	return &pb.ImpressionResponse{Ok: true}, nil
}

// GetPublisherBalance возвращает баланс издателя.
func (h *SspServer) GetPublisherBalance(
	ctx context.Context,
	_ *pb.GetPublisherBalanceRequest,
) (*pb.GetPublisherBalanceResponse, error) {
	publisherID, err := publisherIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	balance, err := h.service.GetPublisherBalance(ctx, publisherID)
	if err != nil {
		return nil, toGRPCError(ctx, err, "publisher_id", publisherID)
	}

	return &pb.GetPublisherBalanceResponse{Balance: balance}, nil
}

// --- Конвертеры ---

// toDomainRegisterSlot конвертирует protobuf-запрос в domain-структуру
// и делает транспортную валидацию — только тех полей, которые
// приходят в запросе. Полная Validate() (с ID и publisher_id) —
// вызывается в сервисе.
func toDomainRegisterSlot(req *pb.RegisterSlotRequest) (domain.Slot, error) {
	slot := domain.Slot{
		Name:     req.GetName(),
		Geo:      req.GetGeo(),
		MinPrice: req.GetMinPrice(),
		Type:     toDomainCreativeType(req.GetType()),
		Banner:   toDomainBanner(req.GetBanner()),
		Video:    toDomainVideo(req.GetVideo()),
		Native:   toDomainNative(req.GetNative()),
		Audio:    toDomainAudio(req.GetAudio()),
	}

	if strings.TrimSpace(slot.Name) == "" {
		return domain.Slot{}, errors.New("name is required")
	}
	if slot.Type == "" {
		return domain.Slot{}, errors.New("unsupported creative type")
	}
	if err := validateParamsForType(slot); err != nil {
		return domain.Slot{}, err
	}

	return slot, nil
}

// validateParamsForType проверяет, что для указанного типа
// заполнены соответствующие параметры.
func validateParamsForType(slot domain.Slot) error {
	switch slot.Type {
	case domain.CreativeTypeBanner:
		if slot.Banner == nil {
			return errors.New("banner params are required")
		}
		return slot.Banner.Validate()
	case domain.CreativeTypeVideo:
		if slot.Video == nil {
			return errors.New("video params are required")
		}
		return slot.Video.Validate()
	case domain.CreativeTypeNative:
		if slot.Native == nil {
			return errors.New("native params are required")
		}
		return slot.Native.Validate()
	case domain.CreativeTypeAudio:
		if slot.Audio == nil {
			return errors.New("audio params are required")
		}
		return slot.Audio.Validate()
	default:
		return fmt.Errorf("unsupported type: %q", slot.Type)
	}
}

func toDomainCreativeType(t pb.CreativeType) domain.CreativeType {
	switch t {
	case pb.CreativeType_CREATIVE_TYPE_BANNER:
		return domain.CreativeTypeBanner
	case pb.CreativeType_CREATIVE_TYPE_VIDEO:
		return domain.CreativeTypeVideo
	case pb.CreativeType_CREATIVE_TYPE_NATIVE:
		return domain.CreativeTypeNative
	case pb.CreativeType_CREATIVE_TYPE_AUDIO:
		return domain.CreativeTypeAudio
	default:
		return ""
	}
}

func toProtoCreativeType(t domain.CreativeType) pb.CreativeType {
	switch t {
	case domain.CreativeTypeBanner:
		return pb.CreativeType_CREATIVE_TYPE_BANNER
	case domain.CreativeTypeVideo:
		return pb.CreativeType_CREATIVE_TYPE_VIDEO
	case domain.CreativeTypeNative:
		return pb.CreativeType_CREATIVE_TYPE_NATIVE
	case domain.CreativeTypeAudio:
		return pb.CreativeType_CREATIVE_TYPE_AUDIO
	default:
		return pb.CreativeType_CREATIVE_TYPE_UNSPECIFIED
	}
}

func toDomainBanner(b *pb.Banner) *domain.Banner {
	if b == nil {
		return nil
	}
	return &domain.Banner{
		Width:  int(b.GetWidth()),
		Height: int(b.GetHeight()),
	}
}

func toDomainVideo(v *pb.Video) *domain.Video {
	if v == nil {
		return nil
	}
	return &domain.Video{
		Width:    int(v.GetWidth()),
		Height:   int(v.GetHeight()),
		Duration: int(v.GetDuration()),
		MIMEs:    v.GetMimes(),
	}
}

func toDomainNative(n *pb.Native) *domain.Native {
	if n == nil {
		return nil
	}
	return &domain.Native{}
}

func toDomainAudio(a *pb.Audio) *domain.Audio {
	if a == nil {
		return nil
	}
	return &domain.Audio{}
}

func toProtoSlot(s *domain.Slot) *pb.Slot {
	protoSlot := &pb.Slot{
		Id:          s.ID,
		PublisherId: s.PublisherID,
		Name:        s.Name,
		Geo:         s.Geo,
		MinPrice:    s.MinPrice,
		Type:        toProtoCreativeType(s.Type),
	}
	if s.Banner != nil {
		protoSlot.Banner = &pb.Banner{
			Width:  int32(s.Banner.Width),
			Height: int32(s.Banner.Height),
		}
	}
	if s.Video != nil {
		protoSlot.Video = &pb.Video{
			Width:    int32(s.Video.Width),
			Height:   int32(s.Video.Height),
			Duration: int32(s.Video.Duration),
			Mimes:    s.Video.MIMEs,
		}
	}
	// Native, Audio — заглушки.
	return protoSlot
}
