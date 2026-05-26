package config

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	Addr                 string
	DataDir              string
	PublicBaseURL        string
	InitialAdminUsername string
	InitialAdminPassword string
	SessionSecret        string
	VLESSRelay           VLESSRelayConfig
}

type VLESSRelayConfig struct {
	Enabled     bool
	PublicHost  string
	ListenHost  string
	PortStart   int
	PortEnd     int
	Username    string
	Password    string
	SingBoxPath string
	ConfigPath  string
}

func Load() (Config, error) {
	publicBaseURL := os.Getenv("PROXYMORPH_PUBLIC_BASE_URL")
	dataDir := env("PROXYMORPH_DATA_DIR", "/data")
	cfg := Config{
		Addr:                 env("PROXYMORPH_ADDR", ":8080"),
		DataDir:              dataDir,
		PublicBaseURL:        publicBaseURL,
		InitialAdminUsername: env("PROXYMORPH_ADMIN_USERNAME", "admin"),
		InitialAdminPassword: os.Getenv("PROXYMORPH_ADMIN_PASSWORD"),
		SessionSecret:        os.Getenv("PROXYMORPH_SESSION_SECRET"),
	}
	if cfg.SessionSecret == "" {
		cfg.SessionSecret = randomSecret()
	}
	relay, err := loadVLESSRelayConfig(publicBaseURL, dataDir)
	if err != nil {
		return Config{}, err
	}
	cfg.VLESSRelay = relay
	return cfg, nil
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func randomSecret() string {
	var buf [32]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "development-session-secret-change-me"
	}
	return base64.RawURLEncoding.EncodeToString(buf[:])
}

func loadVLESSRelayConfig(publicBaseURL, dataDir string) (VLESSRelayConfig, error) {
	portStart, err := envInt("PROXYMORPH_VLESS_RELAY_PORT_START", 18000)
	if err != nil {
		return VLESSRelayConfig{}, err
	}
	portEnd, err := envInt("PROXYMORPH_VLESS_RELAY_PORT_END", 18099)
	if err != nil {
		return VLESSRelayConfig{}, err
	}
	if portEnd < portStart {
		return VLESSRelayConfig{}, fmt.Errorf("PROXYMORPH_VLESS_RELAY_PORT_END must be greater than or equal to PROXYMORPH_VLESS_RELAY_PORT_START")
	}
	return VLESSRelayConfig{
		Enabled:     envBool("PROXYMORPH_VLESS_RELAY_ENABLED", false),
		PublicHost:  env("PROXYMORPH_VLESS_RELAY_PUBLIC_HOST", publicHostFromBaseURL(publicBaseURL)),
		ListenHost:  env("PROXYMORPH_VLESS_RELAY_LISTEN_HOST", "0.0.0.0"),
		PortStart:   portStart,
		PortEnd:     portEnd,
		Username:    env("PROXYMORPH_VLESS_RELAY_USERNAME", "proxymorph"),
		Password:    os.Getenv("PROXYMORPH_VLESS_RELAY_PASSWORD"),
		SingBoxPath: env("PROXYMORPH_SING_BOX_PATH", "sing-box"),
		ConfigPath:  env("PROXYMORPH_SING_BOX_CONFIG_PATH", strings.TrimRight(dataDir, "/")+"/sing-box.json"),
	}, nil
}

func envBool(key string, fallback bool) bool {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value == "1" || strings.EqualFold(value, "true") || strings.EqualFold(value, "yes")
}

func envInt(key string, fallback int) (int, error) {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer: %w", key, err)
	}
	return parsed, nil
}

func publicHostFromBaseURL(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Hostname() == "" {
		return ""
	}
	return parsed.Hostname()
}
