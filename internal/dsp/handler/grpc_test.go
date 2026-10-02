package handler

import (
	"context"
	"errors"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/7cout/minissp/internal/dsp/domain"
	pb "github.com/7cout/minissp/proto/gen/dsp/v1"
)

// fakeService — реализация DspService для тестов.
type fakeService struct {
	bid    *domain.Bid
	bidErr error

	commitErr   error
	rollbackErr error

	// capturedReq — что handler передал в сервис.
	capturedReq         domain.BidRequest
	capturedCommitID    string
	capturedCommitAmt   int64
	capturedRollbackID  string
	capturedRollbackAmt int64
}

func (f *fakeService) GetBid(_ context.Context, req domain.BidRequest) (*domain.Bid, error) {
	f.capturedReq = req
	if f.bidErr != nil {
		return nil, f.bidErr
	}
	return f.bid, nil
}

func (f *fakeService) Commit(_ context.Context, campaignID string, price int64) error {
	f.capturedCommitID = campaignID
	f.capturedCommitAmt = price
	return f.commitErr
}

func (f *fakeService) Rollback(_ context.Context, campaignID string, price int64) error {
	f.capturedRollbackID = campaignID
	f.capturedRollbackAmt = price
	return f.rollbackErr
}

// validProtoBidRequest — корректный protobuf-запрос.
func validProtoBidRequest() *pb.BidRequest {
	return &pb.BidRequest{
		RequestId: "req_1",
		ImpId:     "imp_1",
		SlotId:    "slot_1",
		Width:     320,
		Height:    50,
		Geo:       "RU",
		BidFloor:  1_000_000,
		UserId:    "user_1",
	}
}

// --- GetBid ---

func TestDspServer_GetBid(t *testing.T) {
	t.Run("happy path", func(t *testing.T) {
		svc := &fakeService{
			bid: &domain.Bid{
				ID:         "bid_1",
				ImpID:      "imp_1",
				CampaignID: "camp_1",
				CreativeID: "cr_1",
				Price:      1_500_000,
			},
		}
		srv := NewDspServer(svc)

		resp, err := srv.GetBid(context.Background(), validProtoBidRequest())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if resp.GetBidId() != "bid_1" {
			t.Errorf("bid_id = %q, want bid_1", resp.GetBidId())
		}
		if resp.GetCampaignId() != "camp_1" {
			t.Errorf("campaign_id = %q, want camp_1", resp.GetCampaignId())
		}
		if resp.GetCreativeId() != "cr_1" {
			t.Errorf("creative_id = %q, want cr_1", resp.GetCreativeId())
		}
		if resp.GetPrice() != 1_500_000 {
			t.Errorf("price = %d, want 1500000", resp.GetPrice())
		}
	})

	t.Run("converts pb request to domain correctly", func(t *testing.T) {
		svc := &fakeService{
			bid: &domain.Bid{ID: "bid_1", ImpID: "imp_1", CampaignID: "camp_1", CreativeID: "cr_1", Price: 1},
		}
		srv := NewDspServer(svc)

		_, err := srv.GetBid(context.Background(), validProtoBidRequest())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		got := svc.capturedReq
		if got.RequestID != "req_1" {
			t.Errorf("request_id = %q, want req_1", got.RequestID)
		}
		if got.ImpID != "imp_1" {
			t.Errorf("imp_id = %q, want imp_1", got.ImpID)
		}
		if got.SlotID != "slot_1" {
			t.Errorf("slot_id = %q, want slot_1", got.SlotID)
		}
		if got.Width != 320 {
			t.Errorf("width = %d, want 320", got.Width)
		}
		if got.Height != 50 {
			t.Errorf("height = %d, want 50", got.Height)
		}
		if got.Geo != "RU" {
			t.Errorf("geo = %q, want RU", got.Geo)
		}
		if got.BidFloor != 1_000_000 {
			t.Errorf("bid_floor = %d, want 1000000", got.BidFloor)
		}
		if got.UserID != "user_1" {
			t.Errorf("user_id = %q, want user_1", got.UserID)
		}
	})

	t.Run("empty request_id — InvalidArgument", func(t *testing.T) {
		srv := NewDspServer(&fakeService{})
		req := validProtoBidRequest()
		req.RequestId = ""

		_, err := srv.GetBid(context.Background(), req)
		assertGRPCCode(t, err, codes.InvalidArgument)
	})

	t.Run("empty imp_id — InvalidArgument", func(t *testing.T) {
		srv := NewDspServer(&fakeService{})
		req := validProtoBidRequest()
		req.ImpId = ""

		_, err := srv.GetBid(context.Background(), req)
		assertGRPCCode(t, err, codes.InvalidArgument)
	})

	t.Run("zero width — InvalidArgument", func(t *testing.T) {
		srv := NewDspServer(&fakeService{})
		req := validProtoBidRequest()
		req.Width = 0

		_, err := srv.GetBid(context.Background(), req)
		assertGRPCCode(t, err, codes.InvalidArgument)
	})

	t.Run("invalid geo — InvalidArgument", func(t *testing.T) {
		srv := NewDspServer(&fakeService{})
		req := validProtoBidRequest()
		req.Geo = "RUSSIA"

		_, err := srv.GetBid(context.Background(), req)
		assertGRPCCode(t, err, codes.InvalidArgument)
	})
}

