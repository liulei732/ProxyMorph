package subscription

import (
	"encoding/base64"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/liulei/proxymorph/internal/config"
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
		"vmess://eyJwcyI6IkxlZ2FjeSIsImFkZCI6InZtZXNzLmV4YW1wbGUuY29tIiwicG9ydCI6IjQ0MyIsImlkIjoiZjQ3YWMxMGItNThjYy00MzcyLWE1NjctMGUwMmIyYzNkNDc5IiwidGxzIjoidGxzIiwic25pIjoic25pLmV4YW1wbGUuY29tIiwibmV0Ijoid3MiLCJwYXRoIjoiL3Byb3h5IiwiaG9zdCI6Imhvc3QuZXhhbXBsZS5jb20ifQ==",
		"trojan://secret@trojan.example:2053?allowInsecure=0&sni=sni.example.com&type=ws&path=%2Fvideo&host=host.example.com#TrojanWS",
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
		"Legacy = vmess, vmess.example.com, 443",
		"username=f47ac10b-58cc-4372-a567-0e02b2c3d479",
		"ws=true",
		"TrojanWS = trojan, trojan.example, 2053",
		"ws-path=/video",
		"ws-headers=Host:host.example.com",
		"Proxy = select, Remote, Legacy, TrojanWS, Pinned",
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

func TestGenerateRelaysVLESSWhenRelayEnabled(t *testing.T) {
	uriList := "vless://f47ac10b-58cc-4372-a567-0e02b2c3d479@edge.example:443?security=tls&sni=edge.example&type=ws&path=%2Fproxy&host=cdn.example#Edge"
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
	service.SetVLESSRelay(config.VLESSRelayConfig{
		Enabled:     true,
		PublicHost:  "proxy.example.test",
		ListenHost:  "0.0.0.0",
		PortStart:   19000,
		PortEnd:     19010,
		Username:    "relay",
		Password:    "secret",
		SingBoxPath: fakeSingBox(t),
		ConfigPath:  filepath.Join(t.TempDir(), "sing-box.json"),
	})
	defer service.Close()

	output, err := service.GenerateByTaskID(1)
	if err != nil {
		t.Fatalf("GenerateByTaskID returned error: %v", err)
	}
	for _, want := range []string{
		"Edge = socks5, proxy.example.test, 19000",
		"username=relay",
		"password=secret",
		"Proxy = select, Edge",
		"FINAL,Proxy",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("output missing %q:\n%s", want, output)
		}
	}
	if strings.Contains(output, "= vless") {
		t.Fatalf("output should not contain native VLESS proxy:\n%s", output)
	}
}

func fakeSingBox(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "sing-box")
	script := "#!/bin/sh\nwhile true; do sleep 1; done\n"
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestGenerateSkipsUnsupportedURIListEntries(t *testing.T) {
	uriList := strings.Join([]string{
		"http://legacy.example",
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
	if info.Skipped != 1 || !strings.Contains(info.SkipSummary, "scheme=http") {
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
