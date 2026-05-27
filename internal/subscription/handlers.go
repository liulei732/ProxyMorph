package subscription

import (
	"log"
	"net/http"
	"strings"

	"github.com/liulei/proxymorph/internal/httpapi"
)

type Handler struct {
	Service Generator
}

type Generator interface {
	GenerateByToken(token string) (string, error)
	GenerateByTokenWithRelayHost(token, relayHost string) (string, error)
}

func (h Handler) ServeToken(w http.ResponseWriter, r *http.Request) {
	token := strings.TrimPrefix(r.URL.Path, "/sub/")
	if token == "" || token == r.URL.Path {
		httpapi.Error(w, http.StatusNotFound, "subscription not found")
		return
	}
	output, err := h.Service.GenerateByTokenWithRelayHost(token, r.Host)
	if err != nil {
		log.Printf("subscription generation failed token=%q error=%q", safeTokenLabel(token), err)
		httpapi.Error(w, http.StatusBadGateway, "subscription generation failed")
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte(output))
}
