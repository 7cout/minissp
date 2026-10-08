package service

import (
	"context"
	"errors"
	"net"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"

	dspHandler "github.com/7cout/minissp/internal/dsp/handler"
	dspmemory "github.com/7cout/minissp/internal/dsp/repository/memory"
	dspseed "github.com/7cout/minissp/internal/dsp/seed"
	dspservice "github.com/7cout/minissp/internal/dsp/service"
	"github.com/7cout/minissp/internal/ssp/bidder"
	"github.com/7cout/minissp/internal/ssp/domain"
	sspmemory "github.com/7cout/minissp/internal/ssp/repository/memory"
	sspseed "github.com/7cout/minissp/internal/ssp/seed"
	dsppb "github.com/7cout/minissp/proto/gen/dsp/v1"
)

// e2eTestAPIKey — ключ, которым SSP авторизуется в DSP в этом тесте.
const e2eTestAPIKey = "e2e_secret_key"

// e2eFixture собирает всю связку: реальный DSP-сервер на bufconn,
// реальный bidder.Client, реальный SSP-сервис с in-memory репозиториями.
//
// Всё как в продакшене, кроме транспорта: bufconn вместо TCP.
type e2eFixture struct {
	ssp            *Service
	sspPublishers  *sspmemory.PublisherRepo
	sspSlots       *sspmemory.SlotRepo
	dspAdvertisers *dspmemory.AdvertiserRepo
	dspCampaigns   *dspmemory.CampaignRepo
}

func newE2EFixture(t *testing.T) *e2eFixture {
	t.Helper()
	ctx := context.Background()

	// --- DSP-сторона ---

	dspAdvertisers := dspmemory.NewAdvertiserRepo()
	dspCampaigns := dspmemory.NewCampaignRepo()
	dspCreatives := dspmemory.NewCreativeRepo()
	dspseed.PopulateMemory(dspAdvertisers, dspCampaigns, dspCreatives)

	dspSvc := dspservice.New(dspCampaigns, dspCreatives, dspAdvertisers, 150)
	validator := dspHandler.NewStaticAPIKeyValidator([]string{e2eTestAPIKey})
	grpcSrv := grpc.NewServer(
		grpc.UnaryInterceptor(dspHandler.AuthInterceptor(validator)),
	)
	dsppb.RegisterDspServiceServer(grpcSrv, dspHandler.NewDspServer(dspSvc))

	lis := bufconn.Listen(1024 * 1024)
	go func() {
		_ = grpcSrv.Serve(lis)
	}()
	t.Cleanup(grpcSrv.Stop)

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

	dspClient := bidder.NewClientWithConn("dsp-nike", e2eTestAPIKey, conn)

	// --- SSP-сторона ---

	publishers := sspmemory.NewPublisherRepo()
	slots := sspmemory.NewSlotRepo()
	if err := sspseed.PopulateMemory(ctx, publishers, slots); err != nil {
		t.Fatalf("seed ssp: %v", err)
	}

	sspSvc := New(slots, publishers, []BidderClient{dspClient})
	t.Cleanup(func() { _ = sspSvc.Close() })

	return &e2eFixture{
		ssp:            sspSvc,
		sspPublishers:  publishers,
		sspSlots:       slots,
		dspAdvertisers: dspAdvertisers,
		dspCampaigns:   dspCampaigns,
	}
}

// campaignIDOfRecord — возвращает campaign_id из AuctionRecord,
// не удаляя запись из svc.auctions. Используется до Impression.
func (f *e2eFixture) campaignIDOfRecord(t *testing.T, auctionID string) string {
	t.Helper()

	f.ssp.mu.RLock()
	defer f.ssp.mu.RUnlock()

	rec, ok := f.ssp.auctions[auctionID]
	if !ok {
		t.Fatalf("auction record %q not found", auctionID)
	}
	return rec.CampaignID
}

// --- Тест 1: полный happy-path ---

