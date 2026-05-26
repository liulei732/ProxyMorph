package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequireAuthRejectsMissingCookie(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/tasks", nil)
	res := httptest.NewRecorder()
	RequireAuth(func(token string) (int64, bool) { return 0, false }, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called")
	})).ServeHTTP(res, req)
	if res.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", res.Code)
	}
}

func TestRequireAuthAllowsValidCookie(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/tasks", nil)
	req.AddCookie(&http.Cookie{Name: "proxymorph_session", Value: "valid"})
	res := httptest.NewRecorder()
	called := false
	RequireAuth(func(token string) (int64, bool) { return 42, token == "valid" }, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		if UserID(r.Context()) != 42 {
			t.Fatalf("user id = %d, want 42", UserID(r.Context()))
		}
	})).ServeHTTP(res, req)
	if !called {
		t.Fatal("handler was not called")
	}
}
