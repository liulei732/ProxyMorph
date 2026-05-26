package nodes

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/liulei/proxymorph/internal/httpapi"
)

type Handler struct {
	Service *Service
}

func (h Handler) List(w http.ResponseWriter, r *http.Request) {
	userID := httpapi.UserID(r.Context())
	nodes, err := h.Service.List(userID)
	if err != nil {
		httpapi.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	httpapi.JSON(w, http.StatusOK, nodes)
}

func (h Handler) Import(w http.ResponseWriter, r *http.Request) {
	userID := httpapi.UserID(r.Context())
	var input struct {
		URIs []string `json:"uris"`
		Text string   `json:"text"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		httpapi.Error(w, http.StatusBadRequest, "invalid json")
		return
	}
	uris := input.URIs
	if len(uris) == 0 && input.Text != "" {
		for _, line := range strings.Split(input.Text, "\n") {
			if trimmed := strings.TrimSpace(line); trimmed != "" {
				uris = append(uris, trimmed)
			}
		}
	}
	imported, err := h.Service.ImportURIs(userID, uris)
	if err != nil {
		httpapi.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	httpapi.JSON(w, http.StatusCreated, imported)
}
