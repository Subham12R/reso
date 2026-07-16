package api

import (
	"net/http"

	"github.com/subham12r/reso/internal/api/handlers"
	"github.com/subham12r/reso/internal/rooms"
)

func NewRouter(roomService *rooms.RoomService) http.Handler {
	router := http.NewServeMux()

	router.Handle("GET /health", handlers.NewHealthHandler())
	router.Handle("GET /api/v1/rooms/{roomId}/join-requests", handlers.NewListPendingJoinRequestsHandler(roomService))
	router.Handle("POST /api/v1/rooms", handlers.NewRoomHandler(roomService))
	router.Handle("POST /api/v1/rooms/join-requests", handlers.NewJoinRequestHandler(roomService))
	router.Handle("POST /api/v1/rooms/{roomId}/join-requests/{requestId}/approve", handlers.NewApproveJoinRequestHandler(roomService))
	router.Handle("POST /api/v1/rooms/{roomId}/join-requests/{requestId}/reject", handlers.NewRejectJoinRequestHandler(roomService))
	return router
}