func TestE2E_HappyPath(t *testing.T) {
	f := newE2EFixture(t)
	ctx := context.Background()

	// 1. Аукцион.
	result, err := f.ssp.RunAuction(ctx, domain.BidRequest{
		RequestID: "req_e2e_1",
		SlotID:    sspseed.SlotHomeBanner,
	})
	if err != nil {
		t.Fatalf("RunAuction: %v", err)
	}
	if result.AuctionID == "" {
		t.Fatal("auction_id is empty")
	}
	if result.CreativeURL == "" {
		t.Fatal("creative_url is empty")
	}
	if result.ClickURL == "" {
		t.Fatal("click_url is empty")
	}

	// Узнаём, какая кампания выиграла — чтобы потом проверить
	// именно её состояние в DSP.
	campaignID := f.campaignIDOfRecord(t, result.AuctionID)

	// 2. Impression.
	if err := f.ssp.Impression(ctx, result.AuctionID); err != nil {
		t.Fatalf("Impression: %v", err)
	}

	// 3. Проверяем баланс publisher'а.
	// price = bid_floor * 150 / 100 = 1_500_000.
	// publisher_share = 1_500_000 * 80 / 100 = 1_200_000.
	balance, err := f.ssp.GetPublisherBalance(ctx, sspseed.PublisherT2)
	if err != nil {
		t.Fatalf("GetPublisherBalance: %v", err)
	}
	if balance != 1_200_000 {
		t.Errorf("publisher balance = %d, want 1_200_000", balance)
	}

	// 4. Проверяем состояние кампании в DSP.
	// budget_remaining изначально 5_000_000_000, списано 1_500_000.
	camp, err := f.dspCampaigns.Get(ctx, campaignID)
	if err != nil {
		t.Fatalf("dsp campaign get: %v", err)
	}
	if camp.BudgetReserved != 0 {
		t.Errorf("campaign budget_reserved = %d, want 0", camp.BudgetReserved)
	}
	wantRemaining := int64(5_000_000_000 - 1_500_000)
	if camp.BudgetRemaining != wantRemaining {
		t.Errorf("campaign budget_remaining = %d, want %d", camp.BudgetRemaining, wantRemaining)
	}

	// 5. Проверяем баланс рекламодателя.
	// advertiser.balance изначально 10_000_000_000.
	adv, err := f.dspAdvertisers.Get(ctx, camp.AdvertiserID)
	if err != nil {
		t.Fatalf("dsp advertiser get: %v", err)
	}
	wantAdvBalance := int64(10_000_000_000 - 1_500_000)
	if adv.Balance != wantAdvBalance {
		t.Errorf("advertiser balance = %d, want %d", adv.Balance, wantAdvBalance)
	}
}

// --- Тест 2: DSP не находит кампанию под geo ---

func TestE2E_NoBidsForGeo(t *testing.T) {
	f := newE2EFixture(t)
	ctx := context.Background()

	// Регистрируем слот с geo US. У seed-DSP все кампании на RU.
	slot, err := f.ssp.RegisterSlot(ctx, sspseed.PublisherT2, domain.Slot{
		Name:     "us_banner",
		Geo:      "US",
		MinPrice: 1_000_000,
		Type:     domain.CreativeTypeBanner,
		Banner:   &domain.Banner{Width: 320, Height: 50},
	})
	if err != nil {
		t.Fatalf("RegisterSlot: %v", err)
	}

	_, err = f.ssp.RunAuction(ctx, domain.BidRequest{
		RequestID: "req_e2e_2",
		SlotID:    slot.ID,
	})
	if !errors.Is(err, domain.ErrNoBids) {
		t.Errorf("want ErrNoBids, got %v", err)
	}
}

// --- Тест 3: Impression идемпотентен на уровне всей связки ---

func TestE2E_ImpressionIdempotent(t *testing.T) {
	f := newE2EFixture(t)
	ctx := context.Background()

	result, err := f.ssp.RunAuction(ctx, domain.BidRequest{
		RequestID: "req_e2e_3",
		SlotID:    sspseed.SlotHomeBanner,
	})
	if err != nil {
		t.Fatalf("RunAuction: %v", err)
	}

	campaignID := f.campaignIDOfRecord(t, result.AuctionID)

	// Три Impression подряд. Один должен отработать, два — no-op.
	for i := 0; i < 3; i++ {
		if err := f.ssp.Impression(ctx, result.AuctionID); err != nil {
			t.Fatalf("Impression #%d: %v", i, err)
		}
	}

	// Publisher получил ровно один раз.
	balance, _ := f.ssp.GetPublisherBalance(ctx, sspseed.PublisherT2)
	if balance != 1_200_000 {
		t.Errorf("publisher balance = %d, want 1_200_000 (charged once)", balance)
	}

	// Кампания списана ровно один раз.
	camp, _ := f.dspCampaigns.Get(ctx, campaignID)
	wantRemaining := int64(5_000_000_000 - 1_500_000)
	if camp.BudgetRemaining != wantRemaining {
		t.Errorf("campaign budget_remaining = %d, want %d (charged once)",
			camp.BudgetRemaining, wantRemaining)
	}
	if camp.BudgetReserved != 0 {
		t.Errorf("campaign budget_reserved = %d, want 0", camp.BudgetReserved)
	}
}