// --- Commit ---

func TestDspServer_Commit(t *testing.T) {
	validReq := func() *pb.CommitRequest {
		return &pb.CommitRequest{
			AuctionId:  "a_1",
			CampaignId: "camp_1",
			Price:      1_500_000,
		}
	}

	t.Run("happy path", func(t *testing.T) {
		svc := &fakeService{}
		srv := NewDspServer(svc)

		resp, err := srv.Commit(context.Background(), validReq())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !resp.GetOk() {
			t.Error("want ok=true")
		}
		if svc.capturedCommitID != "camp_1" {
			t.Errorf("campaign_id = %q, want camp_1", svc.capturedCommitID)
		}
		if svc.capturedCommitAmt != 1_500_000 {
			t.Errorf("amount = %d, want 1500000", svc.capturedCommitAmt)
		}
	})

	t.Run("empty campaign_id — InvalidArgument", func(t *testing.T) {
		srv := NewDspServer(&fakeService{})
		req := validReq()
		req.CampaignId = ""

		_, err := srv.Commit(context.Background(), req)
		assertGRPCCode(t, err, codes.InvalidArgument)
	})

	t.Run("zero price — InvalidArgument", func(t *testing.T) {
		srv := NewDspServer(&fakeService{})
		req := validReq()
		req.Price = 0

		_, err := srv.Commit(context.Background(), req)
		assertGRPCCode(t, err, codes.InvalidArgument)
	})

	t.Run("negative price — InvalidArgument", func(t *testing.T) {
		srv := NewDspServer(&fakeService{})
		req := validReq()
		req.Price = -1

		_, err := srv.Commit(context.Background(), req)
		assertGRPCCode(t, err, codes.InvalidArgument)
	})
}

// --- Rollback ---

func TestDspServer_Rollback(t *testing.T) {
	validReq := func() *pb.RollbackRequest {
		return &pb.RollbackRequest{
			AuctionId:  "a_1",
			CampaignId: "camp_1",
			Price:      1_500_000,
		}
	}

	t.Run("happy path", func(t *testing.T) {
		svc := &fakeService{}
		srv := NewDspServer(svc)

		resp, err := srv.Rollback(context.Background(), validReq())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !resp.GetOk() {
			t.Error("want ok=true")
		}
		if svc.capturedRollbackID != "camp_1" {
			t.Errorf("campaign_id = %q, want camp_1", svc.capturedRollbackID)
		}
		if svc.capturedRollbackAmt != 1_500_000 {
			t.Errorf("amount = %d, want 1500000", svc.capturedRollbackAmt)
		}
	})

	t.Run("empty campaign_id — InvalidArgument", func(t *testing.T) {
		srv := NewDspServer(&fakeService{})
		req := validReq()
		req.CampaignId = ""

		_, err := srv.Rollback(context.Background(), req)
		assertGRPCCode(t, err, codes.InvalidArgument)
	})

	t.Run("zero price — InvalidArgument", func(t *testing.T) {
		srv := NewDspServer(&fakeService{})
		req := validReq()
		req.Price = 0

		_, err := srv.Rollback(context.Background(), req)
		assertGRPCCode(t, err, codes.InvalidArgument)
	})
}

// --- Маппинг ошибок ---

func TestDspServer_errorMapping(t *testing.T) {
	tests := []struct {
		name     string
		svcErr   error
		wantCode codes.Code
	}{
		{"no eligible campaign", domain.ErrNoEligibleCampaign, codes.NotFound},
		{"campaign not found", domain.ErrCampaignNotFound, codes.NotFound},
		{"creative not found", domain.ErrCreativeNotFound, codes.NotFound},
		{"advertiser not found", domain.ErrAdvertiserNotFound, codes.NotFound},
		{"insufficient budget", domain.ErrInsufficientBudget, codes.FailedPrecondition},
		{"insufficient balance", domain.ErrInsufficientBalance, codes.FailedPrecondition},
		{"invalid id", domain.ErrInvalidID, codes.InvalidArgument},
		{"context cancelled", context.Canceled, codes.Canceled},
		{"deadline exceeded", context.DeadlineExceeded, codes.DeadlineExceeded},
		{"unknown error", errors.New("boom"), codes.Internal},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &fakeService{bidErr: tt.svcErr}
			srv := NewDspServer(svc)

			_, err := srv.GetBid(context.Background(), validProtoBidRequest())
			assertGRPCCode(t, err, tt.wantCode)
		})
	}
}

// --- Helper ---

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
