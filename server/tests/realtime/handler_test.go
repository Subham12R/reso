package realtime_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/subham12r/ruse/internal/api/handlers"
	"github.com/subham12r/ruse/internal/realtime"
	"github.com/subham12r/ruse/internal/rooms"
	"golang.org/x/net/websocket"
)

func TestRealtimeRejectsUnauthorizedAndDisallowedOriginsBeforeUpgrade(t *testing.T) {
	service := rooms.NewRoomService()
	created, err := service.CreateRoom("Owner")
	if err != nil {
		t.Fatal(err)
	}
	hub := realtime.NewHubWithPresence(&memoryPresence{})
	handler := handlers.NewRealtimeHandler(service, hub, []string{"https://app.example"})

	request := httptest.NewRequest(http.MethodGet, "/api/v1/rooms/"+created.Room.ID+"/events", nil)
	request.SetPathValue("roomId", created.Room.ID)
	request.Header.Set("Origin", "https://app.example")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized status = %d", recorder.Code)
	}

	request = httptest.NewRequest(http.MethodGet, "/api/v1/rooms/"+created.Room.ID+"/events", nil)
	request.SetPathValue("roomId", created.Room.ID)
	request.Header.Set("Origin", "https://evil.example")
	request.AddCookie(&http.Cookie{Name: "ruse_owner_session", Value: created.OwnerSessionToken})
	recorder = httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("disallowed origin status = %d", recorder.Code)
	}
}

func TestRealtimeOwnerConnectsAndRequestsRoomState(t *testing.T) {
	service := rooms.NewRoomService()
	created, err := service.CreateRoom("Owner")
	if err != nil {
		t.Fatal(err)
	}
	hub := realtime.NewHubWithPresence(&memoryPresence{})
	mux := http.NewServeMux()
	mux.Handle("GET /api/v1/rooms/{roomId}/events", handlers.NewRealtimeHandler(service, hub, []string{"http://localhost:5173"}))
	server := httptest.NewServer(mux)
	defer server.Close()

	config, err := websocket.NewConfig("ws"+strings.TrimPrefix(server.URL, "http")+"/api/v1/rooms/"+created.Room.ID+"/events", "http://localhost:5173")
	if err != nil {
		t.Fatal(err)
	}
	config.Header.Set("Cookie", (&http.Cookie{Name: "ruse_owner_session", Value: created.OwnerSessionToken}).String())
	connection, err := websocket.DialConfig(config)
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()

	var joined realtime.Envelope
	if err := websocket.JSON.Receive(connection, &joined); err != nil {
		t.Fatal(err)
	}
	if joined.Type != "participant.joined" {
		t.Fatalf("first event type = %q", joined.Type)
	}
	if err := websocket.JSON.Send(connection, map[string]any{"version": 1, "type": "room.state.request"}); err != nil {
		t.Fatal(err)
	}
	var state realtime.Envelope
	for state.Type != "room.state" {
		if err := websocket.JSON.Receive(connection, &state); err != nil {
			t.Fatal(err)
		}
	}
	payload, ok := state.Payload.(map[string]any)
	if !ok || payload["state"] != string(rooms.RoomStateActive) {
		t.Fatalf("state payload = %#v", state.Payload)
	}
}

func TestRealtimeRejectsUnsupportedClientEvent(t *testing.T) {
	service := rooms.NewRoomService()
	created, _ := service.CreateRoom("Owner")
	hub := realtime.NewHubWithPresence(&memoryPresence{})
	mux := http.NewServeMux()
	mux.Handle("GET /api/v1/rooms/{roomId}/events", handlers.NewRealtimeHandler(service, hub, []string{"http://localhost:5173"}))
	server := httptest.NewServer(mux)
	defer server.Close()
	config, _ := websocket.NewConfig("ws"+strings.TrimPrefix(server.URL, "http")+"/api/v1/rooms/"+created.Room.ID+"/events", "http://localhost:5173")
	config.Header.Set("Cookie", (&http.Cookie{Name: "ruse_owner_session", Value: created.OwnerSessionToken}).String())
	connection, err := websocket.DialConfig(config)
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()
	for index := 0; index < 2; index++ {
		var initial realtime.Envelope
		_ = websocket.JSON.Receive(connection, &initial)
	}
	if err := websocket.JSON.Send(connection, map[string]any{"version": 1, "type": "chat.message"}); err != nil {
		t.Fatal(err)
	}
	_ = connection.SetDeadline(time.Now().Add(time.Second))
	var event realtime.Envelope
	if err := websocket.JSON.Receive(connection, &event); err == nil {
		t.Fatalf("unsupported event kept connection open: %#v", event)
	}
}

