package domain

import "testing"

func TestAdvertiser_Validate(t *testing.T) {
	validAdvertiser := Advertiser{
		ID:      "adv_1",
		Name:    "Nike",
		Balance: 1_000_000_000,
	}

	tests := []struct {
		name    string
		modify  func(*Advertiser)
		wantErr bool
	}{
		{"valid", func(*Advertiser) {}, false},
		{"empty id", func(a *Advertiser) { a.ID = "" }, true},
		{"whitespace id", func(a *Advertiser) { a.ID = "   " }, true},
		{"empty name", func(a *Advertiser) { a.Name = "" }, true},
		{"whitespace name", func(a *Advertiser) { a.Name = "   " }, true},
		{"negative balance", func(a *Advertiser) { a.Balance = -1 }, true},
		{"zero balance ok", func(a *Advertiser) { a.Balance = 0 }, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := validAdvertiser
			tt.modify(&a)
			err := a.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr = %v", err, tt.wantErr)
			}
		})
	}
}
