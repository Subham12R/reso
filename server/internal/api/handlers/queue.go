package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/subham12r/ruse/internal/queue"
	"github.com/subham12r/ruse/internal/rooms"
)

func NewQueueJoinHandler(service *queue.Service) http.Handler {
	return NewQueueJoinHandlerWithCookieSecure(service, true)
}

func NewQueueJoinHandlerWithCookieSecure(service *queue.Service, cookieSecure bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		session, token, err := service.Join(r.Context())
		if err != nil {
			http.Error(w, "queue unavailable", http.StatusServiceUnavailable)
			return
		}
		http.SetCookie(w, &http.Cookie{Name: "ruse_queue_session", Value: token, Path: "/", HttpOnly: true, Secure: cookieSecure, SameSite: http.SameSiteLaxMode})
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(session)
	})
}

func NewQueueStatusHandler(service *queue.Service) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("ruse_queue_session")
		if err != nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		session, err := service.Status(r.Context(), r.PathValue("queueSessionId"), cookie.Value)
		if err != nil {
			http.Error(w, "queue session unavailable", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(session)
	})
}

func NewQueueHeartbeatHandler(service *queue.Service) http.Handler {
	return queueAction(service.Heartbeat)
}
func NewQueueLeaveHandler(service *queue.Service) http.Handler {
	return queueAction(service.Leave)
}

func NewQueueClaimHandler(service *rooms.RoomService) http.Handler {
	return NewQueueClaimHandlerWithCookieSecure(service, true)
}

func NewQueueClaimHandlerWithCookieSecure(service *rooms.RoomService, cookieSecure bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("ruse_queue_session")
		if err != nil || cookie.Value == "" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		var input createRoomRequest
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}
		input.DisplayName = strings.TrimSpace(input.DisplayName)
		if input.DisplayName == "" {
			http.Error(w, "display name is required", http.StatusBadRequest)
			return
		}
		created, err := service.ClaimRoom(input.DisplayName, r.PathValue("queueSessionId"), cookie.Value)
		if err != nil {
			if errors.Is(err, rooms.ErrUnauthorized) {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			http.Error(w, "reservation unavailable", http.StatusConflict)
			return
		}
		writeCreatedRoom(w, created, cookieSecure)
	})
}

func queueAction(action func(context.Context, string, string) error) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("ruse_queue_session")
		if err != nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if err := action(r.Context(), r.PathValue("queueSessionId"), cookie.Value); err != nil {
			http.Error(w, "queue session unavailable", http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
}
