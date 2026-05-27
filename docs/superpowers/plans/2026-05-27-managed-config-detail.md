# Detailed MANAGED-CONFIG Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the current simple MANAGED-CONFIG task toggle with global defaults, task-level overrides, custom URL handling, interval presets, strict overrides, and header preview.

**Architecture:** Store global MANAGED-CONFIG defaults in `app_settings` and task overrides in `conversion_tasks`. Resolve an effective managed config inside the subscription service, then render the same Surge-compatible `#!MANAGED-CONFIG <url> interval=<seconds> strict=<bool>` header. The React UI fetches global defaults alongside rule config and exposes both global and task controls with a read-only effective header preview.

**Tech Stack:** Go, SQLite, React, Vite, embedded frontend assets, Surge 6 MANAGED-CONFIG syntax.

---

## File Structure

- `internal/storage/migrate.go`: replace old task managed columns with new mode/override columns for new databases and add migration columns for existing databases.
- `internal/storage/models.go`: replace `ManagedConfigEnabled`, `ManagedConfigIntervalSeconds`, and `ManagedConfigStrict` with the new task override fields plus a `ManagedConfigDefaults` model.
- `internal/tasks/service.go`: add managed config default service methods, normalize enum values, validate custom URLs and intervals, and include task override fields in create/update/list/get.
- `internal/tasks/handlers.go`: add `ManagedConfigDefaults` handler and extend task update JSON.
- `internal/app/routes.go`: register `/api/managed-config-defaults`.
- `internal/subscription/service.go`: resolve effective MANAGED-CONFIG and generate the final header.
- `web/src/main.tsx`: add types, fetch/save global defaults, task edit controls, interval presets, and header preview.
- `web/src/i18n.ts`: add Chinese and English labels/help text.
- `internal/web/dist/`: refresh embedded frontend assets after `npm run build`.

## Task 1: Storage Models And Task Fields

**Files:**
- Modify: `internal/storage/migrate.go`
- Modify: `internal/storage/models.go`
- Modify: `internal/tasks/service.go`
- Test: `internal/tasks/service_test.go`

- [x] **Step 1: Write failing task default and update tests**

Add this test to `internal/tasks/service_test.go`:

```go
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
```

Run:

```bash
env GOCACHE=/Users/liulei/Codes/ProxyMorph/.cache/go-build GOMODCACHE=/Users/liulei/Codes/ProxyMorph/.cache/go-mod GOPROXY=https://goproxy.cn,direct /opt/homebrew/bin/go test ./internal/tasks
```

Expected: fail because the new fields and update inputs do not exist yet.

- [x] **Step 2: Replace storage model fields**

In `internal/storage/models.go`, replace:

```go
ManagedConfigEnabled         bool
ManagedConfigIntervalSeconds int
ManagedConfigStrict          bool
```

with:

```go
ManagedConfigMode            string
ManagedConfigURLMode         string
ManagedConfigCustomURL       string
ManagedConfigIntervalMode    string
ManagedConfigIntervalSeconds int
ManagedConfigStrictMode      string
```

Add:

```go
type ManagedConfigDefaults struct {
	Enabled                 bool
	URLMode                 string
	CustomURL               string
	IntervalSeconds         int
	Strict                  bool
}
```

- [x] **Step 3: Update database schema**

In `internal/storage/migrate.go`, replace the managed config columns in the `CREATE TABLE IF NOT EXISTS conversion_tasks` statement:

```sql
managed_config_enabled INTEGER NOT NULL DEFAULT 0,
managed_config_interval_seconds INTEGER NOT NULL DEFAULT 86400,
managed_config_strict INTEGER NOT NULL DEFAULT 0,
```

with:

```sql
managed_config_mode TEXT NOT NULL DEFAULT 'global',
managed_config_url_mode TEXT NOT NULL DEFAULT 'global',
managed_config_custom_url TEXT NOT NULL DEFAULT '',
managed_config_interval_mode TEXT NOT NULL DEFAULT 'global',
managed_config_interval_seconds INTEGER NOT NULL DEFAULT 86400,
managed_config_strict_mode TEXT NOT NULL DEFAULT 'global',
```

In the migration `ALTER TABLE` list, add:

```go
`ALTER TABLE conversion_tasks ADD COLUMN managed_config_mode TEXT NOT NULL DEFAULT 'global'`,
`ALTER TABLE conversion_tasks ADD COLUMN managed_config_url_mode TEXT NOT NULL DEFAULT 'global'`,
`ALTER TABLE conversion_tasks ADD COLUMN managed_config_custom_url TEXT NOT NULL DEFAULT ''`,
`ALTER TABLE conversion_tasks ADD COLUMN managed_config_interval_mode TEXT NOT NULL DEFAULT 'global'`,
`ALTER TABLE conversion_tasks ADD COLUMN managed_config_strict_mode TEXT NOT NULL DEFAULT 'global'`,
```

Keep the existing `managed_config_interval_seconds` migration so existing databases still get the interval column.

- [x] **Step 4: Update task service inputs and scan paths**

In `internal/tasks/service.go`, replace the old update input fields:

```go
ManagedConfigEnabled         *bool   `json:"managed_config_enabled"`
ManagedConfigIntervalSeconds *int    `json:"managed_config_interval_seconds"`
ManagedConfigStrict          *bool   `json:"managed_config_strict"`
```

with:

```go
ManagedConfigMode            *string `json:"managed_config_mode"`
ManagedConfigURLMode         *string `json:"managed_config_url_mode"`
ManagedConfigCustomURL       *string `json:"managed_config_custom_url"`
ManagedConfigIntervalMode    *string `json:"managed_config_interval_mode"`
ManagedConfigIntervalSeconds *int    `json:"managed_config_interval_seconds"`
ManagedConfigStrictMode      *string `json:"managed_config_strict_mode"`
```

Update `Create`, `Update`, `List`, `get`, and `scanTask` SQL column lists to use:

```sql
managed_config_mode, managed_config_url_mode, managed_config_custom_url,
managed_config_interval_mode, managed_config_interval_seconds, managed_config_strict_mode
```

Normalize scanned values with helper functions:

```go
func normalizeTriStateMode(mode string) string {
	switch mode {
	case "enabled", "disabled":
		return mode
	default:
		return "global"
	}
}

func normalizeTaskManagedURLMode(mode string) string {
	switch mode {
	case "task_subscription", "custom":
		return mode
	default:
		return "global"
	}
}

func normalizeGlobalManagedURLMode(mode string) string {
	if mode == "custom" {
		return mode
	}
	return "task_subscription"
}

func normalizeIntervalMode(mode string) string {
	if mode == "custom" {
		return mode
	}
	return "global"
}
```

- [x] **Step 5: Validate task managed config fields**

Extend `validateTaskUpdate`:

```go
if input.ManagedConfigIntervalSeconds != nil && *input.ManagedConfigIntervalSeconds < 60 {
	return fmt.Errorf("managed config interval must be at least 60 seconds")
}
if input.ManagedConfigCustomURL != nil {
	if err := validateHTTPURL(*input.ManagedConfigCustomURL, false); err != nil {
		return err
	}
}
```

Add:

```go
func validateHTTPURL(value string, required bool) error {
	value = strings.TrimSpace(value)
	if value == "" {
		if required {
			return fmt.Errorf("managed config custom url is required")
		}
		return nil
	}
	parsed, err := url.Parse(value)
	if err != nil || !parsed.IsAbs() || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return fmt.Errorf("managed config custom url must be an absolute http or https url")
	}
	return nil
}
```

Add `net/url` if it is not already imported.

- [x] **Step 6: Run task tests**

Run:

```bash
env GOCACHE=/Users/liulei/Codes/ProxyMorph/.cache/go-build GOMODCACHE=/Users/liulei/Codes/ProxyMorph/.cache/go-mod GOPROXY=https://goproxy.cn,direct /opt/homebrew/bin/go test ./internal/tasks ./internal/storage
```

Expected: pass.

## Task 2: Global MANAGED-CONFIG Defaults API

**Files:**
- Modify: `internal/tasks/service.go`
- Modify: `internal/tasks/handlers.go`
- Modify: `internal/app/routes.go`
- Test: `internal/tasks/service_test.go`
- Test: `internal/app/app_test.go`

- [x] **Step 1: Write failing service round-trip test**

Add this test to `internal/tasks/service_test.go`:

```go
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
```

Run:

