package realtime_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/subham12r/reso/internal/api"
	"github.com/subham12r/reso/internal/api/handlers"
	"github.com/subham12r/reso/internal/realtime"
	"github.com/subham12r/reso/internal/rooms"
	"golang.org/x/net/websocket"
)

func TestRouterPreservesWebSocketUpgrade(t *testing.T) {
	service := rooms.NewRoomService()
	created, err := service.CreateRoom("Owner")
	if err != nil {
		t.Fatal(err)
	}
	hub := realtime.NewHubWithPresence(&memoryPresence{})
	server := httptest.NewServer(api.NewRouterWithOptions(service, nil, handlers.MediaConfig{}, api.RouterOptions{
		Realtime:       hub,
		AllowedOrigins: []string{"http://localhost:5173"},
	}))
	defer server.Close()

	config, err := websocket.NewConfig("ws"+strings.TrimPrefix(server.URL, "http")+"/api/v1/rooms/"+created.Room.ID+"/events", "http://localhost:5173")
	if err != nil {
		t.Fatal(err)
	}
	config.Header.Set("Cookie", (&http.Cookie{Name: "reso_owner_session", Value: created.OwnerSessionToken}).String())
	connection, err := websocket.DialConfig(config)
	if err != nil {
		t.Fatalf("upgrade through router: %v", err)
	}
	defer connection.Close()
}
