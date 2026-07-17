package redisclient_test

import (
	"testing"

	"github.com/subham12r/ruse/internal/redisclient"
)

func TestNewParsesRedisURL(t *testing.T) {
	client, err := redisclient.New("redis://127.0.0.1:6379/0")
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer client.Close()

	if client.Options().Addr != "127.0.0.1:6379" {
		t.Fatalf("address = %q", client.Options().Addr)
	}
}
