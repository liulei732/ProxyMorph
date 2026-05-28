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
	"github.com/liulei/proxymorph/internal/tasks"
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

func TestGenerateDoesNotRelayVLESSWhenSettingDisabled(t *testing.T) {
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

	_, err = service.GenerateByTaskID(1)
	if err == nil || !strings.Contains(err.Error(), "no Surge 6 compatible proxy nodes") {
		t.Fatalf("error = %v, want no compatible proxy nodes when VLESS setting is disabled", err)
	}
}

func TestGenerateRelaysVLESSWhenRelaySettingEnabled(t *testing.T) {
	uriList := "vless://f47ac10b-58cc-4372-a567-0e02b2c3d479@edge.example:443?security=tls&sni=edge.example&type=ws&path=%2Fproxy&host=cdn.example#Edge"
	upstreamContent := base64.StdEncoding.EncodeToString([]byte(uriList))

	db, err := storage.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	seedTaskWithoutPinnedNodes(t, db, "https://upstream.example.test/sub")
	seedVLESSRelaySetting(t, db, true)

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

func TestGenerateRelaysVLESSWhenTaskEnablesRelayAndGlobalDisabled(t *testing.T) {
	output, err := generateVLESSWithRelaySettings(t, false, "enabled")
	if err != nil {
		t.Fatalf("GenerateByTaskID returned error: %v", err)
	}
	if !strings.Contains(output, "Edge = socks5, proxy.example.test, 19000") {
		t.Fatalf("output should relay VLESS when task enables it:\n%s", output)
	}
}

func TestGenerateDoesNotRelayVLESSWhenTaskDisablesRelayAndGlobalEnabled(t *testing.T) {
	_, err := generateVLESSWithRelaySettings(t, true, "disabled")
	if err == nil || !strings.Contains(err.Error(), "no Surge 6 compatible proxy nodes") {
		t.Fatalf("error = %v, want no compatible proxy nodes when task disables relay", err)
	}
}

func TestGenerateRelaysVLESSWhenTaskFollowsEnabledGlobal(t *testing.T) {
	output, err := generateVLESSWithRelaySettings(t, true, "global")
	if err != nil {
		t.Fatalf("GenerateByTaskID returned error: %v", err)
	}
	if !strings.Contains(output, "Edge = socks5, proxy.example.test, 19000") {
		t.Fatalf("output should relay VLESS when task follows enabled global setting:\n%s", output)
	}
}

func TestGenerateDoesNotRelayVLESSWhenTaskFollowsDisabledGlobal(t *testing.T) {
	_, err := generateVLESSWithRelaySettings(t, false, "global")
	if err == nil || !strings.Contains(err.Error(), "no Surge 6 compatible proxy nodes") {
		t.Fatalf("error = %v, want no compatible proxy nodes when task follows disabled global setting", err)
	}
}

func TestGenerateRelaysVLESSUsesRequestHostWhenConfiguredHostIsLocalhost(t *testing.T) {
	uriList := "vless://f47ac10b-58cc-4372-a567-0e02b2c3d479@edge.example:443?security=tls&sni=edge.example&type=tcp#Edge"
	upstreamContent := base64.StdEncoding.EncodeToString([]byte(uriList))

	db, err := storage.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	seedTaskWithoutPinnedNodes(t, db, "https://upstream.example.test/sub")
	seedVLESSRelaySetting(t, db, true)
	if err := seedTaskToken(t, db, 1); err != nil {
		t.Fatal(err)
	}

	service := NewService(db, &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(upstreamContent)),
			Header:     make(http.Header),
		}, nil
	})})
	service.SetVLESSRelay(config.VLESSRelayConfig{
		Enabled:     true,
		PublicHost:  "localhost",
		ListenHost:  "0.0.0.0",
		PortStart:   19000,
		PortEnd:     19010,
		Username:    "relay",
		Password:    "secret",
		SingBoxPath: fakeSingBox(t),
		ConfigPath:  filepath.Join(t.TempDir(), "sing-box.json"),
	})
	defer service.Close()

	output, err := service.GenerateByTokenWithRelayHost(mustTaskToken(t, db), "proxy.example.test:8080")
	if err != nil {
		t.Fatalf("GenerateByTokenWithRelayHost returned error: %v", err)
	}
	if !strings.Contains(output, "Edge = socks5, proxy.example.test, 19000") {
		t.Fatalf("output should use request host without port instead of localhost:\n%s", output)
	}
	if strings.Contains(output, "localhost") {
		t.Fatalf("output should not contain localhost:\n%s", output)
	}
}

