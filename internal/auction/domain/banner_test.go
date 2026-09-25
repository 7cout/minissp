package domain

import "testing"

func TestBanner_Validate(t *testing.T) {
	validBanner := Banner{
		Width:  320,
		Height: 50,
	}

	tests := []struct {
		name    string
		modify  func(*Banner)
		wantErr bool
	}{
		{"valid", func(*Banner) {}, false},
		{"zero width", func(b *Banner) { b.Width = 0 }, true},
		{"negative width", func(b *Banner) { b.Width = -1 }, true},
		{"zero height", func(b *Banner) { b.Height = 0 }, true},
		{"negative height", func(b *Banner) { b.Height = -1 }, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := validBanner
			tt.modify(&b)
			err := b.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr = %v", err, tt.wantErr)
			}
		})
	}
}