```bash
env GOCACHE=/Users/liulei/Codes/ProxyMorph/.cache/go-build GOMODCACHE=/Users/liulei/Codes/ProxyMorph/.cache/go-mod GOPROXY=https://goproxy.cn,direct /opt/homebrew/bin/go test ./internal/tasks
```

Expected: fail because the default service methods do not exist.

- [x] **Step 2: Implement defaults service methods**

In `internal/tasks/service.go`, add:

```go
type ManagedConfigDefaultsInput struct {
	Enabled         bool   `json:"enabled"`
	URLMode         string `json:"url_mode"`
	CustomURL       string `json:"custom_url"`
	IntervalSeconds int    `json:"interval_seconds"`
	Strict          bool   `json:"strict"`
}
```

Add:

```go
func (s *Service) ManagedConfigDefaults() (storage.ManagedConfigDefaults, error) {
	values, err := s.settings(
		"managed_config_enabled",
		"managed_config_url_mode",
		"managed_config_custom_url",
		"managed_config_interval_seconds",
		"managed_config_strict",
	)
	if err != nil {
		return storage.ManagedConfigDefaults{}, err
	}
	interval := settingInt(values["managed_config_interval_seconds"], 86400)
	if interval < 60 {
		interval = 86400
	}
	return storage.ManagedConfigDefaults{
		Enabled:         settingBool(values["managed_config_enabled"]),
		URLMode:         normalizeGlobalManagedURLMode(values["managed_config_url_mode"]),
		CustomURL:       strings.TrimSpace(values["managed_config_custom_url"]),
		IntervalSeconds: interval,
		Strict:          settingBool(values["managed_config_strict"]),
	}, nil
}
```

Add:

```go
func (s *Service) UpdateManagedConfigDefaults(input ManagedConfigDefaultsInput) (storage.ManagedConfigDefaults, error) {
	input.URLMode = normalizeGlobalManagedURLMode(input.URLMode)
	if input.IntervalSeconds == 0 {
		input.IntervalSeconds = 86400
	}
	if input.IntervalSeconds < 60 {
		return storage.ManagedConfigDefaults{}, fmt.Errorf("managed config interval must be at least 60 seconds")
	}
	if input.URLMode == "custom" {
		if err := validateHTTPURL(input.CustomURL, input.Enabled); err != nil {
			return storage.ManagedConfigDefaults{}, err
		}
	}
	if err := s.setSetting("managed_config_enabled", boolSetting(input.Enabled)); err != nil {
		return storage.ManagedConfigDefaults{}, err
	}
	if err := s.setSetting("managed_config_url_mode", input.URLMode); err != nil {
		return storage.ManagedConfigDefaults{}, err
	}
	if err := s.setSetting("managed_config_custom_url", strings.TrimSpace(input.CustomURL)); err != nil {
		return storage.ManagedConfigDefaults{}, err
	}
	if err := s.setSetting("managed_config_interval_seconds", fmt.Sprintf("%d", input.IntervalSeconds)); err != nil {
		return storage.ManagedConfigDefaults{}, err
	}
	if err := s.setSetting("managed_config_strict", boolSetting(input.Strict)); err != nil {
		return storage.ManagedConfigDefaults{}, err
	}
	return s.ManagedConfigDefaults()
}
```

Add helper:

```go
func settingInt(value string, fallback int) int {
	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return fallback
	}
	return parsed
}
```

- [x] **Step 3: Write failing API round-trip test**

Add this test to `internal/app/app_test.go`:

