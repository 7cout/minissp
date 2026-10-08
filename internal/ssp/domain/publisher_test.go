package domain

import "testing"

func TestPublisher_Validate(t *testing.T) {
	validPublisher := Publisher{
		ID:      "pub_1",
		Name:    "T2",
		APIKey:  "ssp_dev_secret_key_777",
		Balance: 0,
	}

	tests := []struct {
		name    string
		modify  func(*Publisher)
		wantErr bool
	}{
		{"valid", func(*Publisher) {}, false},
		{"valid with balance", func(p *Publisher) { p.Balance = 1_000_000 }, false},
		{"empty id", func(p *Publisher) { p.ID = "" }, true},
		{"whitespace id", func(p *Publisher) { p.ID = "   " }, true},
		{"empty name", func(p *Publisher) { p.Name = "" }, true},
		{"whitespace name", func(p *Publisher) { p.Name = "   " }, true},
		{"empty api key", func(p *Publisher) { p.APIKey = "" }, true},
		{"whitespace api key", func(p *Publisher) { p.APIKey = "   " }, true},
		{"negative balance", func(p *Publisher) { p.Balance = -1 }, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := validPublisher
			tt.modify(&p)
			err := p.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr = %v", err, tt.wantErr)
			}
		})
	}
}