func TestJoinRequestPublishesOnlyPublicDataAfterSuccess(t *testing.T) {
	service := rooms.NewRoomService()
	created, err := service.CreateRoom("Owner")
	if err != nil {
		t.Fatal(err)
	}
	hub := realtime.NewHubWithPresence(&memoryPresence{})
	client, err := hub.Join(context.Background(), created.Room.ID, "owner", "owner-id")
	if err != nil {
		t.Fatal(err)
	}
	defer hub.Leave(context.Background(), client)
	<-client.Events()
	<-client.Events()

	body, _ := json.Marshal(map[string]string{"code": created.Code, "displayName": "Guest"})
	request := httptest.NewRequest(http.MethodPost, "/api/v1/rooms/join-requests", bytes.NewReader(body))
	recorder := httptest.NewRecorder()
	handlers.NewJoinRequestHandler(service, hub).ServeHTTP(recorder, request)
	if recorder.Code != http.StatusAccepted {
		t.Fatalf("status = %d", recorder.Code)
	}
	event := <-client.Events()
	encoded, _ := json.Marshal(event)
	if event.Type != "join.requested" || strings.Contains(string(encoded), created.OwnerSessionToken) || strings.Contains(string(encoded), created.Code) {
		t.Fatalf("published event = %s", encoded)
	}
}

func TestApprovedGuestCanUpgrade(t *testing.T) {
	service := rooms.NewRoomService()
	created, err := service.CreateRoom("Owner")
	if err != nil {
		t.Fatal(err)
	}
	joinRequest, err := service.CreateJoinRequest(created.Code, "Guest")
	if err != nil {
		t.Fatal(err)
	}
	approved, err := service.ApproveJoinRequest(created.Room.ID, joinRequest.ID, created.OwnerSessionToken)
	if err != nil {
		t.Fatal(err)
	}
	hub := realtime.NewHubWithPresence(&memoryPresence{})
	mux := http.NewServeMux()
	mux.Handle("GET /api/v1/rooms/{roomId}/events", handlers.NewRealtimeHandler(service, hub, []string{"http://localhost:5173"}))
	server := httptest.NewServer(mux)
	defer server.Close()

	config, _ := websocket.NewConfig("ws"+strings.TrimPrefix(server.URL, "http")+"/api/v1/rooms/"+created.Room.ID+"/events", "http://localhost:5173")
	config.Header.Set("Cookie", (&http.Cookie{Name: "ruse_guest_session", Value: approved.SessionToken}).String())
	connection, err := websocket.DialConfig(config)
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()
	var joined realtime.Envelope
	if err := websocket.JSON.Receive(connection, &joined); err != nil {
		t.Fatal(err)
	}
	payload, ok := joined.Payload.(map[string]any)
	if joined.Type != "participant.joined" || !ok || payload["role"] != "participant" {
		t.Fatalf("guest join event = %#v", joined)
	}
}

func TestRoomMutationHandlersPublishAfterSuccess(t *testing.T) {
	service := rooms.NewRoomService()
	created, err := service.CreateRoom("Owner")
	if err != nil {
		t.Fatal(err)
	}
	hub := realtime.NewHubWithPresence(&memoryPresence{})
	owner, err := hub.Join(context.Background(), created.Room.ID, "owner", "owner-id")
	if err != nil {
		t.Fatal(err)
	}
	defer hub.Leave(context.Background(), owner)
	<-owner.Events()
	<-owner.Events()

	approveRequest, _ := service.CreateJoinRequest(created.Code, "Approve")
	request := httptest.NewRequest(http.MethodPost, "/approve", nil)
	request.SetPathValue("roomId", created.Room.ID)
	request.SetPathValue("requestId", approveRequest.ID)
	request.AddCookie(&http.Cookie{Name: "ruse_owner_session", Value: created.OwnerSessionToken})
	recorder := httptest.NewRecorder()
	handlers.NewApproveJoinRequestHandler(service, hub).ServeHTTP(recorder, request)
	assertEventType(t, owner, "join.approved")

	rejectRequest, _ := service.CreateJoinRequest(created.Code, "Reject")
	request = httptest.NewRequest(http.MethodPost, "/reject", nil)
	request.SetPathValue("roomId", created.Room.ID)
	request.SetPathValue("requestId", rejectRequest.ID)
	request.AddCookie(&http.Cookie{Name: "ruse_owner_session", Value: created.OwnerSessionToken})
	recorder = httptest.NewRecorder()
	handlers.NewRejectJoinRequestHandler(service, hub).ServeHTTP(recorder, request)
	assertEventType(t, owner, "join.rejected")

	request = httptest.NewRequest(http.MethodPost, "/end", nil)
	request.SetPathValue("roomId", created.Room.ID)
	request.AddCookie(&http.Cookie{Name: "ruse_owner_session", Value: created.OwnerSessionToken})
	recorder = httptest.NewRecorder()
	handlers.NewEndRoomHandler(service, hub).ServeHTTP(recorder, request)
	assertEventType(t, owner, "room.ended")
}

func assertEventType(t *testing.T, client *realtime.Client, want string) {
	t.Helper()
	select {
	case event := <-client.Events():
		if event.Type != want {
			t.Fatalf("event type = %q, want %q", event.Type, want)
		}
	case <-time.After(time.Second):
		t.Fatalf("timed out waiting for %s", want)
	}
}
