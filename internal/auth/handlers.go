package auth

import (
	"encoding/json"
	"net/http"

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
	user, err := h.Service.Authenticate(input.Username, input.Password)
	if err != nil {
		http.Error(w, "invalid credentials", http.StatusUnauthorized)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "proxymorph_session",
		Value:    h.Service.SignUserID(user.ID),
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"ok":true}`))
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
