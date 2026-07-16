package handlers

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/subham12r/reso/internal/media"
	"github.com/subham12r/reso/internal/realtime"
	"github.com/subham12r/reso/internal/rooms"
)

type createRoomRequest struct {
	DisplayName string `json:"displayName"`
}

type createRoomResponse struct {
	RoomID string `json:"roomId"`
	Code   string `json:"code"`
}

type joinRequestInput struct {
	Code        string `json:"code"`
	DisplayName string `json:"displayName"`
}

type joinRequestResponse struct {
	RequestID string                  `json:"requestId"`
	Status    rooms.JoinRequestStatus `json:"status"`
}

func NewJoinRequestHandler(service *rooms.RoomService, hubs ...*realtime.Hub) http.Handler {
	return NewJoinRequestHandlerWithCookieSecure(service, true, hubs...)
}

func NewJoinRequestHandlerWithCookieSecure(service *rooms.RoomService, cookieSecure bool, hubs ...*realtime.Hub) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var input joinRequestInput
		if err := json.NewDecoder(request.Body).Decode(&input); err != nil {
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}

		input.Code = strings.TrimSpace(input.Code)
		input.DisplayName = strings.TrimSpace(input.DisplayName)
		if input.Code == "" || input.DisplayName == "" {
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}

		guestToken, err := generateGuestToken()
		if err != nil {
			http.Error(w, "join request unavailable", http.StatusInternalServerError)
			return
		}
		joinRequest, err := service.CreateGuestJoinRequest(
			input.Code,
			input.DisplayName,
			guestToken,
		)
		if err != nil {
			http.Error(
				w,
				"invalid or unavailable room",
				http.StatusNotFound,
			)
			return
		}
		publish(hubs, joinRequest.RoomID, "join.requested", joinRequest.ID, map[string]any{"requestId": joinRequest.ID, "name": joinRequest.Name, "status": joinRequest.Status})

		w.Header().Set("Content-Type", "application/json")
		http.SetCookie(w, guestJoinTicketCookie(guestToken, cookieSecure))
		w.WriteHeader(http.StatusAccepted)

		_ = json.NewEncoder(w).Encode(joinRequestResponse{
			RequestID: joinRequest.ID,
			Status:    joinRequest.Status,
		})
	})
}

func NewRoomHandler(service *rooms.RoomService) http.Handler {
	return NewRoomHandlerWithCookieSecure(service, true)
}

func NewRoomHandlerWithCookieSecure(service *rooms.RoomService, cookieSecure bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var input createRoomRequest
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			http.Error(w, "Invalid request", http.StatusBadRequest)
			return
		}
		input.DisplayName = strings.TrimSpace(input.DisplayName)
		if input.DisplayName == "" {
			http.Error(w, "Display name is required", http.StatusBadRequest)
			return
		}
		created, err := service.CreateRoom(input.DisplayName)
		if err != nil {
			if errors.Is(err, rooms.ErrRoomCapacityReached) {
				http.Error(w, "all room slots are occupied", http.StatusConflict)
				return
			}
			slog.Error("create room failed", "error", err)
			http.Error(w, "Failed to create room", http.StatusInternalServerError)
			return
		}

		writeCreatedRoom(w, created, cookieSecure)
	})
}

func writeCreatedRoom(w http.ResponseWriter, created rooms.CreatedRoom, cookieSecure ...bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     "reso_owner_session",
		Value:    created.OwnerSessionToken,
		Path:     "/",
		HttpOnly: true,
		Secure:   secureCookie(cookieSecure),
		SameSite: http.SameSiteLaxMode,
	})
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(createRoomResponse{RoomID: created.Room.ID, Code: created.Code})
}

func NewApproveJoinRequestHandler(service *rooms.RoomService, hubs ...*realtime.Hub) http.Handler {
	return NewApproveJoinRequestHandlerWithCookieSecure(service, true, hubs...)
}

func NewApproveJoinRequestHandlerWithCookieSecure(service *rooms.RoomService, cookieSecure bool, hubs ...*realtime.Hub) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		ownerCookie, err := request.Cookie("reso_owner_session")
		if err != nil || ownerCookie.Value == "" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		approved, err := service.ApproveJoinRequest(
			request.PathValue("roomId"),
			request.PathValue("requestId"),
			ownerCookie.Value,
		)
		if err != nil {
			http.Error(w, "approval unavailable", http.StatusNotFound)
			return
		}
		publish(hubs, approved.Request.RoomID, "join.approved", approved.Request.ID, map[string]any{"requestId": approved.Request.ID, "name": approved.Request.Name, "status": approved.Request.Status})

		if approved.SessionToken != "" {
			http.SetCookie(w, guestSessionCookie(approved.SessionToken, cookieSecure))
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		_ = json.NewEncoder(w).Encode(struct {
			Status string `json:"status"`
		}{
			Status: "approved",
		})
	})
}

func NewGuestJoinStatusHandler(service *rooms.RoomService) http.Handler {
	return NewGuestJoinStatusHandlerWithCookieSecure(service, true)
}

func NewGuestJoinStatusHandlerWithCookieSecure(service *rooms.RoomService, cookieSecure bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		ticket, err := request.Cookie("reso_join_ticket")
		if err != nil || ticket.Value == "" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		joinRequest, err := service.GuestJoinRequestStatus(request.PathValue("requestId"), ticket.Value)
		if err != nil {
			http.Error(w, "join request unavailable", http.StatusNotFound)
			return
		}
		if joinRequest.Status == rooms.JoinRequestApproved {
			http.SetCookie(w, guestSessionCookie(ticket.Value, cookieSecure))
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(struct {
			Status rooms.JoinRequestStatus `json:"status"`
			RoomID string                  `json:"roomId,omitempty"`
		}{Status: joinRequest.Status, RoomID: joinRequest.RoomID})
	})
}

