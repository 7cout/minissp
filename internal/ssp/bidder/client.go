package bidder

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/7cout/minissp/internal/ssp/domain"
	pb "github.com/7cout/minissp/proto/gen/dsp/v1"
)

const (
	// apiKeyMetadata — имя поля в gRPC metadata для api-key.
	apiKeyMetadata = "api-key"

	// rpcTimeout — таймаут на один RPC к DSP.
	//
	// В реальном RTB ответ нужен за ~100 мс, но у нас pet-проект
	// и DSP живёт рядом. 2 секунды с большим запасом.
	// Главное — не давать запросу висеть на клиентском ctx:
	// тогда при недоступном DSP мы быстро узнаем об ошибке.
	rpcTimeout = 2 * time.Second
)

// Client — gRPC-клиент к DSP.
type Client struct {
	name   string
	apiKey string
	conn   *grpc.ClientConn
	client pb.DspServiceClient
}

// NewClientWithConn создаёт клиент на готовом gRPC-соединении.
//
// Полезно для тестов (bufconn) и для случаев, когда соединение
// управляется извне — DI-контейнером, пулом. Для обычного сценария
// используй NewClient с адресом.
func NewClientWithConn(name, apiKey string, conn *grpc.ClientConn) *Client {
	return &Client{
		name:   name,
		apiKey: apiKey,
		conn:   conn,
		client: pb.NewDspServiceClient(conn),
	}
}

// NewClient создаёт gRPC-клиент к DSP по адресу.
func NewClient(name, addr, apiKey string) (*Client, error) {
	conn, err := grpc.NewClient(
		addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, fmt.Errorf("dial dsp %s at %s: %w", name, addr, err)
	}
	return NewClientWithConn(name, apiKey, conn), nil
}

// Name возвращает имя DSP (для логов).
func (c *Client) Name() string {
	return c.name
}

// Close закрывает соединение.
func (c *Client) Close() error {
	return c.conn.Close()
}

// WarmUp форсирует установку gRPC-соединения.
//
// grpc.NewClient ленивый: соединение устанавливается при первом RPC.
// Если это происходит во время RunAuction, первый запрос ждёт
// TCP+HTTP/2 handshake и может упасть по клиентскому таймауту
// (у Publisher он 10 секунд).
//
// WarmUp вызывается один раз при старте SSP: ждём Ready с коротким
// таймаутом, логируем результат. Дальше первые RPC идут мгновенно.
func (c *Client) WarmUp(ctx context.Context) error {
	c.conn.Connect()

	for {
		state := c.conn.GetState()
		slog.Info("dsp connection state", "dsp", c.name, "state", state.String())
		if state == connectivity.Ready {
			return nil
		}
		if !c.conn.WaitForStateChange(ctx, state) {
			return ctx.Err()
		}
	}
}

// GetBid запрашивает ставку у DSP.
//
// Возвращает domain.ErrNoBids, если DSP отказался участвовать.
// Возвращает прочие ошибки при проблемах с соединением.
func (c *Client) GetBid(ctx context.Context, req domain.BidRequest, slot *domain.Slot) (*domain.Bid, error) {
	ctx, cancel := context.WithTimeout(ctx, rpcTimeout)
	defer cancel()

	ctx = c.withAuth(ctx)

	protoReq, err := toProtoBidRequest(req, slot)
	if err != nil {
		return nil, fmt.Errorf("build proto bid request: %w", err)
	}

	resp, err := c.client.GetBid(ctx, protoReq)
	if err != nil {
		return nil, mapGetBidError(err)
	}

	bid, err := toDomainBid(resp)
	if err != nil {
		return nil, fmt.Errorf("parse dsp response: %w", err)
	}
	return bid, nil
}

// Commit подтверждает списание после показа.
func (c *Client) Commit(ctx context.Context, campaignID string, price int64) error {
	ctx, cancel := context.WithTimeout(ctx, rpcTimeout)
	defer cancel()

	ctx = c.withAuth(ctx)

	_, err := c.client.Commit(ctx, &pb.CommitRequest{
		CampaignId: campaignID,
		Price:      price,
	})
	if err != nil {
		return mapMutateError(err)
	}
	return nil
}

// Rollback отменяет резерв.
func (c *Client) Rollback(ctx context.Context, campaignID string, price int64) error {
	ctx, cancel := context.WithTimeout(ctx, rpcTimeout)
	defer cancel()

	ctx = c.withAuth(ctx)

	_, err := c.client.Rollback(ctx, &pb.RollbackRequest{
		CampaignId: campaignID,
		Price:      price,
	})
	if err != nil {
		return mapMutateError(err)
	}
	return nil
}

// withAuth добавляет api-key в metadata.
func (c *Client) withAuth(ctx context.Context) context.Context {
	return metadata.AppendToOutgoingContext(ctx, apiKeyMetadata, c.apiKey)
}

// toProtoBidRequest конвертирует domain-запрос в protobuf.
func toProtoBidRequest(req domain.BidRequest, slot *domain.Slot) (*pb.BidRequest, error) {
	if req.ImpID == "" {
		return nil, errors.New("imp id is required")
	}

	protoReq := &pb.BidRequest{
		RequestId: req.RequestID,
		ImpId:     req.ImpID,
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

// mapGetBidError превращает gRPC-ошибки DSP в domain-ошибки SSP
// для вызова GetBid.
//
// Здесь NotFound означает "нет подходящей кампании" — это не ошибка
// с точки зрения аукциона, а нормальная ситуация. Превращаем в
// ErrNoBids, который collectBids пропускает.
func mapGetBidError(err error) error {
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

// mapMutateError — для Commit/Rollback.
//
// Здесь NotFound = "кампания не найдена на DSP", это реальная ошибка,
// а не "просто нет ставки". Возвращаем ErrCampaignNotFound, чтобы
// вызывающий понимал семантику.
func mapMutateError(err error) error {
	switch status.Code(err) {
	case codes.NotFound:
		return domain.ErrCampaignNotFound
	case codes.Canceled:
		return context.Canceled
	case codes.DeadlineExceeded:
		return context.DeadlineExceeded
	default:
		return err
	}
}