```go
func TestManagedConfigDefaultsEndpointRoundTrip(t *testing.T) {
	cfg := config.Config{Addr: ":0", DataDir: t.TempDir(), SessionSecret: "test-secret", InitialAdminUsername: "admin", InitialAdminPassword: "password"}
	app, err := New(cfg, filepath.Join(cfg.DataDir, "test.db"))
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	defer app.Close()

	loginReq := httptest.NewRequest(http.MethodPost, "/api/login", bytes.NewBufferString(`{"username":"admin","password":"password"}`))
	loginRes := httptest.NewRecorder()
	app.Handler().ServeHTTP(loginRes, loginReq)
	if loginRes.Code != http.StatusOK {
		t.Fatalf("login status = %d body=%q", loginRes.Code, loginRes.Body.String())
	}
	cookie := loginRes.Result().Cookies()[0]

	body := `{"enabled":true,"url_mode":"custom","custom_url":"https://profiles.example.com/default.conf","interval_seconds":3600,"strict":true}`
	putReq := httptest.NewRequest(http.MethodPut, "/api/managed-config-defaults", bytes.NewBufferString(body))
	putReq.AddCookie(cookie)
	putRes := httptest.NewRecorder()
	app.Handler().ServeHTTP(putRes, putReq)
	if putRes.Code != http.StatusOK {
		t.Fatalf("put status = %d body=%q", putRes.Code, putRes.Body.String())
	}

	getReq := httptest.NewRequest(http.MethodGet, "/api/managed-config-defaults", nil)
	getReq.AddCookie(cookie)
	getRes := httptest.NewRecorder()
	app.Handler().ServeHTTP(getRes, getReq)
	if getRes.Code != http.StatusOK {
		t.Fatalf("get status = %d body=%q", getRes.Code, getRes.Body.String())
	}
	var payload struct {
		Enabled         bool
		URLMode         string
		CustomURL       string
		IntervalSeconds int
		Strict          bool
	}
	if err := json.NewDecoder(getRes.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !payload.Enabled || payload.URLMode != "custom" || payload.CustomURL != "https://profiles.example.com/default.conf" || payload.IntervalSeconds != 3600 || !payload.Strict {
		t.Fatalf("unexpected defaults: %#v", payload)
	}
}
```

Run:

```bash
env GOCACHE=/Users/liulei/Codes/ProxyMorph/.cache/go-build GOMODCACHE=/Users/liulei/Codes/ProxyMorph/.cache/go-mod GOPROXY=https://goproxy.cn,direct /opt/homebrew/bin/go test ./internal/app
```

Expected: fail because the route does not exist.

- [x] **Step 4: Add handler and route**

In `internal/tasks/handlers.go`, add:

```go
func (h Handler) ManagedConfigDefaults(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		config, err := h.Service.ManagedConfigDefaults()
		if err != nil {
			httpapi.Error(w, http.StatusInternalServerError, err.Error())
			return
		}
		httpapi.JSON(w, http.StatusOK, config)
	case http.MethodPut:
		var input ManagedConfigDefaultsInput
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			httpapi.Error(w, http.StatusBadRequest, "invalid json")
			return
		}
		config, err := h.Service.UpdateManagedConfigDefaults(input)
		if err != nil {
			httpapi.Error(w, http.StatusBadRequest, err.Error())
			return
		}
		httpapi.JSON(w, http.StatusOK, config)
	default:
		httpapi.Error(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}
```

In `internal/app/routes.go`, add:

```go
a.mux.Handle("/api/managed-config-defaults", httpapi.RequireAuth(a.authSvc.VerifyToken, http.HandlerFunc(taskHandler.ManagedConfigDefaults)))
```

- [x] **Step 5: Run API tests**

Run:

```bash
env GOCACHE=/Users/liulei/Codes/ProxyMorph/.cache/go-build GOMODCACHE=/Users/liulei/Codes/ProxyMorph/.cache/go-mod GOPROXY=https://goproxy.cn,direct /opt/homebrew/bin/go test ./internal/tasks ./internal/app
```

Expected: pass.

## Task 3: Subscription Effective MANAGED-CONFIG Resolution

**Files:**
- Modify: `internal/subscription/service.go`
- Test: `internal/subscription/service_test.go`

- [x] **Step 1: Write failing subscription tests**

Add tests to `internal/subscription/service_test.go` that create a task with mixed global and task managed config modes, then assert the first output line.

Use this assertion helper:

```go
func firstLine(text string) string {
	idx := strings.IndexByte(text, '\n')
	if idx == -1 {
		return text
	}
	return text[:idx]
}
```

Add at least these cases:

```go
func TestManagedConfigTaskEnabledOverridesDisabledGlobal(t *testing.T) {
	service, db := newTestService(t)
	seedHTTPSubscription(t, "proxy", clashBase64("ss://YWVzLTI1Ni1nY206cGFzc0BleGFtcGxlLmNvbTo4ODg4#Node"))
	taskID := seedTask(t, db, "Main", testSubscriptionURL("proxy"))
	_, err := db.SQL().Exec(`
		UPDATE conversion_tasks
		SET managed_config_mode = 'enabled',
			managed_config_url_mode = 'task_subscription',
			managed_config_interval_mode = 'custom',
			managed_config_interval_seconds = 7200,
			managed_config_strict_mode = 'enabled'
		WHERE id = ?`, taskID)
	if err != nil {
		t.Fatal(err)
	}

	output, err := service.GenerateByTaskID(taskID)
	if err != nil {
		t.Fatalf("GenerateByTaskID returned error: %v", err)
	}
	want := "#!MANAGED-CONFIG http://localhost:8080/sub/test-token?name=Main interval=7200 strict=true"
	if firstLine(output) != want {
		t.Fatalf("first line = %q, want %q", firstLine(output), want)
	}
}
```

