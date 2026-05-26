package app

import (
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
