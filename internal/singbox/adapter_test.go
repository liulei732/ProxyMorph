package singbox

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/liulei/proxymorph/internal/config"
	"github.com/liulei/proxymorph/internal/convert"
)

func TestAdapterAvailableReportsMissingBinary(t *testing.T) {
	err := New("/definitely/missing/sing-box").Available()
	if !errors.Is(err, ErrUnavailable) {
		t.Fatalf("error = %v, want ErrUnavailable", err)
	}
}

func TestManagerStartRestartsSingBoxWithGeneratedConfig(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "args.log")
	binPath := filepath.Join(dir, "fake-sing-box")
	script := "#!/bin/sh\nprintf '%s\\n' \"$@\" >> " + logPath + "\nexec tail -f /dev/null\n"
	if err := os.WriteFile(binPath, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	manager := NewManager(config.VLESSRelayConfig{
		Enabled:     true,
		PublicHost:  "proxy.example.com",
		ListenHost:  "0.0.0.0",
		PortStart:   19000,
		PortEnd:     19000,
		Password:    "secret",
		SingBoxPath: binPath,
		ConfigPath:  filepath.Join(dir, "sing-box.json"),
	})
	if _, err := manager.Configure([]convert.Node{{Name: "Edge", Protocol: "vless", Server: "edge.example", Port: 443, Params: map[string]string{"uuid": "uuid"}}}); err != nil {
		t.Fatalf("Configure returned error: %v", err)
	}
	if err := manager.Start(); err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	defer manager.Stop()
	if manager.cmd == nil || manager.cmd.Process == nil {
		t.Fatal("expected sing-box process to be started")
	}
	args, err := waitForFile(logPath)
	if err != nil {
		t.Fatalf("read args log: %v", err)
	}
	if !strings.Contains(string(args), "run\n-c\n"+manager.cfg.ConfigPath) {
		t.Fatalf("unexpected sing-box args log:\n%s", string(args))
	}
	firstPID := manager.cmd.Process.Pid
	if _, err := manager.Configure([]convert.Node{{Name: "Edge", Protocol: "vless", Server: "edge.example", Port: 443, Params: map[string]string{"uuid": "uuid"}}}); err != nil {
		t.Fatalf("second Configure returned error: %v", err)
	}
	if err := manager.Start(); err != nil {
		t.Fatalf("second Start returned error: %v", err)
	}
	if manager.cmd.Process.Pid == firstPID {
		t.Fatal("expected Start to restart sing-box process")
	}
	if err := syscall.Kill(firstPID, 0); err == nil {
		t.Fatal("expected old sing-box process to be stopped")
	}
}