func generateVLESSWithRelaySettings(t *testing.T, globalEnabled bool, taskMode string) (string, error) {
	t.Helper()
	uriList := "vless://f47ac10b-58cc-4372-a567-0e02b2c3d479@edge.example:443?security=tls&sni=edge.example&type=ws&path=%2Fproxy&host=cdn.example#Edge"
	upstreamContent := base64.StdEncoding.EncodeToString([]byte(uriList))

	db, err := storage.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	seedTaskWithoutPinnedNodes(t, db, "https://upstream.example.test/sub")
	seedVLESSRelaySetting(t, db, globalEnabled)
	if _, err := db.SQL().Exec(`UPDATE conversion_tasks SET vless_relay_mode = ? WHERE id = 1`, taskMode); err != nil {
		t.Fatal(err)
	}

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
	t.Cleanup(func() { _ = service.Close() })

	return service.GenerateByTaskID(1)
}

func fakeSingBox(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "sing-box")
	script := "#!/bin/sh\nexec tail -f /dev/null\n"
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func seedTaskToken(t *testing.T, db *storage.DB, taskID int64) error {
	t.Helper()
	_, err := db.SQL().Exec(`INSERT INTO subscription_tokens (task_id, token) VALUES (?, 'test-token')`, taskID)
	return err
}

func seedVLESSRelaySetting(t *testing.T, db *storage.DB, enabled bool) {
	t.Helper()
	value := "false"
	if enabled {
		value = "true"
	}
	_, err := db.SQL().Exec(`INSERT INTO app_settings (key, value) VALUES ('vless_relay_enabled', ?)`, value)
	if err != nil {
		t.Fatal(err)
	}
}

func mustTaskToken(t *testing.T, db *storage.DB) string {
	t.Helper()
	var token string
	if err := db.SQL().QueryRow(`SELECT token FROM subscription_tokens WHERE task_id = 1`).Scan(&token); err != nil {
		t.Fatal(err)
	}
	return token
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

func TestGenerateAppliesTaskAndGlobalRuleConfig(t *testing.T) {
	upstreamContent := "proxies:\n  - name: Remote\n    type: ss\n    server: remote.example\n    port: 8388\n    cipher: aes-256-gcm\n    password: pass\nrules:\n  - DOMAIN,upstream.example,Proxy\n"

	db, err := storage.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	seedTaskWithoutPinnedNodes(t, db, "https://upstream.example.test/clash.yaml")
	_, err = db.SQL().Exec(`
		UPDATE conversion_tasks
		SET include_global_rules = 1,
			custom_rules_text = 'DOMAIN,task.example,DIRECT',
			rule_merge_mode = 'custom_first_dedupe',
			custom_groups_text = 'Manual = select, Proxy, DIRECT',
			managed_config_mode = 'enabled',
			managed_config_url_mode = 'task_subscription',
			managed_config_interval_mode = 'custom',
			managed_config_interval_seconds = 7200,
			managed_config_strict_mode = 'enabled'
		WHERE id = 1`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.SQL().Exec(`INSERT INTO app_settings (key, value) VALUES ('global_custom_rules_text', 'DOMAIN,global.example,DIRECT')`)
	if err != nil {
		t.Fatal(err)
	}
	if err := seedTaskToken(t, db, 1); err != nil {
		t.Fatal(err)
	}

	service := NewService(db, &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(upstreamContent)),
			Header:     make(http.Header),
		}, nil
	})})
	output, err := service.GenerateByTokenWithRelayHost("test-token", "localhost:8080")
	if err != nil {
		t.Fatalf("GenerateByTokenWithRelayHost returned error: %v", err)
	}
	if !strings.HasPrefix(output, "#!MANAGED-CONFIG http://localhost:8080/sub/test-token?name=Main interval=7200 strict=true\n") {
		t.Fatalf("managed header missing or not first:\n%s", output)
	}
	assertSubscriptionOrder(t, output,
		"Proxy = select, Remote",
		"Manual = select, Proxy, DIRECT",
		"DOMAIN,global.example,DIRECT",
		"DOMAIN,task.example,DIRECT",
		"DOMAIN,upstream.example,Proxy",
		"FINAL,Proxy",
	)
}

