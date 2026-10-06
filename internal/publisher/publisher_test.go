package publisher

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"google.golang.org/grpc"

	sspv1 "github.com/7cout/minissp/proto/gen/ssp/v1"
)

// --- fake SspClient ---

type fakeSspClient struct {
	mu sync.Mutex

	slotResp       *sspv1.Slot
	slotErr        error
	auctionResp    *sspv1.BidResponse
	auctionErr     error
	impressionResp *sspv1.ImpressionResponse
	impressionErr  error
	balanceResp    *sspv1.GetPublisherBalanceResponse
	balanceErr     error

	auctionCalls    int
	impressionCalls int
}

func (f *fakeSspClient) GetSlotByName(_ context.Context, _ *sspv1.GetSlotByNameRequest, _ ...grpc.CallOption) (*sspv1.Slot, error) {
	if f.slotErr != nil {
		return nil, f.slotErr
	}
	return f.slotResp, nil
}

func (f *fakeSspClient) RunAuction(_ context.Context, _ *sspv1.BidRequest, _ ...grpc.CallOption) (*sspv1.BidResponse, error) {
	f.mu.Lock()
	f.auctionCalls++
	f.mu.Unlock()
	if f.auctionErr != nil {
		return nil, f.auctionErr
	}
	return f.auctionResp, nil
}

func (f *fakeSspClient) Impression(_ context.Context, _ *sspv1.ImpressionRequest, _ ...grpc.CallOption) (*sspv1.ImpressionResponse, error) {
	f.mu.Lock()
	f.impressionCalls++
	f.mu.Unlock()
	if f.impressionErr != nil {
		return nil, f.impressionErr
	}
	return f.impressionResp, nil
}

func (f *fakeSspClient) GetPublisherBalance(_ context.Context, _ *sspv1.GetPublisherBalanceRequest, _ ...grpc.CallOption) (*sspv1.GetPublisherBalanceResponse, error) {
	if f.balanceErr != nil {
		return nil, f.balanceErr
	}
	return f.balanceResp, nil
}

func (f *fakeSspClient) impressionCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.impressionCalls
}

// --- helpers ---

func newTestPublisher(t *testing.T, client SspClient, cfg Config) *Publisher {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return New(cfg, client, logger)
}

func validConfig() Config {
	return Config{
		SSPAddr:         "localhost:50051",
		APIKey:          "test-key",
		SlotName:        "home_banner",
		RPS:             1,
		ImpressionDelay: 0, // в тестах не спим
		ReportInterval:  time.Hour,
	}
}

func validSlot() *sspv1.Slot {
	return &sspv1.Slot{
		Id:       "slot_1",
		Name:     "home_banner",
		Geo:      "RU",
		MinPrice: 1_000_000,
	}
}

func validBidResponse() *sspv1.BidResponse {
	return &sspv1.BidResponse{
		AuctionId:   "a_1",
		CreativeUrl: "https://cdn.example.com/a.jpg",
		ClickUrl:    "https://example.com/click",
	}
}

// --- Tick ---

func TestPublisher_Tick_HappyPath(t *testing.T) {
	client := &fakeSspClient{
		auctionResp:    validBidResponse(),
		impressionResp: &sspv1.ImpressionResponse{Ok: true},
	}
	p := newTestPublisher(t, client, validConfig())
	p.slotID = "slot_1"

	p.Tick(context.Background())

	s := p.Stats()
	if s.AuctionsAttempted != 1 {
		t.Errorf("attempted = %d, want 1", s.AuctionsAttempted)
	}
	if s.AuctionsWon != 1 {
		t.Errorf("won = %d, want 1", s.AuctionsWon)
	}
	if s.ImpressionsOK != 1 {
		t.Errorf("impressions_ok = %d, want 1", s.ImpressionsOK)
	}
	if s.ImpressionsFailed != 0 {
		t.Errorf("impressions_failed = %d, want 0", s.ImpressionsFailed)
	}
}

