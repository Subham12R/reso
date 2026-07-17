package config_test

import (
	"os"
	"testing"

	"github.com/subham12r/ruse/internal/config"
)

func TestLoadReadsRedisURL(t *testing.T) {
	t.Setenv("REDIS_URL", "redis://127.0.0.1:6379/0")
	t.Setenv("TRUST_PROXY_HEADERS", "true")
	t.Setenv("COOKIE_SECURE", "false")
	t.Setenv("ALLOWED_ORIGINS", "https://app.example, http://localhost:5173, *")

	got, err := config.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if got.RedisURL != "redis://127.0.0.1:6379/0" {
		t.Fatalf("RedisURL = %q", got.RedisURL)
	}
	if !got.TrustProxyHeaders {
		t.Fatal("TrustProxyHeaders = false, want true")
	}
	if got.CookieSecure {
		t.Fatal("CookieSecure = true, want false")
	}
	if len(got.AllowedOrigins) != 2 || got.AllowedOrigins[0] != "https://app.example" || got.AllowedOrigins[1] != "http://localhost:5173" {
		t.Fatalf("AllowedOrigins = %#v", got.AllowedOrigins)
	}
}

func TestLoadDefaultsCookiesToSecure(t *testing.T) {
	t.Setenv("REDIS_URL", "redis://127.0.0.1:6379/0")
	t.Setenv("COOKIE_SECURE", "")
	t.Setenv("ALLOWED_ORIGINS", "https://app.example")

	got, err := config.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if !got.CookieSecure {
		t.Fatal("CookieSecure = false, want true")
	}
}

func TestLoadReadsPort(t *testing.T) {
	t.Setenv("REDIS_URL", "redis://127.0.0.1:6379/0")
	t.Setenv("PORT", "9090")
	t.Setenv("COOKIE_SECURE", "false")

	got, err := config.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got.Port != "9090" {
		t.Fatalf("Port = %q, want 9090", got.Port)
	}
}

func TestLoadRejectsInvalidTrustProxyHeaders(t *testing.T) {
	t.Setenv("REDIS_URL", "redis://127.0.0.1:6379/0")
	t.Setenv("TRUST_PROXY_HEADERS", "sometimes")
	if _, err := config.Load(); err == nil {
		t.Fatal("Load() error = nil, want invalid TRUST_PROXY_HEADERS error")
	}
}

func TestLoadReadsRedisURLFromDotEnv(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := os.WriteFile(".env", []byte("REDIS_URL=redis://127.0.0.1:6379/1\nCOOKIE_SECURE=false\n"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	previous, wasSet := os.LookupEnv("REDIS_URL")
	if err := os.Unsetenv("REDIS_URL"); err != nil {
		t.Fatalf("Unsetenv() error = %v", err)
	}
	t.Cleanup(func() {
		if wasSet {
			_ = os.Setenv("REDIS_URL", previous)
			return
		}
		_ = os.Unsetenv("REDIS_URL")
	})

	got, err := config.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if got.RedisURL != "redis://127.0.0.1:6379/1" {
		t.Fatalf("RedisURL = %q", got.RedisURL)
	}
}

func TestLoadRejectsSecureCookiesWithoutAllowedOrigins(t *testing.T) {
	t.Setenv("REDIS_URL", "redis://127.0.0.1:6379/0")
	t.Setenv("COOKIE_SECURE", "true")
	t.Setenv("ALLOWED_ORIGINS", "")

	if _, err := config.Load(); err == nil {
		t.Fatal("Load() error = nil, want ALLOWED_ORIGINS error")
	}
}
