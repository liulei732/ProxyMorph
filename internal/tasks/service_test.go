package tasks

import (
	"path/filepath"
	"testing"

	"github.com/liulei/proxymorph/internal/storage"
)

func TestCreateAndListTasks(t *testing.T) {
	t.Setenv("PROXYMORPH_PUBLIC_BASE_URL", "")
	db, err := storage.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	userID := seedUser(t, db)
	service := NewService(db)

	task, err := service.Create(userID, CreateInput{Name: "Main", SourceURL: "https://example.com/clash.yaml", RefreshIntervalSeconds: 3600})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if task.InputType != "clash" || task.OutputType != "surge6" || !task.Enabled {
		t.Fatalf("unexpected task defaults: %#v", task)
	}
	tasks, err := service.List(userID)
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(tasks) != 1 || tasks[0].Name != "Main" {
		t.Fatalf("unexpected tasks: %#v", tasks)
	}
	if tasks[0].SubscriptionURL == "" {
		t.Fatalf("expected subscription URL: %#v", tasks[0])
	}
	if tasks[0].SubscriptionURL != "/sub/"+tasks[0].SubscriptionToken+"?name=Main" {
		t.Fatalf("SubscriptionURL = %q, want token URL with name", tasks[0].SubscriptionURL)
	}
	var taskID int64
	if err := db.SQL().QueryRow(`SELECT task_id FROM subscription_tokens WHERE token = ?`, tasks[0].SubscriptionToken).Scan(&taskID); err != nil {
		t.Fatalf("expected stored subscription token: %v", err)
	}
	if taskID != task.ID {
		t.Fatalf("token task_id = %d, want %d", taskID, task.ID)
	}
}

func TestCreateUsesNameQueryWhenNameIsBlank(t *testing.T) {
	t.Setenv("PROXYMORPH_PUBLIC_BASE_URL", "https://proxy.example.test")
	db, err := storage.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	userID := seedUser(t, db)
	service := NewService(db)

	task, err := service.Create(userID, CreateInput{
		Name:                   "  ",
		SourceURL:              "https://upstream.example.test/sub?token=abc&name=%E9%A6%99%E6%B8%AF%20A",
		RefreshIntervalSeconds: 3600,
	})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if task.Name != "香港 A" {
		t.Fatalf("Name = %q, want name query value", task.Name)
	}
	wantPrefix := "https://proxy.example.test/sub/" + task.SubscriptionToken + "?name=%E9%A6%99%E6%B8%AF+A"
	if task.SubscriptionURL != wantPrefix {
		t.Fatalf("SubscriptionURL = %q, want %q", task.SubscriptionURL, wantPrefix)
	}
}

func TestListTasksReturnsEmptySlice(t *testing.T) {
	db, err := storage.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	userID := seedUser(t, db)
	service := NewService(db)

	tasks, err := service.List(userID)
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if tasks == nil {
		t.Fatal("List returned nil; want empty slice")
	}
	if len(tasks) != 0 {
		t.Fatalf("unexpected tasks: %#v", tasks)
	}
}

func TestUpdateTaskChangesEditableFields(t *testing.T) {
	db, err := storage.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	userID := seedUser(t, db)
	service := NewService(db)
	task, err := service.Create(userID, CreateInput{Name: "Main", SourceURL: "https://example.com/a.yaml", RefreshIntervalSeconds: 3600})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	enabled := false
	mergeDefaults := false
	next, err := service.Update(userID, task.ID, UpdateInput{
		Name:                    stringPtr("Updated"),
		SourceURL:               stringPtr("https://example.com/b.yaml"),
		RefreshIntervalSeconds:  intPtr(7200),
		Enabled:                 &enabled,
		MergeDefaultPinnedNodes: &mergeDefaults,
	})
	if err != nil {
		t.Fatalf("Update returned error: %v", err)
	}
	if next.Name != "Updated" || next.SourceURL != "https://example.com/b.yaml" || next.RefreshIntervalSeconds != 7200 || next.Enabled || next.MergeDefaultPinnedNodes {
		t.Fatalf("unexpected updated task: %#v", next)
	}
}

