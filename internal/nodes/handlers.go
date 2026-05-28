package nodes

import (
	"encoding/json"
	"net/http"
	"strconv"
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
				if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
					continue
				}
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

func (h Handler) Batch(w http.ResponseWriter, r *http.Request) {
	userID := httpapi.UserID(r.Context())
	var input BatchInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		httpapi.Error(w, http.StatusBadRequest, "invalid json")
		return
	}
	affected, err := h.Service.Batch(userID, input)
	if err != nil {
		httpapi.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	httpapi.JSON(w, http.StatusOK, map[string]any{"ok": true, "affected": affected})
}

func (h Handler) ServeNode(w http.ResponseWriter, r *http.Request) {
	id, ok := parseNodeID(r.URL.Path)
	if !ok {
		httpapi.Error(w, http.StatusNotFound, "node not found")
		return
	}
	switch r.Method {
	case http.MethodDelete:
		h.Delete(w, r, id)
	default:
		httpapi.Error(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (h Handler) Delete(w http.ResponseWriter, r *http.Request, id int64) {
	if err := h.Service.Delete(httpapi.UserID(r.Context()), id); err != nil {
		httpapi.Error(w, http.StatusNotFound, err.Error())
		return
	}
	httpapi.JSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func parseNodeID(path string) (int64, bool) {
	rest := strings.TrimPrefix(path, "/api/nodes/")
	if rest == path || rest == "" || strings.Contains(strings.Trim(rest, "/"), "/") {
		return 0, false
	}
	id, err := strconv.ParseInt(strings.Trim(rest, "/"), 10, 64)
	return id, err == nil && id > 0
}
