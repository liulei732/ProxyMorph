package config

import "testing"

func TestLoadDefaults(t *testing.T) {
	t.Setenv("PROXYMORPH_ADDR", "")
	t.Setenv("PROXYMORPH_DATA_DIR", "")
	t.Setenv("PROXYMORPH_PUBLIC_BASE_URL", "")
	t.Setenv("PROXYMORPH_ADMIN_USERNAME", "")
	t.Setenv("PROXYMORPH_ADMIN_PASSWORD", "")
	t.Setenv("PROXYMORPH_SESSION_SECRET", "")
	t.Setenv("PROXYMORPH_VLESS_RELAY_ENABLED", "")
	t.Setenv("PROXYMORPH_VLESS_RELAY_PUBLIC_HOST", "")
	t.Setenv("PROXYMORPH_VLESS_RELAY_LISTEN_HOST", "")
	t.Setenv("PROXYMORPH_VLESS_RELAY_PORT_START", "")
	t.Setenv("PROXYMORPH_VLESS_RELAY_PORT_END", "")
	t.Setenv("PROXYMORPH_VLESS_RELAY_USERNAME", "")
	t.Setenv("PROXYMORPH_VLESS_RELAY_PASSWORD", "")
	t.Setenv("PROXYMORPH_SING_BOX_PATH", "")
	t.Setenv("PROXYMORPH_SING_BOX_CONFIG_PATH", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.Addr != ":28888" {
		t.Fatalf("Addr = %q, want :28888", cfg.Addr)
	}
	if cfg.DataDir != "/data" {
		t.Fatalf("DataDir = %q, want /data", cfg.DataDir)
	}
	if cfg.VLESSRelay.Enabled {
		t.Fatal("VLESS relay should be disabled by default")
	}
	if cfg.VLESSRelay.PortStart != 31800 || cfg.VLESSRelay.PortEnd != 31999 {
		t.Fatalf("unexpected VLESS relay port range: %#v", cfg.VLESSRelay)
	}
}

func TestLoadRequiresSessionSecret(t *testing.T) {
	t.Setenv("PROXYMORPH_SESSION_SECRET", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load returned error for dev defaults: %v", err)
	}
	if cfg.SessionSecret == "" {
		t.Fatal("SessionSecret must not be empty")
	}
}

func TestLoadFromEnvironment(t *testing.T) {
	t.Setenv("PROXYMORPH_ADDR", ":9090")
	t.Setenv("PROXYMORPH_DATA_DIR", "/tmp/proxymorph")
	t.Setenv("PROXYMORPH_PUBLIC_BASE_URL", "https://proxy.example.test")
	t.Setenv("PROXYMORPH_ADMIN_USERNAME", "root")
	t.Setenv("PROXYMORPH_ADMIN_PASSWORD", "secret")
	t.Setenv("PROXYMORPH_SESSION_SECRET", "01234567890123456789012345678901")
	t.Setenv("PROXYMORPH_VLESS_RELAY_ENABLED", "true")
	t.Setenv("PROXYMORPH_VLESS_RELAY_PUBLIC_HOST", "")
	t.Setenv("PROXYMORPH_VLESS_RELAY_LISTEN_HOST", "127.0.0.1")
	t.Setenv("PROXYMORPH_VLESS_RELAY_PORT_START", "19000")
	t.Setenv("PROXYMORPH_VLESS_RELAY_PORT_END", "19010")
	t.Setenv("PROXYMORPH_VLESS_RELAY_USERNAME", "relay")
	t.Setenv("PROXYMORPH_VLESS_RELAY_PASSWORD", "relay-secret")
	t.Setenv("PROXYMORPH_SING_BOX_PATH", "/usr/bin/sing-box")
	t.Setenv("PROXYMORPH_SING_BOX_CONFIG_PATH", "/tmp/proxymorph/sing-box.json")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.Addr != ":9090" || cfg.DataDir != "/tmp/proxymorph" || cfg.PublicBaseURL != "https://proxy.example.test" {
		t.Fatalf("unexpected config: %#v", cfg)
	}
	if cfg.InitialAdminUsername != "root" || cfg.InitialAdminPassword != "secret" {
		t.Fatalf("unexpected admin config: %#v", cfg)
	}
	if !cfg.VLESSRelay.Enabled || cfg.VLESSRelay.PublicHost != "proxy.example.test" || cfg.VLESSRelay.ListenHost != "127.0.0.1" {
		t.Fatalf("unexpected VLESS relay host config: %#v", cfg.VLESSRelay)
	}
	if cfg.VLESSRelay.PortStart != 19000 || cfg.VLESSRelay.PortEnd != 19010 {
		t.Fatalf("unexpected VLESS relay ports: %#v", cfg.VLESSRelay)
	}
	if cfg.VLESSRelay.Username != "relay" || cfg.VLESSRelay.Password != "relay-secret" {
		t.Fatalf("unexpected VLESS relay credentials: %#v", cfg.VLESSRelay)
	}
	if cfg.VLESSRelay.SingBoxPath != "/usr/bin/sing-box" || cfg.VLESSRelay.ConfigPath != "/tmp/proxymorph/sing-box.json" {
		t.Fatalf("unexpected sing-box config: %#v", cfg.VLESSRelay)
	}
}
