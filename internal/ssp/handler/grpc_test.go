package handler

import (
	"context"
	"errors"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/7cout/minissp/internal/ssp/domain"
	pb "github.com/7cout/minissp/proto/gen/ssp/v1"
)

// --- fake service ---

type fakeSspService struct {
	registerSlotResult *domain.Slot
	registerSlotErr    error

	getSlotResult *domain.Slot
	getSlotErr    error

	runAuctionResult *domain.AuctionResult
	runAuctionErr    error

	impressionErr error

	balance    int64
	balanceErr error

	// captured
	capturedPublisherID string
	capturedSlot        domain.Slot
	capturedName        string
	capturedBidReq      domain.BidRequest
	capturedAuctionID   string
}

func (f *fakeSspService) RegisterSlot(_ context.Context, publisherID string, slot domain.Slot) (*domain.Slot, error) {
	f.capturedPublisherID = publisherID
	f.capturedSlot = slot
	if f.registerSlotErr != nil {
		return nil, f.registerSlotErr
	}
	return f.registerSlotResult, nil
}

func (f *fakeSspService) GetSlotByName(_ context.Context, publisherID, name string) (*domain.Slot, error) {
	f.capturedPublisherID = publisherID
	f.capturedName = name
	if f.getSlotErr != nil {
		return nil, f.getSlotErr
	}
	return f.getSlotResult, nil
}

func (f *fakeSspService) RunAuction(_ context.Context, req domain.BidRequest) (*domain.AuctionResult, error) {
	f.capturedBidReq = req
	if f.runAuctionErr != nil {
		return nil, f.runAuctionErr
	}
	return f.runAuctionResult, nil
}

func (f *fakeSspService) Impression(_ context.Context, auctionID string) error {
	f.capturedAuctionID = auctionID
	return f.impressionErr
}

func (f *fakeSspService) GetPublisherBalance(_ context.Context, publisherID string) (int64, error) {
	f.capturedPublisherID = publisherID
	if f.balanceErr != nil {
		return 0, f.balanceErr
	}
	return f.balance, nil
}

// authedContext — context с publisher_id, как будто прошёл interceptor.
func authedContext() context.Context {
	return context.WithValue(context.Background(), PublisherContextKey{}, "pub_1")
}

// --- RegisterSlot ---

func TestSspServer_RegisterSlot(t *testing.T) {
	validProtoSlotReq := func() *pb.RegisterSlotRequest {
		return &pb.RegisterSlotRequest{
			Name:     "home_banner",
			Geo:      "RU",
			MinPrice: 1_000_000,
			Type:     pb.CreativeType_CREATIVE_TYPE_BANNER,
			Banner:   &pb.Banner{Width: 320, Height: 50},
		}
	}

	t.Run("happy path", func(t *testing.T) {
		svc := &fakeSspService{
			registerSlotResult: &domain.Slot{
				ID:          "slot_1",
				PublisherID: "pub_1",
				Name:        "home_banner",
				Geo:         "RU",
				MinPrice:    1_000_000,
				Type:        domain.CreativeTypeBanner,
				Banner:      &domain.Banner{Width: 320, Height: 50},
			},
		}
		srv := NewSspServer(svc)

		resp, err := srv.RegisterSlot(authedContext(), validProtoSlotReq())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.GetId() != "slot_1" {
			t.Errorf("id = %q", resp.GetId())
		}
		if resp.GetPublisherId() != "pub_1" {
			t.Errorf("publisher_id = %q", resp.GetPublisherId())
		}
		if resp.GetType() != pb.CreativeType_CREATIVE_TYPE_BANNER {
			t.Errorf("type = %v", resp.GetType())
		}
		if resp.GetBanner() == nil || resp.GetBanner().GetWidth() != 320 {
			t.Errorf("banner = %+v", resp.GetBanner())
		}

		// publisher_id взят из context, не из запроса.
		if svc.capturedPublisherID != "pub_1" {
			t.Errorf("captured publisher_id = %q", svc.capturedPublisherID)
		}
	})

	t.Run("unauthorized — InvalidArgument", func(t *testing.T) {
		srv := NewSspServer(&fakeSspService{})

		_, err := srv.RegisterSlot(context.Background(), validProtoSlotReq())
		assertGRPCCode(t, err, codes.Unauthenticated)
	})

	t.Run("invalid slot — InvalidArgument", func(t *testing.T) {
		srv := NewSspServer(&fakeSspService{})
		req := validProtoSlotReq()
		req.Name = "" // пустое имя

		_, err := srv.RegisterSlot(authedContext(), req)
		assertGRPCCode(t, err, codes.InvalidArgument)
	})

	t.Run("service error — mapped", func(t *testing.T) {
		svc := &fakeSspService{registerSlotErr: domain.ErrSlotAlreadyExists}
		srv := NewSspServer(svc)

		_, err := srv.RegisterSlot(authedContext(), validProtoSlotReq())
		assertGRPCCode(t, err, codes.AlreadyExists)
	})
}