func generateGuestToken() (string, error) {
	return rooms.NewGuestSessionToken()
}

func guestJoinTicketCookie(token string, cookieSecure bool) *http.Cookie {
	return &http.Cookie{Name: "reso_join_ticket", Value: token, Path: "/", HttpOnly: true, Secure: cookieSecure, SameSite: http.SameSiteLaxMode}
}

func guestSessionCookie(token string, cookieSecure bool) *http.Cookie {
	return &http.Cookie{Name: "reso_guest_session", Value: token, Path: "/", HttpOnly: true, Secure: cookieSecure, SameSite: http.SameSiteLaxMode}
}

func secureCookie(values []bool) bool {
	return len(values) == 0 || values[0]
}

func NewRejectJoinRequestHandler(service *rooms.RoomService, hubs ...*realtime.Hub) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		ownerCookie, err := request.Cookie("reso_owner_session")
		if err != nil || ownerCookie.Value == "" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		rejected, err := service.RejectJoinRequest(
			request.PathValue("roomId"),
			request.PathValue("requestId"),
			ownerCookie.Value,
		)
		if err != nil {
			http.Error(w, "rejection unavailable", http.StatusNotFound)
			return
		}
		publish(hubs, rejected.RoomID, "join.rejected", rejected.ID, map[string]any{"requestId": rejected.ID, "name": rejected.Name, "status": rejected.Status})

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		_ = json.NewEncoder(w).Encode(struct {
			Status rooms.JoinRequestStatus `json:"status"`
		}{
			Status: rejected.Status,
		})
	})
}

type pendingJoinRequestResponse struct {
	ID     string                  `json:"id"`
	Name   string                  `json:"name"`
	Status rooms.JoinRequestStatus `json:"status"`
}

func NewEndRoomHandler(service *rooms.RoomService, hubs ...*realtime.Hub) http.Handler {
	return NewEndRoomHandlerWithCleaner(service, nil, hubs...)
}

func NewEndRoomHandlerWithCleaner(service *rooms.RoomService, cleaner media.RoomCleaner, hubs ...*realtime.Hub) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		ownerCookie, err := request.Cookie("reso_owner_session")
		if err != nil || ownerCookie.Value == "" {
			http.Error(writer, "unauthorized", http.StatusUnauthorized)
			return
		}

		room, err := service.EndRoom(request.PathValue("roomId"), ownerCookie.Value)
		if err != nil {
			http.Error(writer, "room unavailable", http.StatusNotFound)
			return
		}
		publish(hubs, room.ID, "room.ended", "", map[string]any{"roomId": room.ID, "state": room.State})
		if cleaner != nil {
			if err := cleaner.DeleteRoom(request.Context(), room.ID); err != nil && !errors.Is(err, media.ErrRoomAbsent) {
				slog.Error("livekit room cleanup failed", "roomId", room.ID, "error", err)
			}
		}

		writer.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(writer).Encode(struct {
			State rooms.RoomState `json:"state"`
		}{State: room.State})
	})
}

type transferStreamHostInput struct {
	ParticipantID string `json:"participantId"`
}

func NewTransferStreamHostHandler(service *rooms.RoomService, hubs ...*realtime.Hub) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		owner, err := request.Cookie("reso_owner_session")
		if err != nil || owner.Value == "" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		var input transferStreamHostInput
		if err := json.NewDecoder(request.Body).Decode(&input); err != nil || strings.TrimSpace(input.ParticipantID) == "" {
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}
		room, err := service.TransferStreamHost(request.PathValue("roomId"), owner.Value, input.ParticipantID)
		if err != nil {
			http.Error(w, "stream host unavailable", http.StatusNotFound)
			return
		}
		publish(hubs, room.ID, "stream.host.changed", "", map[string]any{"participantId": input.ParticipantID})
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"participantId": input.ParticipantID})
	})
}

func publish(hubs []*realtime.Hub, roomID, eventType, requestID string, payload any) {
	if len(hubs) != 0 && hubs[0] != nil {
		hubs[0].Publish(roomID, eventType, requestID, payload)
	}
}

func NewRoomStateHandler(service *rooms.RoomService) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		room, err := service.RoomState(request.PathValue("roomId"))
		if err != nil {
			http.Error(writer, "room unavailable", http.StatusNotFound)
			return
		}

		writer.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(writer).Encode(struct {
			ID        string          `json:"roomId"`
			State     rooms.RoomState `json:"state"`
			ExpiresAt time.Time       `json:"expiresAt"`
		}{ID: room.ID, State: room.State, ExpiresAt: room.ExpiresAt})
	})
}

func NewListPendingJoinRequestsHandler(
	service *rooms.RoomService,
) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet {
			http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		ownerCookie, err := request.Cookie("reso_owner_session")
		if err != nil || ownerCookie.Value == "" {
			http.Error(writer, "unauthorized", http.StatusUnauthorized)
			return
		}

		requests, err := service.ListPendingJoinRequests(
			request.PathValue("roomId"),
			ownerCookie.Value,
		)
		if err != nil {
			http.Error(writer, "requests unavailable", http.StatusNotFound)
			return
		}

		response := make([]pendingJoinRequestResponse, 0, len(requests))
		for _, joinRequest := range requests {
			response = append(response, pendingJoinRequestResponse{
				ID:     joinRequest.ID,
				Name:   joinRequest.Name,
				Status: joinRequest.Status,
			})
		}

		writer.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(writer).Encode(struct {
			Requests []pendingJoinRequestResponse `json:"requests"`
		}{
			Requests: response,
		})
	})
}
