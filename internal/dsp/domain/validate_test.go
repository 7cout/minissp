package domain

import "testing"

func TestValidateURL(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr bool
	}{
		{"valid https", "https://example.com/banner.jpg", false},
		{"valid http", "http://example.com/banner.jpg", false},
		{"empty", "", true},
		{"no scheme", "example.com/banner.jpg", true},
		{"ftp scheme", "ftp://example.com/banner.jpg", true},
		{"no host", "https://", true},
		{"malformed", "http://[::1", true}, // ← незакрытая скобка — ошибка url.Parse
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateURL(tt.url)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateURL(%q) error = %v, wantErr = %v", tt.url, err, tt.wantErr)
			}
		})
	}
}

func TestValidateGeoCode(t *testing.T) {
	tests := []struct {
		name    string
		geo     string
		wantErr bool
	}{
		{"valid RU", "RU", false},
		{"valid US", "US", false},
		{"empty", "", true},
		{"one letter", "R", true},
		{"three letters", "RUS", true},
		{"lowercase", "ru", true},
		{"digits", "R1", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateGeoCode(tt.geo)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateGeoCode(%q) error = %v, wantErr = %v", tt.geo, err, tt.wantErr)
			}
		})
	}
}
