package auth

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/liulei/proxymorph/internal/httpapi"
	"github.com/liulei/proxymorph/internal/storage"
)

func TestHandlerLogoutClearsSessionCookie(t *testing.T) {
	service := NewService(nil, []byte("test-secret"))
	req := httptest.NewRequest(http.MethodPost, "/api/logout", nil)
	res := httptest.NewRecorder()

	Handler{Service: service}.Logout(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", res.Code, res.Body.String())
	}
	cookies := res.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Name != "proxymorph_session" || cookies[0].MaxAge != -1 {
		t.Fatalf("expected expired session cookie, got %#v", cookies)
	}
}

func TestHandlerLoginSetsExpiringSecureCookieForHTTPS(t *testing.T) {
	db, err := storage.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	service := NewService(db, []byte("test-secret"))
	if err := service.EnsureAdmin("admin", "password"); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "https://proxymorph.example/api/login", strings.NewReader(`{"username":"admin","password":"password"}`))
	res := httptest.NewRecorder()

	Handler{Service: service}.Login(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", res.Code, res.Body.String())
	}
	cookies := res.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("expected one cookie, got %#v", cookies)
	}
	cookie := cookies[0]
	if cookie.Name != "proxymorph_session" || !cookie.HttpOnly || !cookie.Secure || cookie.MaxAge <= 0 {
		t.Fatalf("session cookie missing security attributes: %#v", cookie)
	}
}

func TestHandlerLoginRateLimitsRepeatedFailures(t *testing.T) {
	db, err := storage.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	service := NewService(db, []byte("test-secret"))
	if err := service.EnsureAdmin("admin", "password"); err != nil {
		t.Fatal(err)
	}
	handler := Handler{Service: service}
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest(http.MethodPost, "/api/login", strings.NewReader(`{"username":"admin","password":"wrong-password"}`))
		req.RemoteAddr = "203.0.113.10:12345"
		res := httptest.NewRecorder()
		handler.Login(res, req)
		if res.Code != http.StatusUnauthorized {
			t.Fatalf("failure %d status = %d body=%s", i+1, res.Code, res.Body.String())
		}
	}
	req := httptest.NewRequest(http.MethodPost, "/api/login", strings.NewReader(`{"username":"admin","password":"password"}`))
	req.RemoteAddr = "203.0.113.10:12345"
	res := httptest.NewRecorder()
	handler.Login(res, req)
	if res.Code != http.StatusTooManyRequests {
		t.Fatalf("rate limited login status = %d body=%s", res.Code, res.Body.String())
	}
}

func TestHandlerChangePasswordUsesAuthenticatedUser(t *testing.T) {
	db, err := storage.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	service := NewService(db, []byte("test-secret"))
	if err := service.EnsureAdmin("admin", "old-password"); err != nil {
		t.Fatal(err)
	}
	user, err := service.Authenticate("admin", "old-password")
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPatch, "/api/me/password", strings.NewReader(`{"current_password":"old-password","new_password":"new-password"}`))
	req.AddCookie(&http.Cookie{Name: "proxymorph_session", Value: "valid"})
	res := httptest.NewRecorder()

	httpapi.RequireAuth(func(token string) (int64, bool) {
		return user.ID, token == "valid"
	}, http.HandlerFunc(Handler{Service: service}.ChangePassword)).ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", res.Code, res.Body.String())
	}
	if _, err := service.Authenticate("admin", "new-password"); err != nil {
		t.Fatalf("Authenticate with new password returned error: %v", err)
	}
}

func TestHandlerChangePasswordReportsCurrentPasswordErrors(t *testing.T) {
	db, err := storage.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	service := NewService(db, []byte("test-secret"))
	if err := service.EnsureAdmin("admin", "old-password"); err != nil {
		t.Fatal(err)
	}
	user, err := service.Authenticate("admin", "old-password")
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPatch, "/api/me/password", strings.NewReader(`{"current_password":"wrong-password","new_password":"new-password"}`))
	req.AddCookie(&http.Cookie{Name: "proxymorph_session", Value: "valid"})
	res := httptest.NewRecorder()

	httpapi.RequireAuth(func(token string) (int64, bool) {
		return user.ID, token == "valid"
	}, http.HandlerFunc(Handler{Service: service}.ChangePassword)).ServeHTTP(res, req)

	if res.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d body=%s", res.Code, res.Body.String())
	}
}