// --- GetSlotByName ---

func TestSspServer_GetSlotByName(t *testing.T) {
	t.Run("happy path", func(t *testing.T) {
		svc := &fakeSspService{
			getSlotResult: &domain.Slot{
				ID:          "slot_1",
				PublisherID: "pub_1",
				Name:        "home_banner",
				Geo:         "RU",
				MinPrice:    1_000_000,
				Type:        domain.CreativeTypeBanner,
				Banner:      &domain.Banner{Width: 320, Height: 50},
			},
		}
		srv := NewSspServer(svc)

		resp, err := srv.GetSlotByName(authedContext(), &pb.GetSlotByNameRequest{
			Name: "home_banner",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.GetId() != "slot_1" {
			t.Errorf("id = %q", resp.GetId())
		}
		if svc.capturedName != "home_banner" {
			t.Errorf("captured name = %q", svc.capturedName)
		}
	})

	t.Run("unauthorized", func(t *testing.T) {
		srv := NewSspServer(&fakeSspService{})

		_, err := srv.GetSlotByName(context.Background(), &pb.GetSlotByNameRequest{
			Name: "home_banner",
		})
		assertGRPCCode(t, err, codes.Unauthenticated)
	})

	t.Run("not found", func(t *testing.T) {
		svc := &fakeSspService{getSlotErr: domain.ErrSlotNotFound}
		srv := NewSspServer(svc)

		_, err := srv.GetSlotByName(authedContext(), &pb.GetSlotByNameRequest{
			Name: "missing",
		})
		assertGRPCCode(t, err, codes.NotFound)
	})
}

// --- RunAuction ---

func TestSspServer_RunAuction(t *testing.T) {
	validReq := func() *pb.BidRequest {
		return &pb.BidRequest{
			RequestId: "req_1",
			SlotId:    "slot_1",
			UserId:    "user_1",
		}
	}

	t.Run("happy path", func(t *testing.T) {
		svc := &fakeSspService{
			runAuctionResult: &domain.AuctionResult{
				AuctionID:   "a_1",
				CreativeURL: "https://cdn.example.com/a.jpg",
				ClickURL:    "https://example.com/click",
			},
		}
		srv := NewSspServer(svc)

		resp, err := srv.RunAuction(authedContext(), validReq())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.GetAuctionId() != "a_1" {
			t.Errorf("auction_id = %q", resp.GetAuctionId())
		}
		if resp.GetCreativeUrl() != "https://cdn.example.com/a.jpg" {
			t.Errorf("creative_url = %q", resp.GetCreativeUrl())
		}
		if resp.GetClickUrl() != "https://example.com/click" {
			t.Errorf("click_url = %q", resp.GetClickUrl())
		}

		// Проверка конвертации pb → domain.
		if svc.capturedBidReq.RequestID != "req_1" {
			t.Errorf("request_id = %q", svc.capturedBidReq.RequestID)
		}
		if svc.capturedBidReq.SlotID != "slot_1" {
			t.Errorf("slot_id = %q", svc.capturedBidReq.SlotID)
		}
		if svc.capturedBidReq.UserID != "user_1" {
			t.Errorf("user_id = %q", svc.capturedBidReq.UserID)
		}
	})

	t.Run("unauthorized", func(t *testing.T) {
		srv := NewSspServer(&fakeSspService{})

		_, err := srv.RunAuction(context.Background(), validReq())
		assertGRPCCode(t, err, codes.Unauthenticated)
	})

	t.Run("empty request_id — InvalidArgument", func(t *testing.T) {
		srv := NewSspServer(&fakeSspService{})
		req := validReq()
		req.RequestId = ""

		_, err := srv.RunAuction(authedContext(), req)
		assertGRPCCode(t, err, codes.InvalidArgument)
	})

	t.Run("empty slot_id — InvalidArgument", func(t *testing.T) {
		srv := NewSspServer(&fakeSspService{})
		req := validReq()
		req.SlotId = ""

		_, err := srv.RunAuction(authedContext(), req)
		assertGRPCCode(t, err, codes.InvalidArgument)
	})

	t.Run("no bids — NotFound", func(t *testing.T) {
		svc := &fakeSspService{runAuctionErr: domain.ErrNoBids}
		srv := NewSspServer(svc)

		_, err := srv.RunAuction(authedContext(), validReq())
		assertGRPCCode(t, err, codes.NotFound)
	})

	t.Run("slot not found — NotFound", func(t *testing.T) {
		svc := &fakeSspService{runAuctionErr: domain.ErrSlotNotFound}
		srv := NewSspServer(svc)

		_, err := srv.RunAuction(authedContext(), validReq())
		assertGRPCCode(t, err, codes.NotFound)
	})
}

// --- Impression ---

func TestSspServer_Impression(t *testing.T) {
	t.Run("happy path", func(t *testing.T) {
		svc := &fakeSspService{}
		srv := NewSspServer(svc)

		resp, err := srv.Impression(authedContext(), &pb.ImpressionRequest{
			AuctionId: "a_1",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !resp.GetOk() {
			t.Error("want ok=true")
		}
		if svc.capturedAuctionID != "a_1" {
			t.Errorf("auction_id = %q", svc.capturedAuctionID)
		}
	})

	t.Run("unauthorized", func(t *testing.T) {
		srv := NewSspServer(&fakeSspService{})

		_, err := srv.Impression(context.Background(), &pb.ImpressionRequest{
			AuctionId: "a_1",
		})
		assertGRPCCode(t, err, codes.Unauthenticated)
	})

	t.Run("empty auction_id — InvalidArgument", func(t *testing.T) {
		srv := NewSspServer(&fakeSspService{})

		_, err := srv.Impression(authedContext(), &pb.ImpressionRequest{
			AuctionId: "",
		})
		assertGRPCCode(t, err, codes.InvalidArgument)
	})

	t.Run("auction not found — NotFound", func(t *testing.T) {
		svc := &fakeSspService{impressionErr: domain.ErrAuctionNotFound}
		srv := NewSspServer(svc)

		_, err := srv.Impression(authedContext(), &pb.ImpressionRequest{
			AuctionId: "missing",
		})
		assertGRPCCode(t, err, codes.NotFound)
	})

	t.Run("unexpected error — Internal", func(t *testing.T) {
		svc := &fakeSspService{impressionErr: errors.New("boom")}
		srv := NewSspServer(svc)

		_, err := srv.Impression(authedContext(), &pb.ImpressionRequest{
			AuctionId: "a_1",
		})
		assertGRPCCode(t, err, codes.Internal)
	})
}

// --- GetPublisherBalance ---

func TestSspServer_GetPublisherBalance(t *testing.T) {
	t.Run("happy path", func(t *testing.T) {
		svc := &fakeSspService{balance: 5_000_000}
		srv := NewSspServer(svc)

		resp, err := srv.GetPublisherBalance(authedContext(), &pb.GetPublisherBalanceRequest{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.GetBalance() != 5_000_000 {
			t.Errorf("balance = %d", resp.GetBalance())
		}
		if svc.capturedPublisherID != "pub_1" {
			t.Errorf("publisher_id = %q", svc.capturedPublisherID)
		}
	})

	t.Run("unauthorized", func(t *testing.T) {
		srv := NewSspServer(&fakeSspService{})

		_, err := srv.GetPublisherBalance(context.Background(), &pb.GetPublisherBalanceRequest{})
		assertGRPCCode(t, err, codes.Unauthenticated)
	})

	t.Run("not found", func(t *testing.T) {
		svc := &fakeSspService{balanceErr: domain.ErrPublisherNotFound}
		srv := NewSspServer(svc)

		_, err := srv.GetPublisherBalance(authedContext(), &pb.GetPublisherBalanceRequest{})
		assertGRPCCode(t, err, codes.NotFound)
	})
}

// --- helper ---

// assertGRPCCode проверяет, что ошибка имеет ожидаемый gRPC-статус.
func assertGRPCCode(t *testing.T, err error, want codes.Code) {
	t.Helper()
	if err == nil {
		t.Fatalf("want error, got nil")
	}
	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("not a gRPC status error: %v", err)
	}
	if st.Code() != want {
		t.Errorf("code = %v, want %v", st.Code(), want)
	}
}
