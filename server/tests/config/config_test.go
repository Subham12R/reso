package config_test

import (
	"os"
	"testing"

	"github.com/subham12r/reso/internal/config"
)

func TestLoadReadsRedisURL(t *testing.T) {
	t.Setenv("REDIS_URL", "redis://127.0.0.1:6379/0")

	got, err := config.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if got.RedisURL != "redis://127.0.0.1:6379/0" {
		t.Fatalf("RedisURL = %q", got.RedisURL)
	}
}

func TestLoadReadsRedisURLFromDotEnv(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := os.WriteFile(".env", []byte("REDIS_URL=redis://127.0.0.1:6379/1\n"), 0o600); err != nil {
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