Add a second test where global defaults are enabled with custom URL and the task uses all `global` modes. Expected first line:

```text
#!MANAGED-CONFIG https://profiles.example.com/default.conf interval=3600 strict=false
```

Add a third test where global defaults are enabled but task `managed_config_mode = 'disabled'`; expected output must not start with `#!MANAGED-CONFIG`.

Run:

```bash
env GOCACHE=/Users/liulei/Codes/ProxyMorph/.cache/go-build GOMODCACHE=/Users/liulei/Codes/ProxyMorph/.cache/go-mod GOPROXY=https://goproxy.cn,direct /opt/homebrew/bin/go test ./internal/subscription
```

Expected: fail until subscription resolution is implemented.

- [x] **Step 2: Implement effective config resolver**

In `internal/subscription/service.go`, add:

```go
type effectiveManagedConfig struct {
	Enabled         bool
	URL             string
	IntervalSeconds int
	Strict          bool
}
```

Add:

```go
func (s *Service) effectiveManagedConfig(task storage.ConversionTask, relayHost string) (effectiveManagedConfig, error) {
	defaults, err := s.tasks.ManagedConfigDefaults()
	if err != nil {
		return effectiveManagedConfig{}, err
	}
	enabled := defaults.Enabled
	switch normalizeVLESSRelayMode(task.ManagedConfigMode) {
	case "enabled":
		enabled = true
	case "disabled":
		enabled = false
	}
	if !enabled {
		return effectiveManagedConfig{Enabled: false}, nil
	}

	urlMode := task.ManagedConfigURLMode
	if urlMode == "" || urlMode == "global" {
		urlMode = defaults.URLMode
	}
	managedURL := taskSubscriptionURL(task, relayHost)
	if urlMode == "custom" {
		managedURL = strings.TrimSpace(defaults.CustomURL)
		if strings.TrimSpace(task.ManagedConfigCustomURL) != "" && task.ManagedConfigURLMode == "custom" {
			managedURL = strings.TrimSpace(task.ManagedConfigCustomURL)
		}
	}
	if managedURL == "" {
		return effectiveManagedConfig{}, fmt.Errorf("managed config url is required")
	}

	interval := defaults.IntervalSeconds
	if interval == 0 {
		interval = 86400
	}
	if task.ManagedConfigIntervalMode == "custom" {
		interval = task.ManagedConfigIntervalSeconds
	}
	if interval < 60 {
		return effectiveManagedConfig{}, fmt.Errorf("managed config interval must be at least 60 seconds")
	}

	strict := defaults.Strict
	switch normalizeVLESSRelayMode(task.ManagedConfigStrictMode) {
	case "enabled":
		strict = true
	case "disabled":
		strict = false
	}
	return effectiveManagedConfig{Enabled: true, URL: managedURL, IntervalSeconds: interval, Strict: strict}, nil
}
```

Use a generic tri-state normalizer instead of `normalizeVLESSRelayMode` if Task 1 introduced one in a shared file or duplicated it locally.

- [x] **Step 3: Replace old header generation**

In `surgeConfig`, replace:

```go
if task.ManagedConfigEnabled {
	cfg.ManagedConfigHeader = managedConfigHeader(task, relayHost)
}
```

with:

```go
managed, err := s.effectiveManagedConfig(task, relayHost)
if err != nil {
	return convert.SurgeConfig{}, err
}
if managed.Enabled {
	cfg.ManagedConfigHeader = managedConfigHeader(managed)
}
```

Replace `managedConfigHeader` with:

```go
func managedConfigHeader(config effectiveManagedConfig) string {
	return fmt.Sprintf("#!MANAGED-CONFIG %s interval=%d strict=%t",
		config.URL,
		config.IntervalSeconds,
		config.Strict,
	)
}
```

