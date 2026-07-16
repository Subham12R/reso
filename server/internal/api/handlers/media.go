package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/subham12r/reso/internal/media"
	"github.com/subham12r/reso/internal/rooms"
)

type MediaConfig struct {
	URL    string
	APIKey string
	Secret string
}

func NewMediaTokenHandler(service *rooms.RoomService, config MediaConfig) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if config.URL == "" || config.APIKey == "" || config.Secret == "" {
			http.Error(writer, "media unavailable", http.StatusServiceUnavailable)
			return
		}

		roomID := request.PathValue("roomId")
		cookie, err := request.Cookie("reso_owner_session")
		if err != nil {
			cookie, err = request.Cookie("reso_guest_session")
		}
		if err != nil || cookie.Value == "" {
			http.Error(writer, "unauthorized", http.StatusUnauthorized)
			return
		}

		_, identity, err := service.AuthorizeRoomSessionIdentity(roomID, cookie.Value)
		if err != nil {
			http.Error(writer, "unauthorized", http.StatusUnauthorized)
			return
		}

		room, err := service.RoomState(roomID)
		if err != nil {
			http.Error(writer, "unauthorized", http.StatusUnauthorized)
			return
		}
		token, err := media.IssueToken(config.APIKey, config.Secret, roomID, identity, room.StreamHostSessionHash == media.SessionHash(cookie.Value))
		if err != nil {
			http.Error(writer, "media unavailable", http.StatusServiceUnavailable)
			return
		}

		writer.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(writer).Encode(map[string]string{"url": config.URL, "token": token})
	})
}
