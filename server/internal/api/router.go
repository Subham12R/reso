package api

import (
	"net/http"

	"github.com/subham12r/reso/internal/api/handlers"
	"github.com/subham12r/reso/internal/queue"
	"github.com/subham12r/reso/internal/rooms"
)

func NewRouter(roomService *rooms.RoomService, mediaConfig ...handlers.MediaConfig) http.Handler {
	router := http.NewServeMux()

	router.Handle("GET /health", handlers.NewHealthHandler())
	router.Handle("GET /api/v1/rooms/{roomId}/join-requests", handlers.NewListPendingJoinRequestsHandler(roomService))
	router.Handle("GET /api/v1/rooms/{roomId}/state", handlers.NewRoomStateHandler(roomService))
	router.Handle("POST /api/v1/rooms", handlers.NewRoomHandler(roomService))
	router.Handle("POST /api/v1/rooms/{roomId}/end", handlers.NewEndRoomHandler(roomService))
	if len(mediaConfig) == 1 {
		router.Handle("POST /api/v1/rooms/{roomId}/media/token", handlers.NewMediaTokenHandler(roomService, mediaConfig[0]))
	}
	router.Handle("POST /api/v1/rooms/join-requests", handlers.NewJoinRequestHandler(roomService))
	router.Handle("POST /api/v1/rooms/{roomId}/join-requests/{requestId}/approve", handlers.NewApproveJoinRequestHandler(roomService))
	router.Handle("POST /api/v1/rooms/{roomId}/join-requests/{requestId}/reject", handlers.NewRejectJoinRequestHandler(roomService))
	return router
}

func NewRouterWithQueue(roomService *rooms.RoomService, queueService *queue.Service, mediaConfig handlers.MediaConfig) http.Handler {
	router := NewRouter(roomService, mediaConfig).(*http.ServeMux)
	router.Handle("POST /api/v1/queue/join", handlers.NewQueueJoinHandler(queueService))
	router.Handle("GET /api/v1/queue/{queueSessionId}/status", handlers.NewQueueStatusHandler(queueService))
	router.Handle("POST /api/v1/queue/{queueSessionId}/heartbeat", handlers.NewQueueHeartbeatHandler(queueService))
	router.Handle("POST /api/v1/queue/{queueSessionId}/leave", handlers.NewQueueLeaveHandler(queueService))
	return router
}
