package nodes

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/liulei/proxymorph/internal/httpapi"
	"github.com/liulei/proxymorph/internal/storage"
)

func TestHandlerBatchUpdatesNodesForCurrentUser(t *testing.T) {
	db, err := storage.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	userID := seedUser(t, db)
	service := NewService(db)
	node, err := service.Create(userID, CreateInput{Name: "Edge", Protocol: "trojan", Server: "edge.example.com", Port: 443, Params: map[string]string{"password": "secret"}, Enabled: false})
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/nodes/batch", strings.NewReader(`{"action":"enable","ids":[`+strconv.FormatInt(node.ID, 10)+`]}`))
	req.AddCookie(&http.Cookie{Name: "proxymorph_session", Value: "valid"})
	res := httptest.NewRecorder()

	httpapi.RequireAuth(func(token string) (int64, bool) {
		return userID, token == "valid"
	}, http.HandlerFunc(Handler{Service: service}.Batch)).ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", res.Code, res.Body.String())
	}
	var payload struct {
		OK       bool  `json:"ok"`
		Affected int64 `json:"affected"`
	}
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	if !payload.OK || payload.Affected != 1 {
		t.Fatalf("unexpected response: %#v", payload)
	}
	nodes, err := service.List(userID)
	if err != nil {
		t.Fatal(err)
	}
	if !nodes[0].Enabled {
		t.Fatalf("node should be enabled: %#v", nodes[0])
	}
}

func TestHandlerBatchRejectsBadJSON(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/nodes/batch", strings.NewReader(`{`))
	res := httptest.NewRecorder()

	Handler{Service: NewService(nil)}.Batch(res, req)

	if res.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body=%s", res.Code, res.Body.String())
	}
}
