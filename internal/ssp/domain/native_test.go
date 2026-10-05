package domain

import "testing"

func TestNative_Validate(t *testing.T) {
	if err := (Native{}).Validate(); err == nil {
		t.Error("want error for native (not implemented), got nil")
	}
}
