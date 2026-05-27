package tasks

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/liulei/proxymorph/internal/httpapi"
)

type Handler struct {
	Service      *Service
	Subscription SubscriptionGenerator
}

type SubscriptionGenerator interface {
	GenerateByTaskID(taskID int64) (string, error)
}

func (h Handler) List(w http.ResponseWriter, r *http.Request) {
	userID := httpapi.UserID(r.Context())
	tasks, err := h.Service.List(userID)
	if err != nil {
		httpapi.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	httpapi.JSON(w, http.StatusOK, tasks)
}

func (h Handler) RuleConfig(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		config, err := h.Service.GlobalRuleConfig()
		if err != nil {
			httpapi.Error(w, http.StatusInternalServerError, err.Error())
			return
		}
		httpapi.JSON(w, http.StatusOK, config)
	case http.MethodPut:
		var input GlobalRuleConfigInput
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			httpapi.Error(w, http.StatusBadRequest, "invalid json")
			return
		}
		config, err := h.Service.UpdateGlobalRuleConfig(input)
		if err != nil {
			httpapi.Error(w, http.StatusBadRequest, err.Error())
			return
		}
		httpapi.JSON(w, http.StatusOK, config)
	default:
		httpapi.Error(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (h Handler) ManagedConfigDefaults(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		config, err := h.Service.ManagedConfigDefaults()
		if err != nil {
			httpapi.Error(w, http.StatusInternalServerError, err.Error())
			return
		}
		httpapi.JSON(w, http.StatusOK, config)
	case http.MethodPut:
		var input ManagedConfigDefaultsInput
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			httpapi.Error(w, http.StatusBadRequest, "invalid json")
			return
		}
		config, err := h.Service.UpdateManagedConfigDefaults(input)
		if err != nil {
			httpapi.Error(w, http.StatusBadRequest, err.Error())
			return
		}
		httpapi.JSON(w, http.StatusOK, config)
	default:
		httpapi.Error(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (h Handler) Create(w http.ResponseWriter, r *http.Request) {
	userID := httpapi.UserID(r.Context())
	var input CreateInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		httpapi.Error(w, http.StatusBadRequest, "invalid json")
		return
	}
	task, err := h.Service.Create(userID, input)
	if err != nil {
		httpapi.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	httpapi.JSON(w, http.StatusCreated, task)
}

func (h Handler) ServeTask(w http.ResponseWriter, r *http.Request) {
	id, action, ok := parseTaskPath(r.URL.Path)
	if !ok {
		httpapi.Error(w, http.StatusNotFound, "task not found")
		return
	}
	switch {
	case action == "" && r.Method == http.MethodPatch:
		h.Update(w, r, id)
	case action == "" && r.Method == http.MethodDelete:
		h.Delete(w, r, id)
	case action == "generate" && r.Method == http.MethodPost:
		h.Generate(w, r, id)
	case action == "preview" && r.Method == http.MethodGet:
		h.Preview(w, r, id)
	default:
		httpapi.Error(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (h Handler) Update(w http.ResponseWriter, r *http.Request, id int64) {
	var input UpdateInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		httpapi.Error(w, http.StatusBadRequest, "invalid json")
		return
	}
	task, err := h.Service.Update(httpapi.UserID(r.Context()), id, input)
	if err != nil {
		httpapi.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	httpapi.JSON(w, http.StatusOK, task)
}

func (h Handler) Delete(w http.ResponseWriter, r *http.Request, id int64) {
	if err := h.Service.Delete(httpapi.UserID(r.Context()), id); err != nil {
		httpapi.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	httpapi.JSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (h Handler) Generate(w http.ResponseWriter, r *http.Request, id int64) {
	if _, err := h.Service.Get(httpapi.UserID(r.Context()), id); err != nil {
		httpapi.Error(w, http.StatusNotFound, "task not found")
		return
	}
	output, err := h.Subscription.GenerateByTaskID(id)
	if err != nil {
		httpapi.Error(w, http.StatusBadGateway, err.Error())
		return
	}
	task, _ := h.Service.Get(httpapi.UserID(r.Context()), id)
	httpapi.JSON(w, http.StatusOK, map[string]any{"ok": true, "task": task, "content": output})
}

func (h Handler) Preview(w http.ResponseWriter, r *http.Request, id int64) {
	if _, err := h.Service.Get(httpapi.UserID(r.Context()), id); err != nil {
		httpapi.Error(w, http.StatusNotFound, "task not found")
		return
	}
	output, err := h.Subscription.GenerateByTaskID(id)
	if err != nil {
		httpapi.Error(w, http.StatusBadGateway, err.Error())
		return
	}
	httpapi.JSON(w, http.StatusOK, map[string]string{"content": output})
}

func parseTaskPath(path string) (int64, string, bool) {
	rest := strings.TrimPrefix(path, "/api/tasks/")
	if rest == path || rest == "" {
		return 0, "", false
	}
	parts := strings.Split(strings.Trim(rest, "/"), "/")
	if len(parts) == 0 || len(parts) > 2 {
		return 0, "", false
	}
	id, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || id <= 0 {
		return 0, "", false
	}
	action := ""
	if len(parts) == 2 {
		action = parts[1]
	}
	return id, action, true
}
