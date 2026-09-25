package domain

import "testing"

func TestImp_Validate(t *testing.T) {
	validImp := Imp{
		ID:       "imp_1",
		SlotID:   "slot_1",
		Banner:   Banner{Width: 320, Height: 50},
		BidFloor: 1_000_000,
	}

	tests := []struct {
		name    string
		modify  func(*Imp)
		wantErr bool
	}{
		{"valid", func(*Imp) {}, false},
		{"empty id", func(i *Imp) { i.ID = "" }, true},
		{"whitespace id", func(i *Imp) { i.ID = "   " }, true},
		{"empty slot id", func(i *Imp) { i.SlotID = "" }, true},
		{"whitespace slot id", func(i *Imp) { i.SlotID = "   " }, true},
		{"invalid banner", func(i *Imp) { i.Banner.Width = 0 }, true},
		{"negative bid floor", func(i *Imp) { i.BidFloor = -1 }, true},
		{"zero bid floor ok", func(i *Imp) { i.BidFloor = 0 }, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			i := validImp
			tt.modify(&i)
			err := i.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr = %v", err, tt.wantErr)
			}
		})
	}
}