- [x] **Step 4: Run subscription tests**

Run:

```bash
env GOCACHE=/Users/liulei/Codes/ProxyMorph/.cache/go-build GOMODCACHE=/Users/liulei/Codes/ProxyMorph/.cache/go-mod GOPROXY=https://goproxy.cn,direct /opt/homebrew/bin/go test ./internal/subscription
```

Expected: pass.

## Task 4: Frontend Configuration UI

**Files:**
- Modify: `web/src/main.tsx`
- Modify: `web/src/i18n.ts`
- Modify: `web/src/i18n.test.ts`
- Update generated assets under `internal/web/dist/`

- [x] **Step 1: Extend i18n tests**

In `web/src/i18n.test.ts`, add:

```ts
it("translates managed config controls", () => {
  assert.equal(translations.zh.forms.managedConfigMode, "MANAGED-CONFIG");
  assert.equal(translations.zh.managedConfigModes.global, "跟随全局");
  assert.equal(translations.en.managedConfigModes.global, "Follow global");
});
```

Run:

```bash
node --experimental-strip-types --test web/src/i18n.test.ts
```

Expected: fail because the labels do not exist.

- [x] **Step 2: Add i18n labels**

In `web/src/i18n.ts`, add form labels:

```ts
managedConfigMode: "MANAGED-CONFIG",
managedConfigURLMode: "托管 URL",
managedConfigCustomURL: "自定义托管 URL",
managedConfigIntervalMode: "刷新间隔",
managedConfigIntervalCustom: "自定义秒数",
managedConfigStrictMode: "严格模式",
managedConfigPreview: "托管头部预览",
managedConfigDefaults: "MANAGED-CONFIG 默认值",
```

Add English equivalents:

```ts
managedConfigMode: "MANAGED-CONFIG",
managedConfigURLMode: "Managed URL",
managedConfigCustomURL: "Custom managed URL",
managedConfigIntervalMode: "Refresh interval",
managedConfigIntervalCustom: "Custom seconds",
managedConfigStrictMode: "Strict mode",
managedConfigPreview: "Managed header preview",
managedConfigDefaults: "MANAGED-CONFIG Defaults",
```

Add option groups:

```ts
managedConfigModes: {
  global: "跟随全局",
  enabled: "启用",
  disabled: "禁用",
},
managedConfigURLModes: {
  global: "跟随全局",
  task_subscription: "任务订阅地址",
  custom: "自定义 URL",
},
managedConfigIntervalModes: {
  global: "跟随全局",
  custom: "自定义",
},
managedConfigStrictModes: {
  global: "跟随全局",
  enabled: "启用",
  disabled: "禁用",
},
managedConfigIntervals: {
  "3600": "1 小时",
  "21600": "6 小时",
  "43200": "12 小时",
  "86400": "24 小时",
  custom: "自定义",
},
```

Add matching English option groups.

- [x] **Step 3: Extend frontend data types and refresh**

In `web/src/main.tsx`, add types:

```ts
type TriStateMode = "global" | "enabled" | "disabled";
type ManagedURLMode = "global" | "task_subscription" | "custom";
type ManagedIntervalMode = "global" | "custom";
type GlobalManagedURLMode = "task_subscription" | "custom";
```

Replace task managed fields:

```ts
ManagedConfigMode: TriStateMode;
ManagedConfigURLMode: ManagedURLMode;
ManagedConfigCustomURL: string;
ManagedConfigIntervalMode: ManagedIntervalMode;
ManagedConfigIntervalSeconds: number;
ManagedConfigStrictMode: TriStateMode;
```

Add:

```ts
type ManagedConfigDefaults = {
  Enabled: boolean;
  URLMode: GlobalManagedURLMode;
  CustomURL: string;
  IntervalSeconds: number;
  Strict: boolean;
};
```

Add state:

```ts
const [managedConfigDefaults, setManagedConfigDefaults] = React.useState<ManagedConfigDefaults>({
  Enabled: false,
  URLMode: "task_subscription",
  CustomURL: "",
  IntervalSeconds: 86400,
  Strict: false,
});
```

Fetch it in `refresh` with:

```ts
api<ManagedConfigDefaults>("/api/managed-config-defaults").catch(() => ({
  Enabled: false,
  URLMode: "task_subscription",
  CustomURL: "",
  IntervalSeconds: 86400,
  Strict: false,
})),
```

