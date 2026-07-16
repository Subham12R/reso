package media_test

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/subham12r/reso/internal/api/handlers"
	"github.com/subham12r/reso/internal/media"
	"github.com/subham12r/reso/internal/rooms"
)

func TestTokenCarriesParticipantName(t *testing.T) {
	token, err := media.IssueToken("key", "secret", "room", "identity", "Alex", false)
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.Split(token, ".")
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		t.Fatal(err)
	}
	var claims struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil {
		t.Fatal(err)
	}
	if claims.Name != "Alex" {
		t.Fatalf("name = %q, want Alex", claims.Name)
	}
}

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
