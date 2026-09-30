package handler

import (
	"context"
	"errors"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/7cout/minissp/internal/auction/domain"
	pb "github.com/7cout/minissp/proto/gen/auction/v1"
)

// fakeService - реализация AuctionService для тестов
type fakeService struct {
	result *domain.AuctionResult
	err    error

	// capturedReq - что handler передал в сервис
	// Нужно, чтобы проверить конвертацию pb - domain
	capturedReq domain.BidRequest
}

func (f *fakeService) RunAuction(_ context.Context, req domain.BidRequest) (*domain.AuctionResult, error) {
	f.capturedReq = req
	if f.err != nil {
		return nil, f.err
	}
	return f.result, nil
}

// validProtoRequest - корректный protobuf-запрос
func validProtoRequest() *pb.BidRequest {
	return &pb.BidRequest{
		Id:     "req_1",
		UserId: "user_1",
		Imp: &pb.Imp{
			Id:       "imp_1",
			SlotId:   "slot_1",
			Banner:   &pb.Banner{Width: 320, Height: 50},
			BidFloor: 1_000_000,
		},
	}
}

func TestAuctionServer_RunAuction(t *testing.T) {
	t.Run("happy path", func(t *testing.T) {
		svc := &fakeService{
			result: &domain.AuctionResult{
				AuctionID:  "auction_1",
				ImpID:      "imp_1",
				BidID:      "bid_1",
				CampaignID: "camp_1",
				CreativeID: "creative_1",
				Price:      5_000_000,
			},
		}
		srv := NewAuctionServer(svc)

		resp, err := srv.RunAuction(context.Background(), validProtoRequest())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if resp.GetAuctionId() != "auction_1" {
			t.Errorf("auction_id = %q, want auction_1", resp.GetAuctionId())
		}
		if resp.GetCampaignId() != "camp_1" {
			t.Errorf("campaign_id = %q, want camp_1", resp.GetCampaignId())
		}
		if resp.GetPrice() != 5_000_000 {
			t.Errorf("price = %d, want 5000000", resp.GetPrice())
		}
	})

	t.Run("converts pb request to domain correctly", func(t *testing.T) {
		svc := &fakeService{
			result: &domain.AuctionResult{AuctionID: "a1"},
		}
		srv := NewAuctionServer(svc)

		_, err := srv.RunAuction(context.Background(), validProtoRequest())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Проверяем, что handler передал в сервис правильные данные
		got := svc.capturedReq
		if got.ID != "req_1" {
			t.Errorf("req.ID = %q, want req_1", got.ID)
		}
		if got.UserID != "user_1" {
			t.Errorf("req.UserID = %q, want user_1", got.UserID)
		}
		if got.Imp.ID != "imp_1" {
			t.Errorf("req.Imp.ID = %q, want imp_1", got.Imp.ID)
		}
		if got.Imp.SlotID != "slot_1" {
			t.Errorf("req.Imp.SlotID = %q, want slot_1", got.Imp.SlotID)
		}
		if got.Imp.Banner.Width != 320 {
			t.Errorf("banner width = %d, want 320", got.Imp.Banner.Width)
		}
		if got.Imp.Banner.Height != 50 {
			t.Errorf("banner height = %d, want 50", got.Imp.Banner.Height)
		}
		if got.Imp.BidFloor != 1_000_000 {
			t.Errorf("bid floor = %d, want 1000000", got.Imp.BidFloor)
		}
	})

	t.Run("empty id - InvalidArgument", func(t *testing.T) {
		srv := NewAuctionServer(&fakeService{})
		req := validProtoRequest()
		req.Id = ""

		_, err := srv.RunAuction(context.Background(), req)
		assertGRPCCode(t, err, codes.InvalidArgument)
	})

	t.Run("nil imp - InvalidArgument", func(t *testing.T) {
		srv := NewAuctionServer(&fakeService{})
		req := validProtoRequest()
		req.Imp = nil

		_, err := srv.RunAuction(context.Background(), req)
		assertGRPCCode(t, err, codes.InvalidArgument)
	})

	t.Run("invalid imp - InvalidArgument", func(t *testing.T) {
		srv := NewAuctionServer(&fakeService{})
		req := validProtoRequest()
		req.Imp.SlotId = "" // domain.Validate() поругается

		_, err := srv.RunAuction(context.Background(), req)
		assertGRPCCode(t, err, codes.InvalidArgument)
	})
}

func TestAuctionServer_RunAuction_errorMapping(t *testing.T) {
	tests := []struct {
		name     string
		svcErr   error
		wantCode codes.Code
	}{
		{"slot not found", domain.ErrSlotNotFound, codes.NotFound},
		{"no bids", domain.ErrNoBids, codes.NotFound},
		{"insufficient budget", domain.ErrInsufficientBudget, codes.FailedPrecondition},
		{"unsupported geo", domain.ErrUnsupportedGeo, codes.InvalidArgument},
		{"context cancelled", context.Canceled, codes.Canceled},
		{"deadline exceeded", context.DeadlineExceeded, codes.DeadlineExceeded},
		{"unknown error", errors.New("boom"), codes.Internal},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := NewAuctionServer(&fakeService{err: tt.svcErr})

			_, err := srv.RunAuction(context.Background(), validProtoRequest())
			assertGRPCCode(t, err, tt.wantCode)
		})
	}
}

// assertGRPCCode проверяет, что ошибка имеет ожидаемый gRPC-статус
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
