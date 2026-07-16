package realtime_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/subham12r/reso/internal/realtime"
)

func TestRedisPresenceAtomicallyLimitsRoom(t *testing.T) {
	redisURL := os.Getenv("REDIS_TEST_URL")
	if redisURL == "" {
		t.Skip("REDIS_TEST_URL is not set")
	}
	options, err := redis.ParseURL(redisURL)
	if err != nil {
		t.Fatal(err)
	}
	client := redis.NewClient(options)
	t.Cleanup(func() { _ = client.Close() })
	hub := realtime.NewHub(client)
	roomID := fmt.Sprintf("realtime-test-%d", time.Now().UnixNano())
	clients := make([]*realtime.Client, 0, 8)
	for index := 0; index < 8; index++ {
		joined, err := hub.Join(context.Background(), roomID, "participant", fmt.Sprintf("person-%d", index))
		if err != nil {
			t.Fatalf("join %d: %v", index, err)
		}
		clients = append(clients, joined)
	}
	if _, err := hub.Join(context.Background(), roomID, "participant", "ninth"); err != realtime.ErrRoomFull {
		t.Fatalf("ninth join error = %v", err)
	}
	for _, joined := range clients {
		hub.Leave(context.Background(), joined)
	}
}
