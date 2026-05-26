package app

import (
	"net/http"

	"github.com/liulei/proxymorph/internal/auth"
	"github.com/liulei/proxymorph/internal/config"
	"github.com/liulei/proxymorph/internal/storage"
)

type App struct {
	cfg     config.Config
	db      *storage.DB
	authSvc *auth.Service
	mux     *http.ServeMux
}

func New(cfg config.Config, dbPath string) (*App, error) {
	db, err := storage.Open(dbPath)
	if err != nil {
		return nil, err
	}
	authSvc := auth.NewService(db, []byte(cfg.SessionSecret))
	if err := authSvc.EnsureAdmin(cfg.InitialAdminUsername, cfg.InitialAdminPassword); err != nil {
		db.Close()
		return nil, err
	}
	a := &App{cfg: cfg, db: db, authSvc: authSvc, mux: http.NewServeMux()}
	a.routes()
	return a, nil
}

func (a *App) Handler() http.Handler {
	return a.mux
}

func (a *App) Close() error {
	return a.db.Close()
}
