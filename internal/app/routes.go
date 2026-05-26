package app

import (
	"net/http"

	"github.com/liulei/proxymorph/internal/auth"
	"github.com/liulei/proxymorph/internal/httpapi"
	"github.com/liulei/proxymorph/internal/nodes"
	"github.com/liulei/proxymorph/internal/tasks"
)

func (a *App) routes() {
	authHandler := auth.Handler{Service: a.authSvc}
	taskHandler := tasks.Handler{Service: a.taskSvc}
	nodeHandler := nodes.Handler{Service: a.nodeSvc}
	a.mux.HandleFunc("/healthz", httpapi.Method(http.MethodGet, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("ok\n"))
	}))
	a.mux.HandleFunc("/api/login", httpapi.Method(http.MethodPost, authHandler.Login))
	a.mux.HandleFunc("/api/tasks", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			taskHandler.List(w, r)
		case http.MethodPost:
			taskHandler.Create(w, r)
		default:
			httpapi.Error(w, http.StatusMethodNotAllowed, "method not allowed")
		}
	})
	a.mux.HandleFunc("/api/nodes", httpapi.Method(http.MethodGet, nodeHandler.List))
	a.mux.HandleFunc("/api/nodes/import", httpapi.Method(http.MethodPost, nodeHandler.Import))
}