func TestGenerateStoresOutputAndWarningForGlobalProxyRuleWhenProxyGroupMissing(t *testing.T) {
	upstreamContent := `proxies:
  - name: Remote
    type: ss
    server: remote.example
    port: 8388
    cipher: aes-256-gcm
    password: pass
proxy-groups:
  - name: "🍃 Proxies"
    type: select
    proxies:
      - Remote
rules:
  - DOMAIN,upstream.example,🍃 Proxies
`

	db, err := storage.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	seedTaskWithoutPinnedNodes(t, db, "https://upstream.example.test/clash.yaml")
	_, err = db.SQL().Exec(`UPDATE conversion_tasks SET include_global_rules = 1, rule_merge_mode = 'custom_first' WHERE id = 1`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.SQL().Exec(`INSERT INTO app_settings (key, value) VALUES ('global_custom_rules_text', 'DOMAIN-SET,https://cdn.jsdelivr.net/gh/Loyalsoldier/surge-rules@release/gfw.txt,Proxy')`)
	if err != nil {
		t.Fatal(err)
	}

	service := NewService(db, &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(upstreamContent)),
			Header:     make(http.Header),
		}, nil
	})})
	output, err := service.GenerateByTaskID(1)
	if err == nil {
		t.Fatal("GenerateByTaskID should return a warning error when a global rule references a missing policy")
	}
	assertSubscriptionOrder(t, output,
		"🍃 Proxies = select, Remote",
		"DOMAIN-SET,https://cdn.jsdelivr.net/gh/Loyalsoldier/surge-rules@release/gfw.txt,Proxy",
		"DOMAIN,upstream.example,🍃 Proxies",
		"FINAL,🍃 Proxies",
	)
	var cached string
	if err := db.SQL().QueryRow(`SELECT content FROM output_cache WHERE task_id = 1`).Scan(&cached); err != nil {
		t.Fatalf("expected output cache despite warning: %v", err)
	}
	if cached != output {
		t.Fatalf("cached output should match returned output")
	}
	var lastError string
	if err := db.SQL().QueryRow(`SELECT last_error_message FROM conversion_tasks WHERE id = 1`).Scan(&lastError); err != nil {
		t.Fatalf("read last error: %v", err)
	}
	for _, want := range []string{
		"规则引用了不存在的策略：Proxy",
		"DOMAIN-SET,https://cdn.jsdelivr.net/gh/Loyalsoldier/surge-rules@release/gfw.txt,Proxy",
		"请修改规则策略或添加同名策略组",
		"🍃 Proxies",
	} {
		if !strings.Contains(err.Error(), want) || !strings.Contains(lastError, want) {
			t.Fatalf("error missing %q:\nerr=%v\nlastError=%s", want, err, lastError)
		}
	}
}

func TestManagedConfigTaskFollowsEnabledGlobalCustomURL(t *testing.T) {
	upstreamContent := "proxies:\n  - name: Remote\n    type: ss\n    server: remote.example\n    port: 8388\n    cipher: aes-256-gcm\n    password: pass\n"

	db, err := storage.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	seedTaskWithoutPinnedNodes(t, db, "https://upstream.example.test/clash.yaml")
	_, err = db.SQL().Exec(`
		INSERT INTO app_settings (key, value) VALUES
			('managed_config_enabled', 'true'),
			('managed_config_url_mode', 'custom'),
			('managed_config_custom_url', 'https://profiles.example.com/default.conf'),
			('managed_config_interval_seconds', '3600'),
			('managed_config_strict', 'false')`)
	if err != nil {
		t.Fatal(err)
	}

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
	want := "#!MANAGED-CONFIG https://profiles.example.com/default.conf interval=3600 strict=false"
	if firstLine(output) != want {
		t.Fatalf("first line = %q, want %q", firstLine(output), want)
	}
}

