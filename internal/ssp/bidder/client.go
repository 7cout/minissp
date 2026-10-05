package bidder

import (
	"context"
	"errors"
	"fmt"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/7cout/minissp/internal/ssp/domain"
	pb "github.com/7cout/minissp/proto/gen/dsp/v1"
)

// apiKeyMetadata — имя поля в gRPC metadata для api-key.
const apiKeyMetadata = "api-key"

// Client — gRPC-клиент к DSP.
type Client struct {
	name   string
	apiKey string
	conn   *grpc.ClientConn
	client pb.DspServiceClient
}

// NewClient создаёт gRPC-клиент к DSP по адресу.
//
// name — человеческое имя DSP, используется в логах и метриках.
// apiKey — секрет, который DSP ожидает в metadata.
func NewClient(name, addr, apiKey string) (*Client, error) {
	conn, err := grpc.NewClient(
		addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, fmt.Errorf("dial dsp %s at %s: %w", name, addr, err)
	}

	return &Client{
		name:   name,
		apiKey: apiKey,
		conn:   conn,
		client: pb.NewDspServiceClient(conn),
	}, nil
}

// Name возвращает имя DSP (для логов).
func (c *Client) Name() string {
	return c.name
}

// Close закрывает соединение.
func (c *Client) Close() error {
	return c.conn.Close()
}

// GetBid запрашивает ставку у DSP.
//
// Возвращает domain.ErrNoBids, если DSP отказался участвовать.
// Возвращает прочие ошибки при проблемах с соединением.
func (c *Client) GetBid(ctx context.Context, req domain.BidRequest, slot *domain.Slot) (*domain.Bid, error) {
	ctx = c.withAuth(ctx)

	protoReq, err := toProtoBidRequest(req, slot)
	if err != nil {
		return nil, fmt.Errorf("build proto bid request: %w", err)
	}

	resp, err := c.client.GetBid(ctx, protoReq)
	if err != nil {
		return nil, mapDSPError(err)
	}

	bid, err := toDomainBid(resp)
	if err != nil {
		return nil, fmt.Errorf("parse dsp response: %w", err)
	}
	return bid, nil
}

// Commit подтверждает списание после показа.
func (c *Client) Commit(ctx context.Context, campaignID string, price int64) error {
	ctx = c.withAuth(ctx)

	_, err := c.client.Commit(ctx, &pb.CommitRequest{
		CampaignId: campaignID,
		Price:      price,
	})
	if err != nil {
		return mapDSPError(err)
	}
	return nil
}

// Rollback отменяет резерв.
func (c *Client) Rollback(ctx context.Context, campaignID string, price int64) error {
	ctx = c.withAuth(ctx)

	_, err := c.client.Rollback(ctx, &pb.RollbackRequest{
		CampaignId: campaignID,
		Price:      price,
	})
	if err != nil {
		return mapDSPError(err)
	}
	return nil
}

// withAuth добавляет api-key в metadata.
func (c *Client) withAuth(ctx context.Context) context.Context {
	return metadata.AppendToOutgoingContext(ctx, apiKeyMetadata, c.apiKey)
}

// toProtoBidRequest конвертирует domain-запрос в protobuf.
func toProtoBidRequest(req domain.BidRequest, slot *domain.Slot) (*pb.BidRequest, error) {
	protoReq := &pb.BidRequest{
		RequestId: req.RequestID,
		SlotId:    req.SlotID,
		Geo:       slot.Geo,
		BidFloor:  slot.MinPrice,
		UserId:    req.UserID,
		Type:      toProtoCreativeType(slot.Type),
	}

	switch slot.Type {
	case domain.CreativeTypeBanner:
		if slot.Banner == nil {
			return nil, errors.New("banner slot without banner params")
		}
		protoReq.Banner = &pb.Banner{
			Width:  int32(slot.Banner.Width),
			Height: int32(slot.Banner.Height),
		}
	case domain.CreativeTypeVideo:
		if slot.Video == nil {
			return nil, errors.New("video slot without video params")
		}
		protoReq.Video = &pb.Video{
			Width:    int32(slot.Video.Width),
			Height:   int32(slot.Video.Height),
			Duration: int32(slot.Video.Duration),
			Mimes:    slot.Video.MIMEs,
		}
	default:
		return nil, fmt.Errorf("unsupported slot type: %q", slot.Type)
	}

	return protoReq, nil
}

// toProtoCreativeType конвертирует domain-тип в protobuf.
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

// toDomainBid конвертирует protobuf-ответ в domain-структуру.
func toDomainBid(resp *pb.BidResponse) (*domain.Bid, error) {
	bid := &domain.Bid{
		ID:          resp.GetBidId(),
		CampaignID:  resp.GetCampaignId(),
		CreativeID:  resp.GetCreativeId(),
		CreativeURL: resp.GetCreativeUrl(),
		ClickURL:    resp.GetClickUrl(),
		Price:       resp.GetPrice(),
	}
	if err := bid.Validate(); err != nil {
		return nil, err
	}
	return bid, nil
}

// mapDSPError превращает gRPC-ошибки DSP в domain-ошибки SSP.
func mapDSPError(err error) error {
	switch status.Code(err) {
	case codes.NotFound:
		return domain.ErrNoBids
	case codes.Canceled:
		return context.Canceled
	case codes.DeadlineExceeded:
		return context.DeadlineExceeded
	default:
		return err
	}
}
