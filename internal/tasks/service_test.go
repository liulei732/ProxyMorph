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
	if task.VLESSRelayMode != "global" {
		t.Fatalf("VLESSRelayMode = %q, want global", task.VLESSRelayMode)
	}
	if task.TrojanWSRelayMode != "global" {
		t.Fatalf("TrojanWSRelayMode = %q, want global", task.TrojanWSRelayMode)
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

func TestCreateTaskAcceptsVLESSRelayMode(t *testing.T) {
	db, err := storage.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	userID := seedUser(t, db)
	service := NewService(db)

	task, err := service.Create(userID, CreateInput{
		Name:                   "Main",
		SourceURL:              "https://example.com/clash.yaml",
		RefreshIntervalSeconds: 3600,
		VLESSRelayMode:         "disabled",
	})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if task.VLESSRelayMode != "disabled" {
		t.Fatalf("VLESSRelayMode = %q, want disabled", task.VLESSRelayMode)
	}
}

func TestCreateTaskAcceptsTrojanWSRelayMode(t *testing.T) {
	db, err := storage.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	userID := seedUser(t, db)
	service := NewService(db)

	task, err := service.Create(userID, CreateInput{
		Name:                   "Main",
		SourceURL:              "https://example.com/clash.yaml",
		RefreshIntervalSeconds: 3600,
		TrojanWSRelayMode:      "enabled",
	})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if task.TrojanWSRelayMode != "enabled" {
		t.Fatalf("TrojanWSRelayMode = %q, want enabled", task.TrojanWSRelayMode)
	}
}

func TestCreateTaskAcceptsSourceUserAgent(t *testing.T) {
	db, err := storage.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	userID := seedUser(t, db)
	service := NewService(db)

	task, err := service.Create(userID, CreateInput{
		Name:                   "Main",
		SourceURL:              "https://example.com/clash.yaml",
		SourceUserAgent:        "Surge iOS/2999",
		RefreshIntervalSeconds: 3600,
	})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if task.SourceUserAgent != "Surge iOS/2999" {
		t.Fatalf("SourceUserAgent = %q, want custom UA", task.SourceUserAgent)
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
		SourceUserAgent:         stringPtr("Surge iOS/2999"),
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
	if next.SourceUserAgent != "Surge iOS/2999" {
		t.Fatalf("SourceUserAgent = %q, want custom UA", next.SourceUserAgent)
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
	next, err := service.Update(userID, task.ID, UpdateInput{
		IncludeGlobalRules:           &includeGlobal,
		CustomRulesText:              stringPtr("DOMAIN,task.example,DIRECT"),
		RuleMergeMode:                stringPtr("upstream_first_dedupe"),
		CustomGroupsText:             stringPtr("Manual = select, Proxy, DIRECT"),
		VLESSRelayMode:               stringPtr("enabled"),
		TrojanWSRelayMode:            stringPtr("disabled"),
		ManagedConfigMode:            stringPtr("enabled"),
		ManagedConfigURLMode:         stringPtr("task_subscription"),
		ManagedConfigIntervalMode:    stringPtr("custom"),
		ManagedConfigIntervalSeconds: intPtr(7200),
		ManagedConfigStrictMode:      stringPtr("enabled"),
	})
	if err != nil {
		t.Fatalf("Update returned error: %v", err)
	}
	if next.IncludeGlobalRules || next.CustomRulesText != "DOMAIN,task.example,DIRECT" || next.RuleMergeMode != "upstream_first_dedupe" {
		t.Fatalf("unexpected rule config: %#v", next)
	}
	if next.CustomGroupsText != "Manual = select, Proxy, DIRECT" || next.ManagedConfigMode != "enabled" || next.ManagedConfigURLMode != "task_subscription" {
		t.Fatalf("unexpected group/managed config: %#v", next)
	}
	if next.ManagedConfigIntervalMode != "custom" || next.ManagedConfigIntervalSeconds != 7200 || next.ManagedConfigStrictMode != "enabled" {
		t.Fatalf("unexpected group/managed config: %#v", next)
	}
	if next.VLESSRelayMode != "enabled" {
		t.Fatalf("VLESSRelayMode = %q, want enabled", next.VLESSRelayMode)
	}
	if next.TrojanWSRelayMode != "disabled" {
		t.Fatalf("TrojanWSRelayMode = %q, want disabled", next.TrojanWSRelayMode)
	}
}

func TestManagedConfigTaskFieldsDefaultAndUpdate(t *testing.T) {
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
	if task.ManagedConfigMode != "global" || task.ManagedConfigURLMode != "global" || task.ManagedConfigIntervalMode != "global" || task.ManagedConfigStrictMode != "global" {
		t.Fatalf("unexpected managed config defaults: %#v", task)
	}

	next, err := service.Update(userID, task.ID, UpdateInput{
		ManagedConfigMode:            stringPtr("enabled"),
		ManagedConfigURLMode:         stringPtr("custom"),
		ManagedConfigCustomURL:       stringPtr("https://profiles.example.com/main.conf"),
		ManagedConfigIntervalMode:    stringPtr("custom"),
		ManagedConfigIntervalSeconds: intPtr(7200),
		ManagedConfigStrictMode:      stringPtr("enabled"),
	})
	if err != nil {
		t.Fatalf("Update returned error: %v", err)
	}
	if next.ManagedConfigMode != "enabled" || next.ManagedConfigURLMode != "custom" || next.ManagedConfigCustomURL != "https://profiles.example.com/main.conf" {
		t.Fatalf("unexpected managed URL config: %#v", next)
	}
	if next.ManagedConfigIntervalMode != "custom" || next.ManagedConfigIntervalSeconds != 7200 || next.ManagedConfigStrictMode != "enabled" {
		t.Fatalf("unexpected managed interval/strict config: %#v", next)
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
	if initial.CustomRulesText != "" || initial.VLESSRelayEnabled || initial.TrojanWSRelayEnabled {
		t.Fatalf("unexpected initial config: %#v", initial)
	}
	if initial.SubscriptionInfoKeywordsText == "" {
		t.Fatalf("expected default subscription info keywords: %#v", initial)
	}
	updated, err := service.UpdateGlobalRuleConfig(GlobalRuleConfigInput{
		CustomRulesText:              "DOMAIN,global.example,DIRECT",
		VLESSRelayEnabled:            true,
		TrojanWSRelayEnabled:         true,
		SubscriptionInfoKeywordsText: "余额\n重置时间",
	})
	if err != nil {
		t.Fatalf("UpdateGlobalRuleConfig returned error: %v", err)
	}
	if updated.CustomRulesText != "DOMAIN,global.example,DIRECT" || !updated.VLESSRelayEnabled || !updated.TrojanWSRelayEnabled || updated.SubscriptionInfoKeywordsText != "余额\n重置时间" {
		t.Fatalf("unexpected updated config: %#v", updated)
	}
	enabled, err := service.VLESSRelayEnabled()
	if err != nil {
		t.Fatalf("VLESSRelayEnabled returned error: %v", err)
	}
	if !enabled {
		t.Fatal("VLESSRelayEnabled = false, want true")
	}
	trojanWSEnabled, err := service.TrojanWSRelayEnabled()
	if err != nil {
		t.Fatalf("TrojanWSRelayEnabled returned error: %v", err)
	}
	if !trojanWSEnabled {
		t.Fatal("TrojanWSRelayEnabled = false, want true")
	}
}

func TestCachedOutputRequiresOwnerAndReturnsCachedContent(t *testing.T) {
	db, err := storage.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	userID := seedUser(t, db)
	otherUserID := seedUserWithName(t, db, "other")
	service := NewService(db)
	task, err := service.Create(userID, CreateInput{Name: "Main", SourceURL: "https://example.com/a.yaml", RefreshIntervalSeconds: 3600})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if _, err := service.CachedOutput(userID, task.ID); err == nil {
		t.Fatal("CachedOutput returned content before cache exists")
	}
	if _, err := db.SQL().Exec(`INSERT INTO output_cache (task_id, content) VALUES (?, ?)`, task.ID, "[Proxy]\nA = direct"); err != nil {
		t.Fatalf("insert cache: %v", err)
	}
	content, err := service.CachedOutput(userID, task.ID)
	if err != nil {
		t.Fatalf("CachedOutput returned error: %v", err)
	}
	if content != "[Proxy]\nA = direct" {
		t.Fatalf("content = %q", content)
	}
	if _, err := service.CachedOutput(otherUserID, task.ID); err == nil {
		t.Fatal("CachedOutput allowed another user to read task cache")
	}
}

func TestManagedConfigDefaultsRoundTrip(t *testing.T) {
	db, err := storage.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	service := NewService(db)

	initial, err := service.ManagedConfigDefaults()
	if err != nil {
		t.Fatalf("ManagedConfigDefaults returned error: %v", err)
	}
	if initial.Enabled || initial.URLMode != "task_subscription" || initial.CustomURL != "" || initial.IntervalSeconds != 86400 || initial.Strict {
		t.Fatalf("unexpected initial defaults: %#v", initial)
	}

	updated, err := service.UpdateManagedConfigDefaults(ManagedConfigDefaultsInput{
		Enabled:         true,
		URLMode:         "custom",
		CustomURL:       "https://profiles.example.com/default.conf",
		IntervalSeconds: 3600,
		Strict:          true,
	})
	if err != nil {
		t.Fatalf("UpdateManagedConfigDefaults returned error: %v", err)
	}
	if !updated.Enabled || updated.URLMode != "custom" || updated.CustomURL != "https://profiles.example.com/default.conf" || updated.IntervalSeconds != 3600 || !updated.Strict {
		t.Fatalf("unexpected updated defaults: %#v", updated)
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
	return seedUserWithName(t, db, "admin")
}

func seedUserWithName(t *testing.T, db *storage.DB, username string) int64 {
	t.Helper()
	res, err := db.SQL().Exec(`INSERT INTO users (username, password_hash) VALUES (?, 'hash')`, username)
	if err != nil {
		t.Fatal(err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	return id
}
