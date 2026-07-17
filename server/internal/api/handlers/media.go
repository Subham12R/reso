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
		var cookie *http.Cookie
		var identity, displayName string
		for _, name := range []string{"reso_owner_session", "reso_guest_session"} {
			candidate, err := request.Cookie(name)
			if err != nil || candidate.Value == "" {
				continue
			}
			_, identity, displayName, err = service.AuthorizeRoomSessionProfile(roomID, candidate.Value)
			if err == nil {
				cookie = candidate
				break
			}
		}
		if cookie == nil {
			http.Error(writer, "unauthorized", http.StatusUnauthorized)
			return
		}

		room, err := service.RoomState(roomID)
		if err != nil {
			http.Error(writer, "unauthorized", http.StatusUnauthorized)
			return
		}
		token, err := media.IssueToken(config.APIKey, config.Secret, roomID, identity, displayName, room.StreamHostSessionHash == media.SessionHash(cookie.Value))
		if err != nil {
			http.Error(writer, "media unavailable", http.StatusServiceUnavailable)
			return
		}

		writer.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(writer).Encode(struct {
			URL        string `json:"url"`
			Token      string `json:"token"`
			CanPublish bool   `json:"canPublish"`
		}{URL: config.URL, Token: token, CanPublish: room.StreamHostSessionHash == media.SessionHash(cookie.Value)})
	})
}
