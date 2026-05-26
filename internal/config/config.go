package config

import (
	"crypto/rand"
	"encoding/base64"
	"os"
)

type Config struct {
	Addr                 string
	DataDir              string
	PublicBaseURL        string
	InitialAdminUsername string
	InitialAdminPassword string
	SessionSecret        string
}

func Load() (Config, error) {
	cfg := Config{
		Addr:                 env("PROXYMORPH_ADDR", ":8080"),
		DataDir:              env("PROXYMORPH_DATA_DIR", "/data"),
		PublicBaseURL:        os.Getenv("PROXYMORPH_PUBLIC_BASE_URL"),
		InitialAdminUsername: env("PROXYMORPH_ADMIN_USERNAME", "admin"),
		InitialAdminPassword: os.Getenv("PROXYMORPH_ADMIN_PASSWORD"),
		SessionSecret:        os.Getenv("PROXYMORPH_SESSION_SECRET"),
	}
	if cfg.SessionSecret == "" {
		cfg.SessionSecret = randomSecret()
	}
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