func TestManagedConfigTaskDisabledOverridesEnabledGlobal(t *testing.T) {
	upstreamContent := "proxies:\n  - name: Remote\n    type: ss\n    server: remote.example\n    port: 8388\n    cipher: aes-256-gcm\n    password: pass\n"

	db, err := storage.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	seedTaskWithoutPinnedNodes(t, db, "https://upstream.example.test/clash.yaml")
	_, err = db.SQL().Exec(`UPDATE conversion_tasks SET managed_config_mode = 'disabled' WHERE id = 1`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.SQL().Exec(`
		INSERT INTO app_settings (key, value) VALUES
			('managed_config_enabled', 'true'),
			('managed_config_interval_seconds', '3600')`)
	if err != nil {
		t.Fatal(err)
	}

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
	if strings.HasPrefix(output, "#!MANAGED-CONFIG") {
		t.Fatalf("output should not include managed header:\n%s", output)
	}
}

func TestGenerateCanExcludeGlobalRuleConfig(t *testing.T) {
	upstreamContent := "proxies:\n  - name: Remote\n    type: ss\n    server: remote.example\n    port: 8388\n    cipher: aes-256-gcm\n    password: pass\nrules:\n  - DOMAIN,upstream.example,Proxy\n"

	db, err := storage.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	seedTaskWithoutPinnedNodes(t, db, "https://upstream.example.test/clash.yaml")
	_, err = db.SQL().Exec(`UPDATE conversion_tasks SET include_global_rules = 0, custom_rules_text = 'DOMAIN,task.example,DIRECT', rule_merge_mode = 'upstream_first' WHERE id = 1`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.SQL().Exec(`INSERT INTO app_settings (key, value) VALUES ('global_custom_rules_text', 'DOMAIN,global.example,DIRECT')`)
	if err != nil {
		t.Fatal(err)
	}

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
	assertSubscriptionOrder(t, output, "DOMAIN,upstream.example,Proxy", "DOMAIN,task.example,DIRECT", "FINAL,Proxy")
	if strings.Contains(output, "global.example") {
		t.Fatalf("global rule should be excluded:\n%s", output)
	}
}

func TestGeneratePreviewAppliesDraftWithoutSavingTask(t *testing.T) {
	upstreamContent := "proxies:\n  - name: Remote\n    type: ss\n    server: remote.example\n    port: 8388\n    cipher: aes-256-gcm\n    password: pass\nrules:\n  - DOMAIN,upstream.example,Proxy\n"

	db, err := storage.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	seedTaskWithoutPinnedNodes(t, db, "https://upstream.example.test/clash.yaml")
	draftRule := "DOMAIN,draft.example,DIRECT"

	service := NewService(db, &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(upstreamContent)),
			Header:     make(http.Header),
		}, nil
	})})
	output, err := service.GeneratePreviewByTaskID(1, tasks.UpdateInput{CustomRulesText: &draftRule})
	if err != nil {
		t.Fatalf("GeneratePreviewByTaskID returned error: %v", err)
	}
	if !strings.Contains(output, "DOMAIN,draft.example,DIRECT") {
		t.Fatalf("draft rule missing from preview:\n%s", output)
	}

	savedOutput, err := service.GenerateByTaskID(1)
	if err != nil {
		t.Fatalf("GenerateByTaskID returned error: %v", err)
	}
	if strings.Contains(savedOutput, "DOMAIN,draft.example,DIRECT") {
		t.Fatalf("draft rule was persisted unexpectedly:\n%s", savedOutput)
	}
}

func TestGeneratePreviewAppliesDraftFinalRulePolicyWithoutSavingTask(t *testing.T) {
	upstreamContent := "proxies:\n  - name: Remote\n    type: ss\n    server: remote.example\n    port: 8388\n    cipher: aes-256-gcm\n    password: pass\nrules:\n  - DOMAIN,upstream.example,Proxy\n"

	db, err := storage.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	seedTaskWithoutPinnedNodes(t, db, "https://upstream.example.test/clash.yaml")
	draftFinal := "DIRECT"

	service := NewService(db, &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(upstreamContent)),
			Header:     make(http.Header),
		}, nil
	})})
	output, err := service.GeneratePreviewByTaskID(1, tasks.UpdateInput{FinalRulePolicy: &draftFinal})
	if err != nil {
		t.Fatalf("GeneratePreviewByTaskID returned error: %v", err)
	}
	if !strings.Contains(output, "DOMAIN,upstream.example,Proxy\nFINAL,DIRECT\n") {
		t.Fatalf("draft final policy missing from preview:\n%s", output)
	}

	savedOutput, err := service.GenerateByTaskID(1)
	if err != nil {
		t.Fatalf("GenerateByTaskID returned error: %v", err)
	}
	if strings.Contains(savedOutput, "FINAL,DIRECT") {
		t.Fatalf("draft final policy was persisted unexpectedly:\n%s", savedOutput)
	}
}

func firstLine(text string) string {
	idx := strings.IndexByte(text, '\n')
	if idx == -1 {
		return text
	}
	return text[:idx]
}

func assertSubscriptionOrder(t *testing.T, text string, values ...string) {
	t.Helper()
	last := -1
	for _, value := range values {
		idx := strings.Index(text, value)
		if idx == -1 {
			t.Fatalf("output missing %q:\n%s", value, text)
		}
		if idx < last {
			t.Fatalf("%q should appear after previous values:\n%s", value, text)
		}
		last = idx
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
