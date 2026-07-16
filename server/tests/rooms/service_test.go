package rooms_test

import (
	"errors"
	"github.com/subham12r/reso/internal/rooms"
	"testing"
	"time"
)

func TestRoomCreate(t *testing.T) {
	service := rooms.NewRoomService()

	created, err := service.CreateRoom("Subham")
	if err != nil {
		t.Fatalf("CreateRoom() error = %v", err)
	}

	if created.Room.OwnerName != "Subham" {
		t.Fatalf("owner name = %q, want %q", created.Room.OwnerName, "Subham")
	}

	if created.Code == "" {
		t.Fatal("room code is empty")
	}

	if created.OwnerSessionToken == "" {
		t.Fatal("owner session token is empty")
	}

	if created.Room.CodeHash == created.Code {
		t.Fatal("room stores the raw room code")
	}

	if created.Room.OwnerSessionHash == created.OwnerSessionToken {
		t.Fatal("room stores the raw owner session token")
	}
}

func TestFindRoom(t *testing.T) {
	service := rooms.NewRoomService()

	created, err := service.CreateRoom("Subham")
	if err != nil {
		t.Fatalf("CreateRoom() error = %v", err)
	}

	room, err := service.FindRoomByCode(created.Code)
	if err != nil {
		t.Fatalf("FindRoomByCode() error = %v", err)
	}

	if room.ID != created.Room.ID {
		t.Fatalf("room ID = %q, want %q", room.ID, created.Room.ID)
	}
}

func TestCreateJoinRequest(t *testing.T) {
	service := rooms.NewRoomService()

	created, err := service.CreateRoom("Subham")
	if err != nil {
		t.Fatalf("CreateRoom() error = %v", err)
	}

	request, err := service.CreateJoinRequest(created.Code, "Alex")
	if err != nil {
		t.Fatalf("CreateJoinRequest() error = %v", err)
	}

	if request.RoomID != created.Room.ID {
		t.Fatalf("room ID = %q, want %q", request.RoomID, created.Room.ID)
	}

	if request.Name != "Alex" {
		t.Fatalf("name = %q, want %q", request.Name, "Alex")
	}

	if request.Status != rooms.JoinRequestPending {
		t.Fatalf("status = %q, want %q", request.Status, rooms.JoinRequestPending)
	}

	if request.ExpiresAt.Sub(request.CreatedAt) != 2*time.Minute {
		t.Fatalf("request lifetime = %s, want 2m", request.ExpiresAt.Sub(request.CreatedAt))
	}
}

func TestApprove(t *testing.T) {
	service := rooms.NewRoomService()

	created, err := service.CreateRoom("Subham")
	if err != nil {
		t.Fatalf("CreateRoom() error = %v", err)
	}

	request, err := service.CreateJoinRequest(created.Code, "Alex")
	if err != nil {
		t.Fatalf("CreateJoinRequest() error = %v", err)
	}

	approved, err := service.ApproveJoinRequest(
		created.Room.ID,
		request.ID,
		created.OwnerSessionToken,
	)
	if err != nil {
		t.Fatalf("ApproveJoinRequest() error = %v", err)
	}

	if approved.SessionToken == "" {
		t.Fatal("guest session token is empty")
	}

	if approved.Request.Status != rooms.JoinRequestApproved {
		t.Fatalf(
			"status = %q, want %q",
			approved.Request.Status,
			rooms.JoinRequestApproved,
		)
	}

	if approved.Request.GuestSessionHash == approved.SessionToken {
		t.Fatal("request stores the raw guest session token")
	}
}

func TestAuthorizedGuestSessionIncludesDisplayName(t *testing.T) {
	service := rooms.NewRoomService()
	created, err := service.CreateRoom("Subham")
	if err != nil {
		t.Fatal(err)
	}
	request, err := service.CreateJoinRequest(created.Code, "Alex")
	if err != nil {
		t.Fatal(err)
	}
	approved, err := service.ApproveJoinRequest(created.Room.ID, request.ID, created.OwnerSessionToken)
	if err != nil {
		t.Fatal(err)
	}

	_, _, name, err := service.AuthorizeRoomSessionProfile(created.Room.ID, approved.SessionToken)
	if err != nil {
		t.Fatal(err)
	}
	if name != "Alex" {
		t.Fatalf("name = %q, want Alex", name)
	}
}

func TestApproveAnotherRoom(t *testing.T) {
	service := rooms.NewRoomService()

	firstRoom, err := service.CreateRoom("Subham")
	if err != nil {
		t.Fatalf("first CreateRoom() error = %v", err)
	}

	secondRoom, err := service.CreateRoom("Subham")
	if err != nil {
		t.Fatalf("second CreateRoom() error = %v", err)
	}

	request, err := service.CreateJoinRequest(secondRoom.Code, "Alex")
	if err != nil {
		t.Fatalf("CreateJoinRequest() error = %v", err)
	}

	_, err = service.ApproveJoinRequest(
		firstRoom.Room.ID,
		request.ID,
		firstRoom.OwnerSessionToken,
	)

	if !errors.Is(err, rooms.ErrJoinRequestNotFound) {
		t.Fatalf("error = %v, want ErrJoinRequestNotFound", err)
	}
}

func TestRejectRequest(t *testing.T) {
	service := rooms.NewRoomService()

	created, err := service.CreateRoom("Subham")
	if err != nil {
		t.Fatalf("CreateRoom() error = %v", err)
	}

	request, err := service.CreateJoinRequest(created.Code, "Alex")
	if err != nil {
		t.Fatalf("CreateJoinRequest() error = %v", err)
	}

	rejected, err := service.RejectJoinRequest(
		created.Room.ID,
		request.ID,
		created.OwnerSessionToken,
	)
	if err != nil {
		t.Fatalf("RejectJoinRequest() error = %v", err)
	}

	if rejected.Status != rooms.JoinRequestRejected {
		t.Fatalf(
			"status = %q, want %q",
			rejected.Status,
			rooms.JoinRequestRejected,
		)
	}
}

func TestListPendingRequests(t *testing.T) {
	service := rooms.NewRoomService()

	created, err := service.CreateRoom("Subham")
	if err != nil {
		t.Fatalf("CreateRoom() error = %v", err)
	}

	_, err = service.CreateJoinRequest(created.Code, "Alex")
	if err != nil {
		t.Fatalf("CreateJoinRequest() error = %v", err)
	}

	requests, err := service.ListPendingJoinRequests(
		created.Room.ID,
		created.OwnerSessionToken,
	)
	if err != nil {
		t.Fatalf("ListPendingJoinRequests() error = %v", err)
	}

	if len(requests) != 1 {
		t.Fatalf("request count = %d, want 1", len(requests))
	}

	if requests[0].Name != "Alex" {
		t.Fatalf("name = %q, want %q", requests[0].Name, "Alex")
	}
}

func TestCreateRoomRejectsFourthActiveRoom(t *testing.T) {
	service := rooms.NewRoomService()

	for range 3 {
		if _, err := service.CreateRoom("Subham"); err != nil {
			t.Fatalf("CreateRoom() error = %v", err)
		}
	}

	_, err := service.CreateRoom("Subham")
	if !errors.Is(err, rooms.ErrRoomCapacityReached) {
		t.Fatalf("error = %v, want ErrRoomCapacityReached", err)
	}
}