func TestUpdateTaskChangesSurgeConfigFields(t *testing.T) {
	db, err := storage.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	userID := seedUser(t, db)
	service := NewService(db)
	task, err := service.Create(userID, CreateInput{Name: "Main", SourceURL: "https://example.com/a.yaml", RefreshIntervalSeconds: 3600})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	includeGlobal := false
	managedEnabled := true
	managedStrict := true
	next, err := service.Update(userID, task.ID, UpdateInput{
		IncludeGlobalRules:           &includeGlobal,
		CustomRulesText:              stringPtr("DOMAIN,task.example,DIRECT"),
		RuleMergeMode:                stringPtr("upstream_first_dedupe"),
		CustomGroupsText:             stringPtr("Manual = select, Proxy, DIRECT"),
		ManagedConfigEnabled:         &managedEnabled,
		ManagedConfigIntervalSeconds: intPtr(7200),
		ManagedConfigStrict:          &managedStrict,
	})
	if err != nil {
		t.Fatalf("Update returned error: %v", err)
	}
	if next.IncludeGlobalRules || next.CustomRulesText != "DOMAIN,task.example,DIRECT" || next.RuleMergeMode != "upstream_first_dedupe" {
		t.Fatalf("unexpected rule config: %#v", next)
	}
	if next.CustomGroupsText != "Manual = select, Proxy, DIRECT" || !next.ManagedConfigEnabled || next.ManagedConfigIntervalSeconds != 7200 || !next.ManagedConfigStrict {
		t.Fatalf("unexpected group/managed config: %#v", next)
	}
}

func TestGlobalRuleConfigRoundTrip(t *testing.T) {
	db, err := storage.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	service := NewService(db)

	initial, err := service.GlobalRuleConfig()
	if err != nil {
		t.Fatalf("GlobalRuleConfig returned error: %v", err)
	}
	if initial.CustomRulesText != "" {
		t.Fatalf("unexpected initial config: %#v", initial)
	}
	updated, err := service.UpdateGlobalRuleConfig(GlobalRuleConfigInput{
		CustomRulesText: "DOMAIN,global.example,DIRECT",
	})
	if err != nil {
		t.Fatalf("UpdateGlobalRuleConfig returned error: %v", err)
	}
	if updated.CustomRulesText != "DOMAIN,global.example,DIRECT" {
		t.Fatalf("unexpected updated config: %#v", updated)
	}
}

func TestUpdateRejectsInvalidSurgeConfigText(t *testing.T) {
	db, err := storage.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	userID := seedUser(t, db)
	service := NewService(db)
	task, err := service.Create(userID, CreateInput{Name: "Main", SourceURL: "https://example.com/a.yaml", RefreshIntervalSeconds: 3600})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	if _, err := service.Update(userID, task.ID, UpdateInput{CustomRulesText: stringPtr("[Rule]\nDOMAIN,example.com,DIRECT")}); err == nil {
		t.Fatal("Update accepted [Rule] in custom rules")
	}
	if _, err := service.Update(userID, task.ID, UpdateInput{CustomGroupsText: stringPtr("[Proxy Group]\nManual = select, Proxy")}); err == nil {
		t.Fatal("Update accepted [Proxy Group] in custom groups")
	}
	if _, err := service.UpdateGlobalRuleConfig(GlobalRuleConfigInput{CustomRulesText: "[Rule]\nFINAL,DIRECT"}); err == nil {
		t.Fatal("UpdateGlobalRuleConfig accepted [Rule] in custom rules")
	}
}

func TestUpdateRejectsTooSmallManagedConfigInterval(t *testing.T) {
	db, err := storage.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	userID := seedUser(t, db)
	service := NewService(db)
	task, err := service.Create(userID, CreateInput{Name: "Main", SourceURL: "https://example.com/a.yaml", RefreshIntervalSeconds: 3600})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	if _, err := service.Update(userID, task.ID, UpdateInput{ManagedConfigIntervalSeconds: intPtr(59)}); err == nil {
		t.Fatal("Update accepted managed config interval below 60")
	}
}

func TestDeleteTaskRemovesTaskAndToken(t *testing.T) {
	db, err := storage.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	userID := seedUser(t, db)
	service := NewService(db)
	task, err := service.Create(userID, CreateInput{Name: "Main", SourceURL: "https://example.com/a.yaml", RefreshIntervalSeconds: 3600})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	if err := service.Delete(userID, task.ID); err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}
	tasks, err := service.List(userID)
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(tasks) != 0 {
		t.Fatalf("unexpected tasks after delete: %#v", tasks)
	}
	var tokens int
	if err := db.SQL().QueryRow(`SELECT COUNT(*) FROM subscription_tokens WHERE task_id = ?`, task.ID).Scan(&tokens); err != nil {
		t.Fatal(err)
	}
	if tokens != 0 {
		t.Fatalf("tokens = %d, want 0", tokens)
	}
}

func stringPtr(value string) *string {
	return &value
}

func intPtr(value int) *int {
	return &value
}

func seedUser(t *testing.T, db *storage.DB) int64 {
	t.Helper()
	res, err := db.SQL().Exec(`INSERT INTO users (username, password_hash) VALUES ('admin', 'hash')`)
	if err != nil {
		t.Fatal(err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	return id
}
