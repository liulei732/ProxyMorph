package auth

import (
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"strings"

	"github.com/liulei/proxymorph/internal/httpapi"
)

type Handler struct {
	Service *Service
}

func (h Handler) Login(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	limitKey := loginLimitKey(r, input.Username)
	if !h.Service.AllowLoginAttempt(limitKey) {
		httpapi.Error(w, http.StatusTooManyRequests, "too many login attempts")
		return
	}
	user, err := h.Service.Authenticate(input.Username, input.Password)
	if err != nil {
		h.Service.RecordLoginFailure(limitKey)
		http.Error(w, "invalid credentials", http.StatusUnauthorized)
		return
	}
	h.Service.RecordLoginSuccess(limitKey)
	http.SetCookie(w, &http.Cookie{
		Name:     "proxymorph_session",
		Value:    h.Service.SignUserID(user.ID),
		Path:     "/",
		HttpOnly: true,
		Secure:   requestIsHTTPS(r),
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(SessionDuration.Seconds()),
	})
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"ok":true}`))
}

func (h Handler) Logout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     "proxymorph_session",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
	httpapi.JSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func loginLimitKey(r *http.Request, username string) string {
	host, _, err := net.SplitHostPort(strings.TrimSpace(r.RemoteAddr))
	if err != nil || host == "" {
		host = strings.TrimSpace(r.RemoteAddr)
	}
	return strings.ToLower(strings.TrimSpace(username)) + "|" + host
}

func requestIsHTTPS(r *http.Request) bool {
	if r.TLS != nil {
		return true
	}
	return strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
}

func (h Handler) Me(w http.ResponseWriter, r *http.Request) {
	user, err := h.Service.UserByID(r.Context(), httpapi.UserID(r.Context()))
	if err != nil {
		httpapi.Error(w, http.StatusUnauthorized, "authentication required")
		return
	}
	httpapi.JSON(w, http.StatusOK, map[string]any{
		"id":       user.ID,
		"username": user.Username,
	})
}

func (h Handler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	var input struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		httpapi.Error(w, http.StatusBadRequest, "invalid json")
		return
	}
	err := h.Service.ChangePassword(httpapi.UserID(r.Context()), input.CurrentPassword, input.NewPassword)
	if errors.Is(err, ErrInvalidCredentials) {
		httpapi.Error(w, http.StatusUnauthorized, "current password is incorrect")
		return
	}
	if errors.Is(err, ErrPasswordTooShort) {
		httpapi.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	if err != nil {
		httpapi.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	httpapi.JSON(w, http.StatusOK, map[string]bool{"ok": true})
}
