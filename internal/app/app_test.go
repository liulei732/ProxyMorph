package app

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
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

	putReq := httptest.NewRequest(http.MethodPut, "/api/rule-config", bytes.NewBufferString(`{"custom_rules_text":"DOMAIN,global.example,DIRECT","rule_merge_mode":"upstream_first_dedupe"}`))
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
		CustomRulesText string
		RuleMergeMode   string
	}
	if err := json.NewDecoder(getRes.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.CustomRulesText != "DOMAIN,global.example,DIRECT" || payload.RuleMergeMode != "upstream_first_dedupe" {
		t.Fatalf("unexpected config: %#v", payload)
	}
}
