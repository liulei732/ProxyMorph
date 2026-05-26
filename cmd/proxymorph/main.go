package main

import (
	"log"
	"net/http"
	"path/filepath"

	"github.com/liulei/proxymorph/internal/app"
	"github.com/liulei/proxymorph/internal/config"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}
	application, err := app.New(cfg, filepath.Join(cfg.DataDir, "proxymorph.db"))
	if err != nil {
		log.Fatal(err)
	}
	defer application.Close()
	log.Printf("ProxyMorph listening on %s", cfg.Addr)
	log.Fatal(http.ListenAndServe(cfg.Addr, application.Handler()))
}
