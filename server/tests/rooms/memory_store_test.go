package rooms_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/subham12r/reso/internal/rooms"
)
func TestMemoryStoreFindsRoomByCodeHash(t *testing.T) {
	store := rooms.NewMemoryStore()
	ctx := context.Background()

	room := rooms.Room{
		ID:       "room-1",
		CodeHash: "code-1",
	}

	if err := store.CreateRoom(ctx, room); err != nil {
		t.Fatalf("CreateRoom() error = %v", err)
	}

	got, err := store.FindRoomByCodeHash(ctx, "code-1")
	if err != nil {
		t.Fatalf("FindRoomByCodeHash() error = %v", err)
	}

	if got.ID != room.ID {
		t.Fatalf("room ID = %q, want %q", got.ID, room.ID)
	}
}

func TestMemoryStoreFindsRoomByID(t *testing.T) {
	store := rooms.NewMemoryStore()
	ctx := context.Background()

	room := rooms.Room{
		ID:       "room-1",
		CodeHash: "code-1",
	}

	if err := store.CreateRoom(ctx, room); err != nil {
		t.Fatalf("CreateRoom() error = %v", err)
	}

	got, err := store.FindRoomByID(ctx, "room-1")
	if err != nil {
		t.Fatalf("FindRoomByID() error = %v", err)
	}

	if got.CodeHash != room.CodeHash {
		t.Fatalf("code hash = %q, want %q", got.CodeHash, room.CodeHash)
	}
}
func TestMemoryStoreCreatesAndFindsJoinRequest(t *testing.T) {
	store := rooms.NewMemoryStore()
	ctx := context.Background()

	request := rooms.JoinRequest{
		ID:     "request-1",
		RoomID: "room-1",
		Name:   "Alex",
		Status: rooms.JoinRequestPending,
	}

	if err := store.CreateJoinRequest(ctx, request); err != nil {
		t.Fatalf("CreateJoinRequest() error = %v", err)
	}

	got, err := store.FindJoinRequest(ctx, request.ID)
	if err != nil {
		t.Fatalf("FindJoinRequest() error = %v", err)
	}

	if got.Name != "Alex" {
		t.Fatalf("name = %q, want %q", got.Name, "Alex")
	}
}
func TestMemoryStoreUpdatesJoinRequest(t *testing.T) {
	store := rooms.NewMemoryStore()
	ctx := context.Background()

	request := rooms.JoinRequest{
		ID:     "request-1",
		RoomID: "room-1",
		Status: rooms.JoinRequestPending,
	}

	if err := store.CreateJoinRequest(ctx, request); err != nil {
		t.Fatalf("CreateJoinRequest() error = %v", err)
	}

	request.Status = rooms.JoinRequestApproved
	if err := store.UpdateJoinRequest(ctx, request); err != nil {
		t.Fatalf("UpdateJoinRequest() error = %v", err)
	}

	got, err := store.FindJoinRequest(ctx, request.ID)
	if err != nil {
		t.Fatalf("FindJoinRequest() error = %v", err)
	}

	if got.Status != rooms.JoinRequestApproved {
		t.Fatalf("status = %q, want %q", got.Status, rooms.JoinRequestApproved)
	}
}

func TestMemoryStoreListsOnlyPendingRequestsForRoom(t *testing.T) {
	store := rooms.NewMemoryStore()
	ctx := context.Background()

	requests := []rooms.JoinRequest{
		{
			ID:     "pending",
			RoomID: "room-1",
			Status: rooms.JoinRequestPending,
		},
		{
			ID:     "approved",
			RoomID: "room-1",
			Status: rooms.JoinRequestApproved,
		},
		{
			ID:     "other-room",
			RoomID: "room-2",
			Status: rooms.JoinRequestPending,
		},
	}

	for _, request := range requests {
		if err := store.CreateJoinRequest(ctx, request); err != nil {
			t.Fatalf("CreateJoinRequest() error = %v", err)
		}
	}

	got, err := store.ListPendingJoinRequests(ctx, "room-1")
	if err != nil {
		t.Fatalf("ListPendingJoinRequests() error = %v", err)
	}

	if len(got) != 1 {
		t.Fatalf("request count = %d, want 1", len(got))
	}

	if got[0].ID != "pending" {
		t.Fatalf("request ID = %q, want %q", got[0].ID, "pending")
	}
}


func TestMemoryStoreRejectsFourthRoom(t *testing.T) {
	store := rooms.NewMemoryStore()
	ctx := context.Background()

	for number := range 3 {
		room := rooms.Room{
			ID:       fmt.Sprintf("room-%d", number),
			CodeHash: fmt.Sprintf("code-%d", number),
		}

		if err := store.CreateRoom(ctx, room); err != nil {
			t.Fatalf("CreateRoom() error = %v", err)
		}
	}

	err := store.CreateRoom(ctx, rooms.Room{
		ID:       "room-4",
		CodeHash: "code-4",
	})

	if !errors.Is(err, rooms.ErrRoomCapacityReached) {
		t.Fatalf("error = %v, want ErrRoomCapacityReached", err)
	}
}
