package app

import (
	"net/http"

	"github.com/liulei/proxymorph/internal/auth"
	"github.com/liulei/proxymorph/internal/httpapi"
)

func (a *App) routes() {
	authHandler := auth.Handler{Service: a.authSvc}
	a.mux.HandleFunc("/healthz", httpapi.Method(http.MethodGet, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("ok\n"))
	}))
	a.mux.HandleFunc("/api/login", httpapi.Method(http.MethodPost, authHandler.Login))
}
