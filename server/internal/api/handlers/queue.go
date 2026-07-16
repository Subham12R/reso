package handlers

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/subham12r/reso/internal/queue"
)

func NewQueueJoinHandler(service *queue.Service) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		session, token, err := service.Join(r.Context())
		if err != nil {
			http.Error(w, "queue unavailable", http.StatusServiceUnavailable)
			return
		}
		http.SetCookie(w, &http.Cookie{Name: "reso_queue_session", Value: token, Path: "/", HttpOnly: true, Secure: true, SameSite: http.SameSiteLaxMode})
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(session)
	})
}

func NewQueueStatusHandler(service *queue.Service) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("reso_queue_session")
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

func queueAction(action func(context.Context, string, string) error) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("reso_queue_session")
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
