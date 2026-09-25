package domain

import "fmt"

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
