package storage

import (
	"path/filepath"
	"testing"
)

func TestOpenMigratesSchema(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "proxymorph.db"))
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer db.Close()

	tables := []string{"users", "conversion_tasks", "pinned_nodes", "task_node_overrides", "subscription_tokens", "conversion_runs", "output_cache"}
	for _, table := range tables {
		if !db.HasTableForTest(t, table) {
			t.Fatalf("expected table %s to exist", table)
		}
	}
}