- [x] **Step 4: Add global defaults form**

In the Settings tab, add a new `panel settings-form` for global MANAGED-CONFIG defaults:

```tsx
<form className="panel settings-form" onSubmit={updateManagedConfigDefaults}>
  <h3>{t.forms.managedConfigDefaults}</h3>
  <label className="check-label"><input name="enabled" type="checkbox" defaultChecked={managedConfigDefaults.Enabled} /> {t.forms.enabled}</label>
  <label>{t.forms.managedConfigURLMode}<select name="url_mode" defaultValue={managedConfigDefaults.URLMode}>{globalManagedURLModeOptions(t)}</select></label>
  <label>{t.forms.managedConfigCustomURL}<input name="custom_url" defaultValue={managedConfigDefaults.CustomURL} placeholder="https://profiles.example.com/default.conf" /></label>
  <label>{t.forms.managedConfigIntervalMode}<select name="interval_preset" defaultValue={String(managedConfigDefaults.IntervalSeconds)}>{managedIntervalPresetOptions(t)}</select></label>
  <label>{t.forms.managedConfigIntervalCustom}<input name="interval_seconds" type="number" min="60" defaultValue={managedConfigDefaults.IntervalSeconds || 86400} /></label>
  <label className="check-label"><input name="strict" type="checkbox" defaultChecked={managedConfigDefaults.Strict} /> {t.forms.managedConfigStrictMode}</label>
  <p className="muted">{t.settings.managedStrictHelp}</p>
  <button><Save size={15} /> {t.forms.save}</button>
</form>
```

Add submit handler:

```ts
async function updateManagedConfigDefaults(event: React.FormEvent<HTMLFormElement>) {
  event.preventDefault();
  const form = new FormData(event.currentTarget);
  const preset = String(form.get("interval_preset") || "86400");
  const interval = preset === "custom" ? Number(form.get("interval_seconds") || 86400) : Number(preset);
  try {
    const next = await api<ManagedConfigDefaults>("/api/managed-config-defaults", {
      method: "PUT",
      body: JSON.stringify({
        enabled: form.get("enabled") === "on",
        url_mode: String(form.get("url_mode") || "task_subscription"),
        custom_url: String(form.get("custom_url") || ""),
        interval_seconds: interval,
        strict: form.get("strict") === "on",
      }),
    });
    setManagedConfigDefaults(next);
    notify(t.settings.managedConfigSaved);
  } catch {
    notify(t.settings.managedConfigSaveFailed);
  }
}
```

- [x] **Step 5: Add task edit controls and preview**

In the task edit form, replace old managed checkbox/interval/strict inputs with:

```tsx
<label>{t.forms.managedConfigMode}<select name="managed_config_mode" defaultValue={task.ManagedConfigMode || "global"}>{triStateOptions(t.managedConfigModes)}</select></label>
<label>{t.forms.managedConfigURLMode}<select name="managed_config_url_mode" defaultValue={task.ManagedConfigURLMode || "global"}>{managedURLModeOptions(t)}</select></label>
<label>{t.forms.managedConfigCustomURL}<input name="managed_config_custom_url" defaultValue={task.ManagedConfigCustomURL} placeholder="https://profiles.example.com/main.conf" /></label>
<label>{t.forms.managedConfigIntervalMode}<select name="managed_config_interval_mode" defaultValue={task.ManagedConfigIntervalMode || "global"}>{managedIntervalModeOptions(t)}</select></label>
<label>{t.forms.managedConfigIntervalCustom}<input name="managed_config_interval_seconds" type="number" min="60" defaultValue={task.ManagedConfigIntervalSeconds || 86400} /></label>
<label>{t.forms.managedConfigStrictMode}<select name="managed_config_strict_mode" defaultValue={task.ManagedConfigStrictMode || "global"}>{triStateOptions(t.managedConfigStrictModes)}</select></label>
<label className="wide-field">{t.forms.managedConfigPreview}<pre className="inline-preview">{managedHeaderPreview(task, managedConfigDefaults)}</pre></label>
```

Update save payload:

