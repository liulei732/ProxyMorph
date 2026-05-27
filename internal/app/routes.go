package app

import (
	"html/template"
	"io/fs"
	"net/http"
	"strings"

	"github.com/liulei/proxymorph/internal/auth"
	"github.com/liulei/proxymorph/internal/httpapi"
	"github.com/liulei/proxymorph/internal/nodes"
	"github.com/liulei/proxymorph/internal/subscription"
	"github.com/liulei/proxymorph/internal/tasks"
	"github.com/liulei/proxymorph/internal/web"
)

func (a *App) routes() {
	authHandler := auth.Handler{Service: a.authSvc}
	taskHandler := tasks.Handler{Service: a.taskSvc, Subscription: a.subSvc}
	nodeHandler := nodes.Handler{Service: a.nodeSvc}
	subHandler := subscription.Handler{Service: a.subSvc}
	a.mux.HandleFunc("/healthz", httpapi.Method(http.MethodGet, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("ok\n"))
	}))
	a.mux.HandleFunc("/api/login", httpapi.Method(http.MethodPost, authHandler.Login))
	a.mux.Handle("/api/me", httpapi.RequireAuth(a.authSvc.VerifyToken, http.HandlerFunc(httpapi.Method(http.MethodGet, authHandler.Me))))
	a.mux.Handle("/api/tasks", httpapi.RequireAuth(a.authSvc.VerifyToken, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			taskHandler.List(w, r)
		case http.MethodPost:
			taskHandler.Create(w, r)
		default:
			httpapi.Error(w, http.StatusMethodNotAllowed, "method not allowed")
		}
	})))
	a.mux.Handle("/api/tasks/", httpapi.RequireAuth(a.authSvc.VerifyToken, http.HandlerFunc(taskHandler.ServeTask)))
	a.mux.Handle("/api/rule-config", httpapi.RequireAuth(a.authSvc.VerifyToken, http.HandlerFunc(taskHandler.RuleConfig)))
	a.mux.Handle("/api/managed-config-defaults", httpapi.RequireAuth(a.authSvc.VerifyToken, http.HandlerFunc(taskHandler.ManagedConfigDefaults)))
	a.mux.Handle("/api/nodes", httpapi.RequireAuth(a.authSvc.VerifyToken, http.HandlerFunc(httpapi.Method(http.MethodGet, nodeHandler.List))))
	a.mux.Handle("/api/nodes/import", httpapi.RequireAuth(a.authSvc.VerifyToken, http.HandlerFunc(httpapi.Method(http.MethodPost, nodeHandler.Import))))
	a.mux.HandleFunc("/sub/", httpapi.Method(http.MethodGet, subHandler.ServeToken))
	a.mux.HandleFunc("/", a.serveWeb)
}

func (a *App) serveWeb(w http.ResponseWriter, r *http.Request) {
	dist, err := fs.Sub(web.Dist, "dist")
	if err != nil {
		httpapi.Error(w, http.StatusInternalServerError, "web assets unavailable")
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/")
	if path == "" {
		path = "index.html"
	}
	if _, err := fs.Stat(dist, path); err != nil {
		path = "index.html"
	}
	if path == "index.html" {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		tpl, err := template.ParseFS(dist, "index.html")
		if err != nil {
			httpapi.Error(w, http.StatusInternalServerError, "index unavailable")
			return
		}
		_ = tpl.Execute(w, nil)
		return
	}
	http.FileServer(http.FS(dist)).ServeHTTP(w, r)
}
