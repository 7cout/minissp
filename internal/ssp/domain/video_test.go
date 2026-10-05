package domain

import "testing"

func TestVideo_Validate(t *testing.T) {
	validVideo := Video{
		Width:    640,
		Height:   480,
		Duration: 30,
		MIMEs:    []string{"video/mp4"},
	}

	tests := []struct {
		name    string
		modify  func(*Video)
		wantErr bool
	}{
		{"valid", func(*Video) {}, false},
		{"valid multiple mimes", func(v *Video) { v.MIMEs = []string{"video/mp4", "video/webm"} }, false},
		{"zero width", func(v *Video) { v.Width = 0 }, true},
		{"negative width", func(v *Video) { v.Width = -1 }, true},
		{"zero height", func(v *Video) { v.Height = 0 }, true},
		{"negative height", func(v *Video) { v.Height = -1 }, true},
		{"zero duration", func(v *Video) { v.Duration = 0 }, true},
		{"negative duration", func(v *Video) { v.Duration = -1 }, true},
		{"nil mimes", func(v *Video) { v.MIMEs = nil }, true},
		{"empty mimes", func(v *Video) { v.MIMEs = []string{} }, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := validVideo
			tt.modify(&v)
			err := v.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr = %v", err, tt.wantErr)
			}
		})
	}
}
