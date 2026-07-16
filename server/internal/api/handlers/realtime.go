package handlers

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/subham12r/reso/internal/realtime"
	"github.com/subham12r/reso/internal/rooms"
)

const (
	realtimeReadLimit = 4096
	realtimeTimeout   = 60 * time.Second
	writeTimeout      = 10 * time.Second
)

type clientEvent struct {
	Version   int    `json:"version"`
	Type      string `json:"type"`
	RequestID string `json:"requestId,omitempty"`
}

func NewRealtimeHandler(service *rooms.RoomService, hub *realtime.Hub, allowedOrigins []string) http.Handler {
	origins := make(map[string]struct{}, len(allowedOrigins))
	for _, origin := range allowedOrigins {
		origin = strings.TrimSpace(origin)
		if origin != "" && origin != "*" {
			origins[origin] = struct{}{}
		}
	}
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		roomID := request.PathValue("roomId")
		role, token, err := authorizeRealtime(service, roomID, request)
		if err != nil {
			http.Error(writer, "unauthorized", http.StatusUnauthorized)
			return
		}
		if _, allowed := origins[request.Header.Get("Origin")]; !allowed {
			http.Error(writer, "origin not allowed", http.StatusForbidden)
			return
		}
		connection, err := (&websocket.Upgrader{
			CheckOrigin: func(*http.Request) bool { return true },
		}).Upgrade(writer, request, nil)
		if err != nil {
			return
		}
		serveRealtime(connection, service, hub, request, roomID, string(role), stableIdentity(role, token))
	})
}

func authorizeRealtime(service *rooms.RoomService, roomID string, request *http.Request) (rooms.SessionRole, string, error) {
	for _, name := range []string{"reso_owner_session", "reso_guest_session"} {
		cookie, err := request.Cookie(name)
		if err != nil || cookie.Value == "" {
			continue
		}
		role, err := service.AuthorizeRoomSession(roomID, cookie.Value)
		if err == nil {
			return role, cookie.Value, nil
		}
	}
	return "", "", rooms.ErrUnauthorized
}

func stableIdentity(role rooms.SessionRole, token string) string {
	digest := sha256.Sum256([]byte(token))
	return string(role) + ":" + hex.EncodeToString(digest[:12])
}

func serveRealtime(connection *websocket.Conn, service *rooms.RoomService, hub *realtime.Hub, request *http.Request, roomID, role, identity string) {
	connection.SetReadLimit(realtimeReadLimit)
	_ = connection.SetReadDeadline(time.Now().Add(realtimeTimeout))
	connection.SetPongHandler(func(string) error {
		return connection.SetReadDeadline(time.Now().Add(realtimeTimeout))
	})
	client, err := hub.Join(request.Context(), roomID, role, identity)
	if err != nil {
		_ = connection.Close()
		return
	}
	defer hub.Leave(request.Context(), client)

	writerDone := make(chan struct{})
	go func() {
		defer close(writerDone)
		ping := time.NewTicker(realtimeTimeout / 2)
		defer ping.Stop()
		for {
			select {
			case <-client.Done():
				_ = connection.Close()
				return
			case event := <-client.Events():
				_ = connection.SetWriteDeadline(time.Now().Add(writeTimeout))
				if err := connection.WriteJSON(event); err != nil {
					client.Close()
					return
				}
			case <-ping.C:
				_ = connection.SetWriteDeadline(time.Now().Add(writeTimeout))
				if err := connection.WriteMessage(websocket.PingMessage, nil); err != nil {
					client.Close()
					return
				}
			}
		}
	}()

	for {
		var event clientEvent
		if err := connection.ReadJSON(&event); err != nil {
			break
		}
		if event.Version != 1 {
			break
		}
		switch event.Type {
		case "presence.heartbeat":
			if err := hub.Heartbeat(request.Context(), client); err != nil {
				client.Close()
			}
		case "room.state.request":
			room, err := service.RoomState(roomID)
			if err != nil {
				client.Close()
				break
			}
			client.Send(realtime.NewEnvelope("room.state", event.RequestID, map[string]any{"roomId": room.ID, "state": room.State, "expiresAt": room.ExpiresAt.UTC().Format(time.RFC3339Nano)}))
		default:
			client.Close()
		}
		select {
		case <-client.Done():
			break
		default:
			continue
		}
		break
	}
	client.Close()
	_ = connection.Close()
	select {
	case <-writerDone:
	case <-time.After(writeTimeout):
	}
}
