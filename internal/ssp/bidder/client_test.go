package bidder

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"

	"github.com/7cout/minissp/internal/ssp/domain"
	pb "github.com/7cout/minissp/proto/gen/dsp/v1"
)

// --- fake DSP ---

type fakeDSP struct {
	pb.UnimplementedDspServiceServer

	getBidResp *pb.BidResponse
	getBidErr  error

	commitResp *pb.CommitResponse
	commitErr  error

	rollbackResp *pb.RollbackResponse
	rollbackErr  error

	// captured
	capturedGetBidReq   *pb.BidRequest
	capturedCommitReq   *pb.CommitRequest
	capturedRollbackReq *pb.RollbackRequest
	capturedMetadata    metadata.MD
}

func (f *fakeDSP) GetBid(ctx context.Context, req *pb.BidRequest) (*pb.BidResponse, error) {
	f.capturedGetBidReq = req
	f.capturedMetadata, _ = metadata.FromIncomingContext(ctx)
	if f.getBidErr != nil {
		return nil, f.getBidErr
	}
	return f.getBidResp, nil
}

func (f *fakeDSP) Commit(ctx context.Context, req *pb.CommitRequest) (*pb.CommitResponse, error) {
	f.capturedCommitReq = req
	f.capturedMetadata, _ = metadata.FromIncomingContext(ctx)
	if f.commitErr != nil {
		return nil, f.commitErr
	}
	return f.commitResp, nil
}

func (f *fakeDSP) Rollback(ctx context.Context, req *pb.RollbackRequest) (*pb.RollbackResponse, error) {
	f.capturedRollbackReq = req
	f.capturedMetadata, _ = metadata.FromIncomingContext(ctx)
	if f.rollbackErr != nil {
		return nil, f.rollbackErr
	}
	return f.rollbackResp, nil
}

// --- helpers ---

