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
