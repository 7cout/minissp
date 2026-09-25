package domain

import "testing"

func TestSlot_Validate(t *testing.T) {
	validSlot := Slot{
		ID:          "slot_1",
		PublisherID: "pub_1",
		Name:        "home_banner",
		Banner:      Banner{Width: 320, Height: 50},
		Geo:         "RU",
		MinPrice:    1_000_000,
	}

	tests := []struct {
		name    string
		modify  func(*Slot)
		wantErr bool
	}{
		{"valid", func(*Slot) {}, false},
		{"empty id", func(s *Slot) { s.ID = "" }, true},
		{"whitespace id", func(s *Slot) { s.ID = "   " }, true},
		{"empty publisher id", func(s *Slot) { s.PublisherID = "" }, true},
		{"whitespace publisher id", func(s *Slot) { s.PublisherID = "   " }, true},
		{"empty name", func(s *Slot) { s.Name = "" }, true},
		{"whitespace name", func(s *Slot) { s.Name = "   " }, true},
		{"invalid banner", func(s *Slot) { s.Banner.Width = 0 }, true},
		{"invalid geo", func(s *Slot) { s.Geo = "RUSSIA" }, true},
		{"lowercase geo", func(s *Slot) { s.Geo = "ru" }, true},
		{"empty geo", func(s *Slot) { s.Geo = "" }, true},
		{"negative min price", func(s *Slot) { s.MinPrice = -1 }, true},
		{"zero min price ok", func(s *Slot) { s.MinPrice = 0 }, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := validSlot
			tt.modify(&s)
			err := s.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr = %v", err, tt.wantErr)
			}
		})
	}
}
