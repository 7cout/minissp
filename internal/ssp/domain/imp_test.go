package domain

import "testing"

func validBannerImp() Imp {
	return Imp{
		ID:       "imp_1",
		SlotID:   "slot_1",
		BidFloor: 1_000_000,
		Type:     CreativeTypeBanner,
		Banner:   &Banner{Width: 320, Height: 50},
	}
}

func validVideoImp() Imp {
	return Imp{
		ID:       "imp_2",
		SlotID:   "slot_2",
		BidFloor: 5_000_000,
		Type:     CreativeTypeVideo,
		Video: &Video{
			Width:    640,
			Height:   480,
			Duration: 15,
			MIMEs:    []string{"video/mp4"},
		},
	}
}

func TestImp_Validate(t *testing.T) {
	t.Run("banner imp", func(t *testing.T) {
		tests := []struct {
			name    string
			modify  func(*Imp)
			wantErr bool
		}{
			{"valid", func(*Imp) {}, false},
			{"empty id", func(i *Imp) { i.ID = "" }, true},
			{"whitespace id", func(i *Imp) { i.ID = "   " }, true},
			{"empty slot id", func(i *Imp) { i.SlotID = "" }, true},
			{"negative bid floor", func(i *Imp) { i.BidFloor = -1 }, true},
			{"zero bid floor ok", func(i *Imp) { i.BidFloor = 0 }, false},
			{"nil banner", func(i *Imp) { i.Banner = nil }, true},
			{"zero banner width", func(i *Imp) { i.Banner.Width = 0 }, true},
			{"unsupported type", func(i *Imp) { i.Type = "unknown" }, true},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				i := validBannerImp()
				tt.modify(&i)
				err := i.Validate()
				if (err != nil) != tt.wantErr {
					t.Errorf("Validate() error = %v, wantErr = %v", err, tt.wantErr)
				}
			})
		}
	})

	t.Run("video imp", func(t *testing.T) {
		tests := []struct {
			name    string
			modify  func(*Imp)
			wantErr bool
		}{
			{"valid", func(*Imp) {}, false},
			{"nil video", func(i *Imp) { i.Video = nil }, true},
			{"zero video duration", func(i *Imp) { i.Video.Duration = 0 }, true},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				i := validVideoImp()
				tt.modify(&i)
				err := i.Validate()
				if (err != nil) != tt.wantErr {
					t.Errorf("Validate() error = %v, wantErr = %v", err, tt.wantErr)
				}
			})
		}
	})

	t.Run("native imp — not implemented", func(t *testing.T) {
		i := validBannerImp()
		i.Type = CreativeTypeNative
		i.Banner = nil
		i.Native = &Native{}
		if err := i.Validate(); err == nil {
			t.Error("want error for native, got nil")
		}
	})

	t.Run("audio imp — not implemented", func(t *testing.T) {
		i := validBannerImp()
		i.Type = CreativeTypeAudio
		i.Banner = nil
		i.Audio = &Audio{}
		if err := i.Validate(); err == nil {
			t.Error("want error for audio, got nil")
		}
	})
}
