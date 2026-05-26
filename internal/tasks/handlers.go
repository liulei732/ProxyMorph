package tasks

import (
	"encoding/json"
	"net/http"

	"github.com/liulei/proxymorph/internal/httpapi"
)

type Handler struct {
	Service *Service
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
