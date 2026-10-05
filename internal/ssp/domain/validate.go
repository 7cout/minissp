package domain

import (
	"errors"
	"fmt"
	"net/url"
)

// validateGeoCode проверяет, что код страны в формате ISO 3166-1 alpha-2:
// ровно две заглавные латинские буквы.
func validateGeoCode(geo string) error {
	if len(geo) != 2 {
		return fmt.Errorf("must be 2 letters, got %q", geo)
	}
	for _, r := range geo {
		if r < 'A' || r > 'Z' {
			return fmt.Errorf("must be uppercase A-Z, got %q", geo)
		}
	}
	return nil
}

// validateURL проверяет, что URL удовлетворяет формату:
// http:// или https://, содержит хост и не пустой.
func validateURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("invalid url: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("url must use http or https, got %q", u.Scheme)
	}
	if u.Host == "" {
		return errors.New("url must have a host")
	}
	return nil
}