func TestPublisher_Tick_AuctionFails(t *testing.T) {
	client := &fakeSspClient{
		auctionErr: errors.New("ssp down"),
	}
	p := newTestPublisher(t, client, validConfig())
	p.slotID = "slot_1"

	p.Tick(context.Background())

	s := p.Stats()
	if s.AuctionsAttempted != 1 {
		t.Errorf("attempted = %d, want 1", s.AuctionsAttempted)
	}
	if s.AuctionsFailed != 1 {
		t.Errorf("failed = %d, want 1", s.AuctionsFailed)
	}
	if s.AuctionsWon != 0 {
		t.Errorf("won = %d, want 0", s.AuctionsWon)
	}
	if client.impressionCount() != 0 {
		t.Errorf("impression should not be called when auction failed")
	}
}

func TestPublisher_Tick_ImpressionFails(t *testing.T) {
	client := &fakeSspClient{
		auctionResp:   validBidResponse(),
		impressionErr: errors.New("impression failed"),
	}
	p := newTestPublisher(t, client, validConfig())
	p.slotID = "slot_1"

	p.Tick(context.Background())

	s := p.Stats()
	if s.AuctionsWon != 1 {
		t.Errorf("won = %d, want 1", s.AuctionsWon)
	}
	if s.ImpressionsOK != 0 {
		t.Errorf("impressions_ok = %d, want 0", s.ImpressionsOK)
	}
	if s.ImpressionsFailed != 1 {
		t.Errorf("impressions_failed = %d, want 1", s.ImpressionsFailed)
	}
}

// --- Run ---

func TestPublisher_Run_SlotNotFound(t *testing.T) {
	client := &fakeSspClient{
		slotErr: errors.New("slot not found"),
	}
	p := newTestPublisher(t, client, validConfig())

	err := p.Run(context.Background())
	if err == nil {
		t.Fatal("want error, got nil")
	}
}

func TestPublisher_Run_ShortDuration(t *testing.T) {
	client := &fakeSspClient{
		slotResp:       validSlot(),
		auctionResp:    validBidResponse(),
		impressionResp: &sspv1.ImpressionResponse{Ok: true},
		balanceResp:    &sspv1.GetPublisherBalanceResponse{Balance: 100_000},
	}
	cfg := validConfig()
	cfg.RPS = 100
	cfg.Duration = 100 * time.Millisecond

	p := newTestPublisher(t, client, cfg)

	err := p.Run(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	s := p.Stats()
	if s.AuctionsAttempted == 0 {
		t.Error("attempted = 0, want > 0")
	}
	if s.AuctionsWon != s.ImpressionsOK {
		t.Errorf("won = %d, impressions_ok = %d, want equal", s.AuctionsWon, s.ImpressionsOK)
	}
}

func TestPublisher_Run_ContextCancelled(t *testing.T) {
	client := &fakeSspClient{
		slotResp:       validSlot(),
		auctionResp:    validBidResponse(),
		impressionResp: &sspv1.ImpressionResponse{Ok: true},
		balanceResp:    &sspv1.GetPublisherBalanceResponse{Balance: 0},
	}
	cfg := validConfig()
	cfg.RPS = 100
	// Duration не выставляем — ждём отмену извне.

	p := newTestPublisher(t, client, cfg)

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)
	go func() { done <- p.Run(ctx) }()

	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Run did not return after context cancellation")
	}
}

// --- Config ---

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		modify  func(*Config)
		wantErr bool
	}{
		{"valid", func(*Config) {}, false},
		{"empty addr", func(c *Config) { c.SSPAddr = "" }, true},
		{"empty api key", func(c *Config) { c.APIKey = "" }, true},
		{"empty slot name", func(c *Config) { c.SlotName = "" }, true},
		{"zero rps", func(c *Config) { c.RPS = 0 }, true},
		{"negative rps", func(c *Config) { c.RPS = -1 }, true},
		{"negative duration", func(c *Config) { c.Duration = -time.Second }, true},
		{"negative impression delay", func(c *Config) { c.ImpressionDelay = -time.Second }, true},
		{"zero report interval", func(c *Config) { c.ReportInterval = 0 }, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validConfig()
			tt.modify(&cfg)
			err := cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr = %v", err, tt.wantErr)
			}
		})
	}
}
