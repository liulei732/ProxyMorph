package subscription

import (
	"encoding/base64"
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

func TestGenerateSupportsBase64URIListSubscriptions(t *testing.T) {
	uriList := strings.Join([]string{
		"ss://YWVzLTI1Ni1nY206cGFzc3dvcmQ@remote.example:8388#Remote",
		"vless://f47ac10b-58cc-4372-a567-0e02b2c3d479@edge.example:443?security=tls&sni=edge.example&type=ws&path=%2Fproxy#Edge",
	}, "\n")
	upstreamContent := base64.StdEncoding.EncodeToString([]byte(uriList))

	db, err := storage.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	seedTaskAndPinnedNode(t, db, "https://upstream.example.test/sub")

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
	for _, want := range []string{
		"Remote = ss, remote.example, 8388",
		"Proxy = select, Remote, Pinned",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("output missing %q:\n%s", want, output)
		}
	}
	if strings.Contains(output, "vless") || strings.Contains(output, "Edge") {
		t.Fatalf("output should not contain unsupported VLESS node:\n%s", output)
	}
}

func TestGenerateFailsWhenSubscriptionOnlyContainsVLESS(t *testing.T) {
	uriList := "vless://f47ac10b-58cc-4372-a567-0e02b2c3d479@edge.example:443?security=tls&sni=edge.example&type=tcp#Edge"
	upstreamContent := base64.StdEncoding.EncodeToString([]byte(uriList))

	db, err := storage.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	seedTaskWithoutPinnedNodes(t, db, "https://upstream.example.test/sub")

	service := NewService(db, &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(upstreamContent)),
			Header:     make(http.Header),
		}, nil
	})})
	_, err = service.GenerateByTaskID(1)
	if err == nil || !strings.Contains(err.Error(), "no Surge 6 compatible proxy nodes") {
		t.Fatalf("error = %v, want no compatible proxy nodes", err)
	}
}

func TestGenerateSkipsUnsupportedURIListEntries(t *testing.T) {
	uriList := strings.Join([]string{
		"vmess://eyJwcyI6IkxlZ2FjeSIsImFkZCI6ImxlZ2FjeS5leGFtcGxlIiwicG9ydCI6IjQ0MyJ9",
		"ss://YWVzLTI1Ni1nY206cGFzc3dvcmQ@remote.example:8388#Remote",
	}, "\n")
	upstreamContent := base64.StdEncoding.EncodeToString([]byte(uriList))

	doc, info, err := parseSubscription([]byte(upstreamContent))
	if err != nil {
		t.Fatalf("parseSubscription returned error: %v", err)
	}
	if len(doc.Nodes) != 1 || doc.Nodes[0].Name != "Remote" {
		t.Fatalf("unexpected nodes: %#v", doc.Nodes)
	}
	if info.Skipped != 1 || !strings.Contains(info.SkipSummary, "scheme=vmess") {
		t.Fatalf("unexpected parse info: %#v", info)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return fn(r)
}

func seedTaskAndPinnedNode(t *testing.T, db *storage.DB, upstreamURL string) {
	t.Helper()
	userID := seedTaskUser(t, db)
	seedTask(t, db, userID, upstreamURL, 1)
	_, err := db.SQL().Exec(`
		INSERT INTO pinned_nodes (
			user_id, name, protocol, server, port, parameters_json, tags_json,
			enabled, default_include, sort_order
		) VALUES (?, 'Pinned', 'trojan', 'pinned.example', 443, '{"password":"secret"}', '[]', 1, 1, 0)`, userID)
	if err != nil {
		t.Fatal(err)
	}
}

func seedTaskWithoutPinnedNodes(t *testing.T, db *storage.DB, upstreamURL string) {
	t.Helper()
	userID := seedTaskUser(t, db)
	seedTask(t, db, userID, upstreamURL, 0)
}

func seedTaskUser(t *testing.T, db *storage.DB) int64 {
	t.Helper()
	userRes, err := db.SQL().Exec(`INSERT INTO users (username, password_hash) VALUES ('admin', 'hash')`)
	if err != nil {
		t.Fatal(err)
	}
	userID, err := userRes.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	return userID
}

func seedTask(t *testing.T, db *storage.DB, userID int64, upstreamURL string, mergeDefaultPinnedNodes int) {
	t.Helper()
	_, err := db.SQL().Exec(`
		INSERT INTO conversion_tasks (
			id, user_id, name, input_type, output_type, source_url, enabled,
			refresh_interval_seconds, merge_default_pinned_nodes, pinned_node_order_mode
		) VALUES (1, ?, 'Main', 'clash', 'surge6', ?, 1, 3600, ?, 'after_remote')`, userID, upstreamURL, mergeDefaultPinnedNodes)
	if err != nil {
		t.Fatal(err)
	}
}