```ts
managed_config_mode: String(form.get("managed_config_mode") || "global") as TriStateMode,
managed_config_url_mode: String(form.get("managed_config_url_mode") || "global") as ManagedURLMode,
managed_config_custom_url: String(form.get("managed_config_custom_url") || ""),
managed_config_interval_mode: String(form.get("managed_config_interval_mode") || "global") as ManagedIntervalMode,
managed_config_interval_seconds: Number(form.get("managed_config_interval_seconds") || 86400),
managed_config_strict_mode: String(form.get("managed_config_strict_mode") || "global") as TriStateMode,
```

Pass `managedConfigDefaults` into `TaskList` and `TaskRow`.

- [x] **Step 6: Implement frontend preview helpers**

Add helpers in `web/src/main.tsx`:

```ts
function managedHeaderPreview(task: Task, defaults: ManagedConfigDefaults) {
  const enabled = task.ManagedConfigMode === "enabled" || (task.ManagedConfigMode === "global" && defaults.Enabled);
  if (!enabled) return "MANAGED-CONFIG disabled";
  const urlMode = task.ManagedConfigURLMode === "global" ? defaults.URLMode : task.ManagedConfigURLMode;
  const url = urlMode === "custom" ? (task.ManagedConfigCustomURL || defaults.CustomURL) : absoluteSubscriptionURL(task.SubscriptionURL);
  const interval = task.ManagedConfigIntervalMode === "custom" ? task.ManagedConfigIntervalSeconds : defaults.IntervalSeconds;
  const strict = task.ManagedConfigStrictMode === "enabled" || (task.ManagedConfigStrictMode === "global" && defaults.Strict);
  return `#!MANAGED-CONFIG ${url} interval=${interval || 86400} strict=${strict}`;
}
```

Add option helper functions using the i18n option maps.

- [x] **Step 7: Build frontend and copy assets**

Run:

```bash
node --experimental-strip-types --test web/src/i18n.test.ts
cd web && npm run build
```

Then from repo root:

```bash
rm -rf internal/web/dist
mkdir -p internal/web/dist
cp -R web/dist/. internal/web/dist/
```

Expected: frontend tests and build pass.

## Task 5: Final Verification And Commit

**Files:**
- All changed files.

- [x] **Step 1: Format Go files**

Run:

```bash
/opt/homebrew/bin/gofmt -w internal/storage/migrate.go internal/storage/models.go internal/tasks/service.go internal/tasks/handlers.go internal/app/routes.go internal/app/app_test.go internal/tasks/service_test.go internal/subscription/service.go internal/subscription/service_test.go
```

- [x] **Step 2: Run full test suite**

Run:

```bash
node --experimental-strip-types --test web/src/i18n.test.ts
env GOCACHE=/Users/liulei/Codes/ProxyMorph/.cache/go-build GOMODCACHE=/Users/liulei/Codes/ProxyMorph/.cache/go-mod GOPROXY=https://goproxy.cn,direct /opt/homebrew/bin/go test ./...
```

Expected: all tests pass.

- [x] **Step 3: Build Docker image**

Run:

```bash
/opt/homebrew/bin/docker build -t proxymorph:dev .
```

Expected: image builds successfully and includes `sing-box` from the Dockerfile.

- [x] **Step 4: Inspect diff**

Run:

```bash
git status --short
git diff --stat
git diff --check
```

Expected: only implementation files and refreshed frontend assets are changed; `.DS_Store` remains untracked and must not be staged.

- [x] **Step 5: Commit and push**

Run:

```bash
git add internal/storage/migrate.go internal/storage/models.go internal/tasks/service.go internal/tasks/handlers.go internal/app/routes.go internal/app/app_test.go internal/tasks/service_test.go internal/subscription/service.go internal/subscription/service_test.go web/src/main.tsx web/src/i18n.ts web/src/i18n.test.ts internal/web/dist
git commit -m "feat: add detailed managed config"
git push origin main
```

Expected: commit lands on `main` and pushes successfully.

## Self-Review

- Spec coverage: the plan covers global defaults, task overrides, custom URL validation, interval presets, strict mode, preview, API, rendering, and tests.
- Placeholder scan: no placeholder or vague implementation steps remain.
- Type consistency: task fields use `ManagedConfigMode`, `ManagedConfigURLMode`, `ManagedConfigCustomURL`, `ManagedConfigIntervalMode`, `ManagedConfigIntervalSeconds`, and `ManagedConfigStrictMode` consistently across backend and frontend.
