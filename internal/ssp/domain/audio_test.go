package domain

import "testing"

func TestAudio_Validate(t *testing.T) {
	if err := (Audio{}).Validate(); err == nil {
		t.Error("want error for audio (not implemented), got nil")
	}
}
