package subscription

import (
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/liulei/proxymorph/internal/storage"
)

func TestGenerateMergesPinnedNodes(t *testing.T) {
	upstreamContent := "proxies:\n  - name: Remote\n    type: ss\n    server: remote.example\n    port: 8388\n    cipher: aes-256-gcm\n    password: pass\nrules:\n  - FINAL,Proxy\n"

	db, err := storage.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	seedTaskAndPinnedNode(t, db, "https://upstream.example.test/clash.yaml")

	service := NewService(db, &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(upstreamContent)),
			Header:     make(http.Header),
		}, nil
	})})
	output, err := service.GenerateByTaskID(1)
	if err != nil {
		t.Fatalf("GenerateByTaskID returned error: %v", err)
	}
	if !strings.Contains(output, "Remote = ss") || !strings.Contains(output, "Pinned = trojan") {
		t.Fatalf("expected remote and pinned nodes in output:\n%s", output)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return fn(r)
}

func seedTaskAndPinnedNode(t *testing.T, db *storage.DB, upstreamURL string) {
	t.Helper()
	userRes, err := db.SQL().Exec(`INSERT INTO users (username, password_hash) VALUES ('admin', 'hash')`)
	if err != nil {
		t.Fatal(err)
	}
	userID, err := userRes.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.SQL().Exec(`
		INSERT INTO conversion_tasks (
			id, user_id, name, input_type, output_type, source_url, enabled,
			refresh_interval_seconds, merge_default_pinned_nodes, pinned_node_order_mode
		) VALUES (1, ?, 'Main', 'clash', 'surge6', ?, 1, 3600, 1, 'after_remote')`, userID, upstreamURL)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.SQL().Exec(`
		INSERT INTO pinned_nodes (
			user_id, name, protocol, server, port, parameters_json, tags_json,
			enabled, default_include, sort_order
		) VALUES (?, 'Pinned', 'trojan', 'pinned.example', 443, '{"password":"secret"}', '[]', 1, 1, 0)`, userID)
	if err != nil {
		t.Fatal(err)
	}
}
