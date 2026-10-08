package domain

import "testing"

func validBannerSlot() Slot {
	return Slot{
		ID:          "slot_1",
		PublisherID: "pub_1",
		Name:        "home_banner",
		Geo:         "RU",
		MinPrice:    1_000_000,
		Type:        CreativeTypeBanner,
		Banner:      &Banner{Width: 320, Height: 50},
	}
}

func validVideoSlot() Slot {
	return Slot{
		ID:          "slot_2",
		PublisherID: "pub_1",
		Name:        "preroll",
		Geo:         "RU",
		MinPrice:    5_000_000,
		Type:        CreativeTypeVideo,
		Video: &Video{
			Width:    640,
			Height:   480,
			Duration: 15,
			MIMEs:    []string{"video/mp4"},
		},
	}
}

func TestSlot_Validate(t *testing.T) {
	t.Run("banner slot", func(t *testing.T) {
		tests := []struct {
			name    string
			modify  func(*Slot)
			wantErr bool
		}{
			{"valid", func(*Slot) {}, false},
			{"empty id", func(s *Slot) { s.ID = "" }, true},
			{"whitespace id", func(s *Slot) { s.ID = "   " }, true},
			{"empty publisher id", func(s *Slot) { s.PublisherID = "" }, true},
			{"empty name", func(s *Slot) { s.Name = "" }, true},
			{"invalid geo", func(s *Slot) { s.Geo = "RUSSIA" }, true},
			{"lowercase geo", func(s *Slot) { s.Geo = "ru" }, true},
			{"empty geo", func(s *Slot) { s.Geo = "" }, true},
			{"negative min price", func(s *Slot) { s.MinPrice = -1 }, true},
			{"zero min price ok", func(s *Slot) { s.MinPrice = 0 }, false},
			{"nil banner", func(s *Slot) { s.Banner = nil }, true},
			{"zero banner width", func(s *Slot) { s.Banner.Width = 0 }, true},
			{"unsupported type", func(s *Slot) { s.Type = "unknown" }, true},
			{"empty type", func(s *Slot) { s.Type = "" }, true},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				s := validBannerSlot()
				tt.modify(&s)
				err := s.Validate()
				if (err != nil) != tt.wantErr {
					t.Errorf("Validate() error = %v, wantErr = %v", err, tt.wantErr)
				}
			})
		}
	})

	t.Run("video slot", func(t *testing.T) {
		tests := []struct {
			name    string
			modify  func(*Slot)
			wantErr bool
		}{
			{"valid", func(*Slot) {}, false},
			{"nil video", func(s *Slot) { s.Video = nil }, true},
			{"zero video duration", func(s *Slot) { s.Video.Duration = 0 }, true},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				s := validVideoSlot()
				tt.modify(&s)
				err := s.Validate()
				if (err != nil) != tt.wantErr {
					t.Errorf("Validate() error = %v, wantErr = %v", err, tt.wantErr)
				}
			})
		}
	})

	t.Run("native slot — not implemented", func(t *testing.T) {
		s := validBannerSlot()
		s.Type = CreativeTypeNative
		s.Banner = nil
		s.Native = &Native{}
		if err := s.Validate(); err == nil {
			t.Error("want error for native, got nil")
		}
	})

	t.Run("audio slot — not implemented", func(t *testing.T) {
		s := validBannerSlot()
		s.Type = CreativeTypeAudio
		s.Banner = nil
		s.Audio = &Audio{}
		if err := s.Validate(); err == nil {
			t.Error("want error for audio, got nil")
		}
	})

	t.Run("native type without native params", func(t *testing.T) {
		s := validBannerSlot()
		s.Type = CreativeTypeNative
		s.Banner = nil
		if err := s.Validate(); err == nil {
			t.Error("want error for native without params, got nil")
		}
	})
}
