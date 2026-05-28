package app

import (
	"net/http"

	"github.com/liulei/proxymorph/internal/auth"
	"github.com/liulei/proxymorph/internal/config"
	"github.com/liulei/proxymorph/internal/nodes"
	"github.com/liulei/proxymorph/internal/storage"
	"github.com/liulei/proxymorph/internal/subscription"
	"github.com/liulei/proxymorph/internal/tasks"
)

type App struct {
	cfg     config.Config
	db      *storage.DB
	authSvc *auth.Service
	taskSvc *tasks.Service
	nodeSvc *nodes.Service
	subSvc  *subscription.Service
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
	a := &App{
		cfg:     cfg,
		db:      db,
		authSvc: authSvc,
		taskSvc: tasks.NewService(db),
		nodeSvc: nodes.NewService(db),
		subSvc:  subscription.NewService(db, http.DefaultClient),
		mux:     http.NewServeMux(),
	}
	a.subSvc.SetVLESSRelay(cfg.VLESSRelay)
	if err := a.subSvc.RestoreVLESSRelay(); err != nil {
		db.Close()
		return nil, err
	}
	a.routes()
	return a, nil
}

func (a *App) Handler() http.Handler {
	return a.mux
}

func (a *App) Close() error {
	if err := a.subSvc.Close(); err != nil {
		_ = a.db.Close()
		return err
	}
	return a.db.Close()
}
