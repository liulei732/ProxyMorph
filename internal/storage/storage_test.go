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

func TestMigrateRemovesLegacyGlobalRuleMergeSetting(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "proxymorph.db"))
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer db.Close()

	if _, err := db.SQL().Exec(`INSERT INTO app_settings (key, value) VALUES ('global_rule_merge_mode', 'upstream_first')`); err != nil {
		t.Fatalf("seed legacy setting: %v", err)
	}
	if err := db.Migrate(t.Context()); err != nil {
		t.Fatalf("Migrate returned error: %v", err)
	}
	var count int
	if err := db.SQL().QueryRow(`SELECT COUNT(*) FROM app_settings WHERE key = 'global_rule_merge_mode'`).Scan(&count); err != nil {
		t.Fatalf("query legacy setting: %v", err)
	}
	if count != 0 {
		t.Fatalf("legacy global rule merge setting count = %d, want 0", count)
	}
}
