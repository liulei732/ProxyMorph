package subscription

import (
	"net/http"
	"strings"

	"github.com/liulei/proxymorph/internal/httpapi"
)

type Handler struct {
	Service *Service
}

func (h Handler) ServeToken(w http.ResponseWriter, r *http.Request) {
	token := strings.TrimPrefix(r.URL.Path, "/sub/")
	if token == "" || token == r.URL.Path {
		httpapi.Error(w, http.StatusNotFound, "subscription not found")
		return
	}
	output, err := h.Service.GenerateByToken(token)
	if err != nil {
		httpapi.Error(w, http.StatusBadGateway, "subscription generation failed")
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte(output))
}
