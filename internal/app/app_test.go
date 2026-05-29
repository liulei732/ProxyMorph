package app

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/liulei/proxymorph/internal/config"
)

func TestHealthEndpoint(t *testing.T) {
	cfg := config.Config{Addr: ":0", DataDir: t.TempDir(), SessionSecret: "test-secret", InitialAdminUsername: "admin", InitialAdminPassword: "password"}
	app, err := New(cfg, filepath.Join(cfg.DataDir, "test.db"))
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	defer app.Close()

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	res := httptest.NewRecorder()
	app.Handler().ServeHTTP(res, req)
	if res.Code != http.StatusOK || res.Body.String() != "ok\n" {
		t.Fatalf("unexpected response: %d %q", res.Code, res.Body.String())
	}
}

func TestMeEndpointReturnsAuthenticatedUser(t *testing.T) {
	cfg := config.Config{Addr: ":0", DataDir: t.TempDir(), SessionSecret: "test-secret", InitialAdminUsername: "admin", InitialAdminPassword: "password"}
	app, err := New(cfg, filepath.Join(cfg.DataDir, "test.db"))
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	defer app.Close()

	loginBody := bytes.NewBufferString(`{"username":"admin","password":"password"}`)
	loginReq := httptest.NewRequest(http.MethodPost, "/api/login", loginBody)
	loginRes := httptest.NewRecorder()
	app.Handler().ServeHTTP(loginRes, loginReq)
	if loginRes.Code != http.StatusOK {
		t.Fatalf("login status = %d body=%q", loginRes.Code, loginRes.Body.String())
	}
	cookies := loginRes.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("login did not set session cookie")
	}

	meReq := httptest.NewRequest(http.MethodGet, "/api/me", nil)
	meReq.AddCookie(cookies[0])
	meRes := httptest.NewRecorder()
	app.Handler().ServeHTTP(meRes, meReq)
	if meRes.Code != http.StatusOK {
		t.Fatalf("me status = %d body=%q", meRes.Code, meRes.Body.String())
	}
	var payload struct {
		Username string `json:"username"`
	}
	if err := json.NewDecoder(meRes.Body).Decode(&payload); err != nil {
		t.Fatalf("decode me response: %v", err)
	}
	if payload.Username != "admin" {
		t.Fatalf("username = %q, want admin", payload.Username)
	}
}

func TestAccountPasswordChangeAndLogoutEndpoints(t *testing.T) {
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

	changeReq := httptest.NewRequest(http.MethodPatch, "/api/me/password", bytes.NewBufferString(`{"current_password":"password","new_password":"new-password"}`))
	changeReq.AddCookie(cookie)
	changeRes := httptest.NewRecorder()
	app.Handler().ServeHTTP(changeRes, changeReq)
	if changeRes.Code != http.StatusOK {
		t.Fatalf("change password status = %d body=%q", changeRes.Code, changeRes.Body.String())
	}

	oldLoginReq := httptest.NewRequest(http.MethodPost, "/api/login", bytes.NewBufferString(`{"username":"admin","password":"password"}`))
	oldLoginRes := httptest.NewRecorder()
	app.Handler().ServeHTTP(oldLoginRes, oldLoginReq)
	if oldLoginRes.Code != http.StatusUnauthorized {
		t.Fatalf("old password login status = %d body=%q", oldLoginRes.Code, oldLoginRes.Body.String())
	}
	newLoginReq := httptest.NewRequest(http.MethodPost, "/api/login", bytes.NewBufferString(`{"username":"admin","password":"new-password"}`))
	newLoginRes := httptest.NewRecorder()
	app.Handler().ServeHTTP(newLoginRes, newLoginReq)
	if newLoginRes.Code != http.StatusOK {
		t.Fatalf("new password login status = %d body=%q", newLoginRes.Code, newLoginRes.Body.String())
	}

	logoutReq := httptest.NewRequest(http.MethodPost, "/api/logout", nil)
	logoutRes := httptest.NewRecorder()
	app.Handler().ServeHTTP(logoutRes, logoutReq)
	if logoutRes.Code != http.StatusOK {
		t.Fatalf("logout status = %d body=%q", logoutRes.Code, logoutRes.Body.String())
	}
	cookies := logoutRes.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Name != "proxymorph_session" || cookies[0].MaxAge != -1 {
		t.Fatalf("logout should clear session cookie, got %#v", cookies)
	}
}

func TestRuleConfigEndpointRoundTrip(t *testing.T) {
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

	putReq := httptest.NewRequest(http.MethodPut, "/api/rule-config", bytes.NewBufferString(`{"custom_rules_text":"DOMAIN,global.example,DIRECT","vless_relay_enabled":true,"trojan_ws_relay_enabled":true,"subscription_info_keywords_text":"余额\n重置时间"}`))
	putReq.AddCookie(cookie)
	putRes := httptest.NewRecorder()
	app.Handler().ServeHTTP(putRes, putReq)
	if putRes.Code != http.StatusOK {
		t.Fatalf("put status = %d body=%q", putRes.Code, putRes.Body.String())
	}

	getReq := httptest.NewRequest(http.MethodGet, "/api/rule-config", nil)
	getReq.AddCookie(cookie)
	getRes := httptest.NewRecorder()
	app.Handler().ServeHTTP(getRes, getReq)
	if getRes.Code != http.StatusOK {
		t.Fatalf("get status = %d body=%q", getRes.Code, getRes.Body.String())
	}
	var payload struct {
		CustomRulesText              string
		VLESSRelayEnabled            bool
		TrojanWSRelayEnabled         bool
		SubscriptionInfoKeywordsText string
	}
	if err := json.NewDecoder(getRes.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.CustomRulesText != "DOMAIN,global.example,DIRECT" || !payload.VLESSRelayEnabled || !payload.TrojanWSRelayEnabled || payload.SubscriptionInfoKeywordsText != "余额\n重置时间" {
		t.Fatalf("unexpected config: %#v", payload)
	}
}

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

func TestCachedPreviewEndpointReturnsCachedOutput(t *testing.T) {
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

	createReq := httptest.NewRequest(http.MethodPost, "/api/tasks", bytes.NewBufferString(`{"name":"Main","source_url":"https://example.com/clash.yaml","refresh_interval_seconds":3600}`))
	createReq.AddCookie(cookie)
	createRes := httptest.NewRecorder()
	app.Handler().ServeHTTP(createRes, createReq)
	if createRes.Code != http.StatusCreated {
		t.Fatalf("create status = %d body=%q", createRes.Code, createRes.Body.String())
	}
	var created struct {
		ID int64
	}
	if err := json.NewDecoder(createRes.Body).Decode(&created); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	if _, err := app.db.SQL().Exec(`INSERT INTO output_cache (task_id, content) VALUES (?, ?)`, created.ID, "[Proxy]\nA = direct"); err != nil {
		t.Fatalf("insert cache: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/tasks/"+strconv.FormatInt(created.ID, 10)+"/cached-preview", nil)
	req.AddCookie(cookie)
	res := httptest.NewRecorder()
	app.Handler().ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("cached preview status = %d body=%q", res.Code, res.Body.String())
	}
	var payload struct {
		Content string
	}
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		t.Fatalf("decode cached preview response: %v", err)
	}
	if payload.Content != "[Proxy]\nA = direct" {
		t.Fatalf("content = %q", payload.Content)
	}
}