func TestManagerStartReturnsEarlyExitOutput(t *testing.T) {
	dir := t.TempDir()
	binPath := filepath.Join(dir, "fake-sing-box")
	script := "#!/bin/sh\necho 'bad config' >&2\nexit 1\n"
	if err := os.WriteFile(binPath, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	manager := NewManager(config.VLESSRelayConfig{
		Enabled:     true,
		PublicHost:  "proxy.example.com",
		ListenHost:  "0.0.0.0",
		PortStart:   19000,
		PortEnd:     19000,
		Password:    "secret",
		SingBoxPath: binPath,
		ConfigPath:  filepath.Join(dir, "sing-box.json"),
	})
	if _, err := manager.Configure([]convert.Node{{Name: "Edge", Protocol: "vless", Server: "edge.example", Port: 443, Params: map[string]string{"uuid": "uuid"}}}); err != nil {
		t.Fatalf("Configure returned error: %v", err)
	}
	err := manager.Start()
	if err == nil || !strings.Contains(err.Error(), "sing-box exited") || !strings.Contains(err.Error(), "bad config") {
		t.Fatalf("error = %v, want early exit output", err)
	}
}

func TestManagerStartWritesProcessLogs(t *testing.T) {
	dir := t.TempDir()
	binPath := filepath.Join(dir, "fake-sing-box")
	script := "#!/bin/sh\necho 'relay ready' >&2\nexec tail -f /dev/null\n"
	if err := os.WriteFile(binPath, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	var logs bytes.Buffer
	manager := NewManager(config.VLESSRelayConfig{
		Enabled:     true,
		PublicHost:  "proxy.example.com",
		ListenHost:  "0.0.0.0",
		PortStart:   19000,
		PortEnd:     19000,
		Password:    "secret",
		SingBoxPath: binPath,
		ConfigPath:  filepath.Join(dir, "sing-box.json"),
	})
	manager.SetLogOutput(&logs)
	if _, err := manager.Configure([]convert.Node{{Name: "Edge", Protocol: "vless", Server: "edge.example", Port: 443, Params: map[string]string{"uuid": "uuid"}}}); err != nil {
		t.Fatalf("Configure returned error: %v", err)
	}
	if err := manager.Start(); err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	defer manager.Stop()
	if !waitForString(&logs, "relay ready") {
		t.Fatalf("expected sing-box output in logs, got %q", logs.String())
	}
}

func waitForFile(path string) ([]byte, error) {
	var lastErr error
	for range 20 {
		content, err := os.ReadFile(path)
		if err == nil {
			return content, nil
		}
		lastErr = err
		time.Sleep(25 * time.Millisecond)
	}
	return nil, lastErr
}

func waitForString(buf *bytes.Buffer, value string) bool {
	for range 20 {
		if strings.Contains(buf.String(), value) {
			return true
		}
		time.Sleep(25 * time.Millisecond)
	}
	return false
}

func TestConvertVLESSSimpleTLS(t *testing.T) {
	_, err := New("").ConvertVLESS(convert.Node{
		Name:     "Edge",
		Protocol: "vless",
		Server:   "edge.example.com",
		Port:     443,
		Params: map[string]string{
			"uuid": "f47ac10b-58cc-4372-a567-0e02b2c3d479",
			"tls":  "tls",
			"sni":  "sni.example.com",
		},
	})
	if !errors.Is(err, ErrStaticVLESSUnsupported) {
		t.Fatalf("error = %v, want ErrStaticVLESSUnsupported", err)
	}
}

func TestManagerReplacesVLESSWithSocks5RelayNode(t *testing.T) {
	manager := NewManager(config.VLESSRelayConfig{
		Enabled:    true,
		PublicHost: "proxy.example.com",
		ListenHost: "0.0.0.0",
		PortStart:  19000,
		PortEnd:    19010,
		Username:   "relay",
		Password:   "secret",
		ConfigPath: filepath.Join(t.TempDir(), "sing-box.json"),
	})
	nodes, err := manager.Configure([]convert.Node{
		{Name: "Remote", Protocol: "ss", Server: "remote.example", Port: 8388, Params: map[string]string{"cipher": "aes-256-gcm", "password": "pass"}},
		{
			Name:     "VLESS Edge",
			Protocol: "vless",
			Server:   "edge.example",
			Port:     443,
			Params: map[string]string{
				"uuid":               "f47ac10b-58cc-4372-a567-0e02b2c3d479",
				"tls":                "tls",
				"sni":                "sni.example",
				"network":            "ws",
				"ws_path":            "/proxy",
				"ws_host":            "cdn.example",
				"client_fingerprint": "chrome",
				"reality_public_key": "public-key-value",
				"reality_short_id":   "short-id-value",
				"alpn":               "h2,http/1.1",
				"skip_cert_verify":   "true",
			},
		},
	})
	if err != nil {
		t.Fatalf("Configure returned error: %v", err)
	}
	if len(nodes) != 2 {
		t.Fatalf("nodes len = %d, want 2: %#v", len(nodes), nodes)
	}
	relay := nodes[1]
	if relay.Protocol != "socks5" || relay.Server != "proxy.example.com" || relay.Port != 19000 {
		t.Fatalf("unexpected relay node: %#v", relay)
	}
	if relay.Params["username"] != "relay" || relay.Params["password"] != "secret" || relay.Params["relay_protocol"] != "vless" {
		t.Fatalf("unexpected relay params: %#v", relay.Params)
	}
	configBytes, err := os.ReadFile(manager.cfg.ConfigPath)
	if err != nil {
		t.Fatalf("read generated config: %v", err)
	}
	var generated map[string]any
	if err := json.Unmarshal(configBytes, &generated); err != nil {
		t.Fatalf("generated config is not JSON: %v\n%s", err, string(configBytes))
	}
	configText := string(configBytes)
	for _, want := range []string{
		`"type": "socks"`,
		`"listen_port": 19000`,
		`"type": "vless"`,
		`"server": "edge.example"`,
		`"network": "tcp"`,
		`"uuid": "f47ac10b-58cc-4372-a567-0e02b2c3d479"`,
		`"server_name": "sni.example"`,
		`"insecure": true`,
		`"alpn"`,
		`"h2"`,
		`"http/1.1"`,
		`"utls"`,
		`"fingerprint": "chrome"`,
		`"reality"`,
		`"public_key": "public-key-value"`,
		`"short_id": "short-id-value"`,
		`"type": "ws"`,
		`"Host": "cdn.example"`,
		`"outbound": "vless-out-0"`,
	} {
		if !strings.Contains(configText, want) {
			t.Fatalf("generated config missing %q:\n%s", want, configText)
		}
	}
}

func TestManagerRejectsTooManyVLESSNodes(t *testing.T) {
	manager := NewManager(config.VLESSRelayConfig{
		Enabled:    true,
		PublicHost: "proxy.example.com",
		ListenHost: "0.0.0.0",
		PortStart:  19000,
		PortEnd:    19000,
		Password:   "secret",
		ConfigPath: filepath.Join(t.TempDir(), "sing-box.json"),
	})
	_, err := manager.Configure([]convert.Node{
		{Name: "A", Protocol: "vless", Server: "a.example", Port: 443, Params: map[string]string{"uuid": "a"}},
		{Name: "B", Protocol: "vless", Server: "b.example", Port: 443, Params: map[string]string{"uuid": "b"}},
	})
	if err == nil || !strings.Contains(err.Error(), "not enough VLESS relay ports") {
		t.Fatalf("error = %v, want not enough relay ports", err)
	}
}
