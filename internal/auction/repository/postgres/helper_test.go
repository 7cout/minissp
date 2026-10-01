//go:build integration

package postgres

import (
	"context"
	"testing"
)

// cleanTables очищает все таблицы перед тестом.
// CASCADE — удаляет связанные строки (creatives зависят от campaigns).
func cleanTables(t *testing.T) {
	t.Helper()
	_, err := testPool.Exec(context.Background(),
		"TRUNCATE creatives, campaigns, ad_slots CASCADE")
	if err != nil {
		t.Fatalf("clean tables: %v", err)
	}
}
