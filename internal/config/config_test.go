package config

import "testing"

func TestLoadDefaults(t *testing.T) {
	t.Setenv("PROXYMORPH_ADDR", "")
	t.Setenv("PROXYMORPH_DATA_DIR", "")
	t.Setenv("PROXYMORPH_PUBLIC_BASE_URL", "")
	t.Setenv("PROXYMORPH_ADMIN_USERNAME", "")
	t.Setenv("PROXYMORPH_ADMIN_PASSWORD", "")
	t.Setenv("PROXYMORPH_SESSION_SECRET", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.Addr != ":8080" {
		t.Fatalf("Addr = %q, want :8080", cfg.Addr)
	}
	if cfg.DataDir != "/data" {
		t.Fatalf("DataDir = %q, want /data", cfg.DataDir)
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
}