// startFakeDSP запускает fake DSP через bufconn и возвращает клиент.
func startFakeDSP(t *testing.T, fake pb.DspServiceServer) *Client {
	t.Helper()

	lis := bufconn.Listen(1024 * 1024)
	srv := grpc.NewServer()
	pb.RegisterDspServiceServer(srv, fake)

	go func() {
		_ = srv.Serve(lis)
	}()
	t.Cleanup(srv.Stop)

	conn, err := grpc.NewClient(
		"passthrough:///bufnet",
		grpc.WithContextDialer(func(_ context.Context, _ string) (net.Conn, error) {
			return lis.DialContext(context.Background())
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("dial bufconn: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	return newClientWithConn("test-dsp", "test-api-key", conn)
}

func validBidRequest() domain.BidRequest {
	return domain.BidRequest{
		RequestID: "req_1",
		ImpID:     "imp_1",
		SlotID:    "slot_1",
		UserID:    "user_1",
	}
}

func validBannerSlot() *domain.Slot {
	return &domain.Slot{
		ID:          "slot_1",
		PublisherID: "pub_1",
		Name:        "home_banner",
		Geo:         "RU",
		MinPrice:    1_000_000,
		Type:        domain.CreativeTypeBanner,
		Banner:      &domain.Banner{Width: 320, Height: 50},
	}
}

func validVideoSlot() *domain.Slot {
	return &domain.Slot{
		ID:          "slot_2",
		PublisherID: "pub_1",
		Name:        "preroll",
		Geo:         "RU",
		MinPrice:    5_000_000,
		Type:        domain.CreativeTypeVideo,
		Video: &domain.Video{
			Width:    640,
			Height:   480,
			Duration: 15,
			MIMEs:    []string{"video/mp4"},
		},
	}
}

func validBidResponse() *pb.BidResponse {
	return &pb.BidResponse{
		BidId:       "bid_1",
		CampaignId:  "camp_1",
		CreativeId:  "cr_1",
		CreativeUrl: "https://cdn.example.com/a.jpg",
		ClickUrl:    "https://example.com/click",
		Price:       1_500_000,
	}
}

// --- GetBid ---

func TestClient_GetBid_HappyPath(t *testing.T) {
	fake := &fakeDSP{getBidResp: validBidResponse()}
	c := startFakeDSP(t, fake)

	bid, err := c.GetBid(context.Background(), validBidRequest(), validBannerSlot())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if bid.ID != "bid_1" {
		t.Errorf("id = %q, want bid_1", bid.ID)
	}
	if bid.CampaignID != "camp_1" {
		t.Errorf("campaign = %q, want camp_1", bid.CampaignID)
	}
	if bid.Price != 1_500_000 {
		t.Errorf("price = %d, want 1500000", bid.Price)
	}
}

func TestClient_GetBid_SendsAPIKey(t *testing.T) {
	fake := &fakeDSP{getBidResp: validBidResponse()}
	c := startFakeDSP(t, fake)

	_, err := c.GetBid(context.Background(), validBidRequest(), validBannerSlot())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	keys := fake.capturedMetadata.Get("api-key")
	if len(keys) == 0 || keys[0] != "test-api-key" {
		t.Errorf("api-key metadata = %v, want [test-api-key]", keys)
	}
}

func TestClient_GetBid_ConvertsBannerSlot(t *testing.T) {
	fake := &fakeDSP{getBidResp: validBidResponse()}
	c := startFakeDSP(t, fake)

	_, err := c.GetBid(context.Background(), validBidRequest(), validBannerSlot())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	req := fake.capturedGetBidReq
	if req == nil {
		t.Fatal("request not captured")
	}
	if req.GetRequestId() != "req_1" {
		t.Errorf("request_id = %q", req.GetRequestId())
	}
	if req.GetImpId() != "imp_1" {
		t.Errorf("imp_id = %q, want imp_1", req.GetImpId())
	}
	if req.GetSlotId() != "slot_1" {
		t.Errorf("slot_id = %q", req.GetSlotId())
	}
	if req.GetGeo() != "RU" {
		t.Errorf("geo = %q", req.GetGeo())
	}
	if req.GetBidFloor() != 1_000_000 {
		t.Errorf("bid_floor = %d", req.GetBidFloor())
	}
	if req.GetUserId() != "user_1" {
		t.Errorf("user_id = %q", req.GetUserId())
	}
	if req.GetType() != pb.CreativeType_CREATIVE_TYPE_BANNER {
		t.Errorf("type = %v", req.GetType())
	}
	if req.GetBanner() == nil || req.GetBanner().GetWidth() != 320 || req.GetBanner().GetHeight() != 50 {
		t.Errorf("banner = %+v", req.GetBanner())
	}
}

func TestClient_GetBid_ConvertsVideoSlot(t *testing.T) {
	fake := &fakeDSP{getBidResp: validBidResponse()}
	c := startFakeDSP(t, fake)

	_, err := c.GetBid(context.Background(), validBidRequest(), validVideoSlot())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	req := fake.capturedGetBidReq
	if req.GetType() != pb.CreativeType_CREATIVE_TYPE_VIDEO {
		t.Errorf("type = %v", req.GetType())
	}
	if req.GetVideo() == nil {
		t.Fatal("video is nil")
	}
	if req.GetVideo().GetDuration() != 15 {
		t.Errorf("duration = %d", req.GetVideo().GetDuration())
	}
	if len(req.GetVideo().GetMimes()) != 1 || req.GetVideo().GetMimes()[0] != "video/mp4" {
		t.Errorf("mimes = %v", req.GetVideo().GetMimes())
	}
}

func TestClient_GetBid_UnsupportedSlotType(t *testing.T) {
	fake := &fakeDSP{getBidResp: validBidResponse()}
	c := startFakeDSP(t, fake)

	slot := validBannerSlot()
	slot.Type = "unknown"
	slot.Banner = nil

	_, err := c.GetBid(context.Background(), validBidRequest(), slot)
	if err == nil {
		t.Fatal("want error, got nil")
	}
}

func TestClient_GetBid_BannerSlotWithoutBanner(t *testing.T) {
	fake := &fakeDSP{getBidResp: validBidResponse()}
	c := startFakeDSP(t, fake)

	slot := validBannerSlot()
	slot.Banner = nil

	_, err := c.GetBid(context.Background(), validBidRequest(), slot)
	if err == nil {
		t.Fatal("want error, got nil")
	}
}

func TestClient_GetBid_NotFoundReturnsErrNoBids(t *testing.T) {
	fake := &fakeDSP{
		getBidErr: status.Error(codes.NotFound, "no bid"),
	}
	c := startFakeDSP(t, fake)

	_, err := c.GetBid(context.Background(), validBidRequest(), validBannerSlot())
	if !errors.Is(err, domain.ErrNoBids) {
		t.Errorf("want ErrNoBids, got %v", err)
	}
}

func TestClient_GetBid_DeadlineExceeded(t *testing.T) {
	fake := &fakeDSP{
		getBidErr: status.Error(codes.DeadlineExceeded, "timeout"),
	}
	c := startFakeDSP(t, fake)

	_, err := c.GetBid(context.Background(), validBidRequest(), validBannerSlot())
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("want context.DeadlineExceeded, got %v", err)
	}
}

func TestClient_GetBid_Canceled(t *testing.T) {
	fake := &fakeDSP{
		getBidErr: status.Error(codes.Canceled, "canceled"),
	}
	c := startFakeDSP(t, fake)

	_, err := c.GetBid(context.Background(), validBidRequest(), validBannerSlot())
	if !errors.Is(err, context.Canceled) {
		t.Errorf("want context.Canceled, got %v", err)
	}
}

func TestClient_GetBid_InvalidResponseFromDSP(t *testing.T) {
	// DSP вернул битую ставку — клиент должен отбросить.
	fake := &fakeDSP{
		getBidResp: &pb.BidResponse{
			BidId:       "bid_1",
			CampaignId:  "camp_1",
			CreativeId:  "cr_1",
			CreativeUrl: "not-a-url",
			ClickUrl:    "https://example.com/click",
			Price:       1_500_000,
		},
	}
	c := startFakeDSP(t, fake)

	_, err := c.GetBid(context.Background(), validBidRequest(), validBannerSlot())
	if err == nil {
		t.Fatal("want error, got nil")
	}
}

// --- Commit ---

func TestClient_Commit_HappyPath(t *testing.T) {
	fake := &fakeDSP{commitResp: &pb.CommitResponse{Ok: true}}
	c := startFakeDSP(t, fake)

	err := c.Commit(context.Background(), "camp_1", 1_500_000)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	req := fake.capturedCommitReq
	if req.GetCampaignId() != "camp_1" {
		t.Errorf("campaign_id = %q", req.GetCampaignId())
	}
	if req.GetPrice() != 1_500_000 {
		t.Errorf("price = %d", req.GetPrice())
	}

	keys := fake.capturedMetadata.Get("api-key")
	if len(keys) == 0 || keys[0] != "test-api-key" {
		t.Errorf("api-key metadata missing")
	}
}

func TestClient_Commit_NotFound(t *testing.T) {
	fake := &fakeDSP{
		commitErr: status.Error(codes.NotFound, "campaign not found"),
	}
	c := startFakeDSP(t, fake)

	err := c.Commit(context.Background(), "camp_1", 1_500_000)
	if !errors.Is(err, domain.ErrCampaignNotFound) {
		t.Errorf("want ErrCampaignNotFound, got %v", err)
	}
}

// --- Rollback ---

func TestClient_Rollback_HappyPath(t *testing.T) {
	fake := &fakeDSP{rollbackResp: &pb.RollbackResponse{Ok: true}}
	c := startFakeDSP(t, fake)

	err := c.Rollback(context.Background(), "camp_1", 1_500_000)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	req := fake.capturedRollbackReq
	if req.GetCampaignId() != "camp_1" {
		t.Errorf("campaign_id = %q", req.GetCampaignId())
	}
	if req.GetPrice() != 1_500_000 {
		t.Errorf("price = %d", req.GetPrice())
	}
}

func TestClient_Rollback_NotFound(t *testing.T) {
	fake := &fakeDSP{
		rollbackErr: status.Error(codes.NotFound, "campaign not found"),
	}
	c := startFakeDSP(t, fake)

	err := c.Rollback(context.Background(), "camp_1", 1_500_000)
	if !errors.Is(err, domain.ErrCampaignNotFound) {
		t.Errorf("want ErrCampaignNotFound, got %v", err)
	}
}

// --- Прочее ---

func TestClient_Name(t *testing.T) {
	fake := &fakeDSP{}
	c := startFakeDSP(t, fake)

	if c.Name() != "test-dsp" {
		t.Errorf("name = %q, want test-dsp", c.Name())
	}
}

func TestClient_Close(t *testing.T) {
	fake := &fakeDSP{}
	c := startFakeDSP(t, fake)

	if err := c.Close(); err != nil {
		t.Errorf("close error: %v", err)
	}
}

func TestClient_GetBid_ContextTimeout(t *testing.T) {
	// DSP отвечает медленно — клиент должен получить DeadlineExceeded.
	fake := &fakeDSP{
		getBidResp: validBidResponse(),
	}
	// Обёртка для замедления: используем задержку в fake-сервере
	slowFake := &slowDSP{delay: 200 * time.Millisecond, inner: fake}
	c := startFakeDSP(t, slowFake)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := c.GetBid(ctx, validBidRequest(), validBannerSlot())
	if err == nil {
		t.Fatal("want error, got nil")
	}
}

// slowDSP — обёртка для задержки ответа.
type slowDSP struct {
	pb.UnimplementedDspServiceServer
	delay time.Duration
	inner *fakeDSP
}

func (s *slowDSP) GetBid(ctx context.Context, req *pb.BidRequest) (*pb.BidResponse, error) {
	select {
	case <-time.After(s.delay):
		return s.inner.GetBid(ctx, req)
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}
