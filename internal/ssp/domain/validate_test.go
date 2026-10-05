package domain

import "testing"

func TestValidateGeoCode(t *testing.T) {
	tests := []struct {
		name    string
		geo     string
		wantErr bool
	}{
		{"valid RU", "RU", false},
		{"valid US", "US", false},
		{"valid KZ", "KZ", false},
		{"empty", "", true},
		{"one letter", "R", true},
		{"three letters", "RUS", true},
		{"lowercase", "ru", true},
		{"mixed case", "Ru", true},
		{"digits", "R1", true},
		{"symbols", "R!", true},
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

func TestValidateURL(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr bool
	}{
		{"valid https", "https://example.com/banner.jpg", false},
		{"valid http", "http://example.com/banner.jpg", false},
		{"valid with path", "https://cdn.example.com/a/b/c.png", false},
		{"valid with query", "https://example.com/img?x=1&y=2", false},
		{"empty", "", true},
		{"no scheme", "example.com/banner.jpg", true},
		{"ftp scheme", "ftp://example.com/banner.jpg", true},
		{"no host", "https://", true},
		{"malformed", "http://[::1", true},
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
