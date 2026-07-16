package rooms_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/subham12r/reso/internal/rooms"
)

func TestRedisStoreRoomAndJoinRequestLifecycle(t *testing.T) {
	redisURL := os.Getenv("REDIS_TEST_URL")
	if redisURL == "" {
		t.Skip("REDIS_TEST_URL is not set")
	}

	options, err := redis.ParseURL(redisURL)
	if err != nil {
		t.Fatalf("ParseURL() error = %v", err)
	}

	client := redis.NewClient(options)
	if err := client.FlushDB(context.Background()).Err(); err != nil {
		t.Fatalf("FlushDB() error = %v", err)
	}
	t.Cleanup(func() {
		_ = client.FlushDB(context.Background()).Err()
		_ = client.Close()
	})

	store := rooms.NewRedisStore(client)
	ctx := context.Background()

	room := rooms.Room{ID: "room-1", CodeHash: "code-1"}
	if err := store.CreateRoom(ctx, room); err != nil {
		t.Fatalf("CreateRoom() error = %v", err)
	}

	gotRoom, err := store.FindRoomByCodeHash(ctx, room.CodeHash)
	if err != nil {
		t.Fatalf("FindRoomByCodeHash() error = %v", err)
	}
	if gotRoom.ID != room.ID {
		t.Fatalf("room ID = %q, want %q", gotRoom.ID, room.ID)
	}

	request := rooms.JoinRequest{
		ID:        "request-1",
		RoomID:    room.ID,
		Name:      "Alex",
		Status:    rooms.JoinRequestPending,
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(2 * time.Minute),
	}
	if err := store.CreateJoinRequest(ctx, request); err != nil {
		t.Fatalf("CreateJoinRequest() error = %v", err)
	}

	requests, err := store.ListPendingJoinRequests(ctx, room.ID)
	if err != nil {
		t.Fatalf("ListPendingJoinRequests() error = %v", err)
	}
	if len(requests) != 1 || requests[0].ID != request.ID {
		t.Fatalf("requests = %#v, want %q", requests, request.ID)
	}

	request.Status = rooms.JoinRequestApproved
	if err := store.UpdateJoinRequest(ctx, request); err != nil {
		t.Fatalf("UpdateJoinRequest() error = %v", err)
	}

	requests, err = store.ListPendingJoinRequests(ctx, room.ID)
	if err != nil {
		t.Fatalf("ListPendingJoinRequests() error = %v", err)
	}
	if len(requests) != 0 {
		t.Fatalf("request count = %d, want 0", len(requests))
	}

	for number := 2; number <= 3; number++ {
		if err := store.CreateRoom(ctx, rooms.Room{
			ID:       fmt.Sprintf("room-%d", number),
			CodeHash: fmt.Sprintf("code-%d", number),
		}); err != nil {
			t.Fatalf("CreateRoom() error = %v", err)
		}
	}

	err = store.CreateRoom(ctx, rooms.Room{ID: "room-4", CodeHash: "code-4"})
	if !errors.Is(err, rooms.ErrRoomCapacityReached) {
		t.Fatalf("error = %v, want ErrRoomCapacityReached", err)
	}
}

func TestRedisStoreUpdateMissingJoinRequest(t *testing.T) {
	redisURL := os.Getenv("REDIS_TEST_URL")
	if redisURL == "" {
		t.Skip("REDIS_TEST_URL is not set")
	}

	options, err := redis.ParseURL(redisURL)
	if err != nil {
		t.Fatalf("ParseURL() error = %v", err)
	}

	client := redis.NewClient(options)
	if err := client.FlushDB(context.Background()).Err(); err != nil {
		t.Fatalf("FlushDB() error = %v", err)
	}
	t.Cleanup(func() {
		_ = client.FlushDB(context.Background()).Err()
		_ = client.Close()
	})

	store := rooms.NewRedisStore(client)

	err = store.UpdateJoinRequest(context.Background(), rooms.JoinRequest{ID: "missing"})
	if !errors.Is(err, rooms.ErrJoinRequestNotFound) {
		t.Fatalf("UpdateJoinRequest() error = %v, want ErrJoinRequestNotFound", err)
	}
}
