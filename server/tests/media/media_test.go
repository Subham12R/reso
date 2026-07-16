package media_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/subham12r/reso/internal/api/handlers"
	"github.com/subham12r/reso/internal/rooms"
)

func TestOnlyTransferredHostCanPublish(t *testing.T) {
	service := rooms.NewRoomService()
	created, err := service.CreateRoom("owner")
	if err != nil {
		t.Fatal(err)
	}
	request, err := service.CreateJoinRequest(created.Code, "guest")
	if err != nil {
		t.Fatal(err)
	}
	approved, err := service.ApproveJoinRequest(created.Room.ID, request.ID, created.OwnerSessionToken)
	if err != nil {
		t.Fatal(err)
	}
	_, guestID, err := service.AuthorizeRoomSessionIdentity(created.Room.ID, approved.SessionToken)
	if err != nil {
		t.Fatal(err)
	}

	transfer := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"participantId":"`+guestID+`"}`))
	transfer.SetPathValue("roomId", created.Room.ID)
	transfer.AddCookie(&http.Cookie{Name: "reso_owner_session", Value: created.OwnerSessionToken})
	recorder := httptest.NewRecorder()
	handlers.NewTransferStreamHostHandler(service).ServeHTTP(recorder, transfer)
	if recorder.Code != http.StatusOK {
		t.Fatalf("transfer status = %d", recorder.Code)
	}

	room, err := service.RoomState(created.Room.ID)
	if err != nil {
		t.Fatal(err)
	}
	if room.StreamHostSessionHash == room.OwnerSessionHash {
		t.Fatal("owner remained host")
	}
}
