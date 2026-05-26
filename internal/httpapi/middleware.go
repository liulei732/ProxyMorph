package httpapi

import (
	"context"
	"net/http"
)

type contextKey string

const userIDKey contextKey = "user_id"

func Method(method string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != method {
			Error(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		next(w, r)
	}
}

func RequireAuth(verify func(string) (int64, bool), next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("proxymorph_session")
		if err != nil {
			Error(w, http.StatusUnauthorized, "authentication required")
			return
		}
		userID, ok := verify(cookie.Value)
		if !ok {
			Error(w, http.StatusUnauthorized, "authentication required")
			return
		}
		ctx := context.WithValue(r.Context(), userIDKey, userID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func UserID(ctx context.Context) int64 {
	id, _ := ctx.Value(userIDKey).(int64)
	return id
}
