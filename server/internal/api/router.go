package api

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/redis/go-redis/v9"
	"github.com/subham12r/reso/internal/api/handlers"
	"github.com/subham12r/reso/internal/queue"
	"github.com/subham12r/reso/internal/realtime"
	"github.com/subham12r/reso/internal/rooms"
)

type RedisClient interface {
	Eval(context.Context, string, []string, ...interface{}) *redis.Cmd
	Ping(context.Context) *redis.StatusCmd
}

type RouterOptions struct {
	Redis             RedisClient
	LiveKitURL        string
	HTTPClient        *http.Client
	Logger            *slog.Logger
	TrustProxyHeaders bool
	Realtime          *realtime.Hub
	AllowedOrigins    []string
}

func NewRouter(roomService *rooms.RoomService, queueService *queue.Service, mediaConfig handlers.MediaConfig) http.Handler {
	return NewRouterWithOptions(roomService, queueService, mediaConfig, RouterOptions{})
}

func NewRouterWithOptions(roomService *rooms.RoomService, queueService *queue.Service, mediaConfig handlers.MediaConfig, options RouterOptions) http.Handler {
	router := http.NewServeMux()

	router.Handle("GET /health", handlers.NewHealthHandler())
	router.Handle("GET /ready", handlers.NewReadyHandler(options.Redis, options.LiveKitURL, options.HTTPClient))
	router.Handle("GET /api/v1/rooms/{roomId}/join-requests", handlers.NewListPendingJoinRequestsHandler(roomService))
	router.Handle("GET /api/v1/rooms/{roomId}/state", handlers.NewRoomStateHandler(roomService))
	router.Handle("POST /api/v1/rooms", validateDisplayName(handlers.NewRoomHandler(roomService)))
	router.Handle("POST /api/v1/rooms/{roomId}/end", handlers.NewEndRoomHandler(roomService, options.Realtime))
	router.Handle("POST /api/v1/rooms/{roomId}/media/token", handlers.NewMediaTokenHandler(roomService, mediaConfig))
	router.Handle("POST /api/v1/rooms/join-requests", validateDisplayName(handlers.NewJoinRequestHandler(roomService, options.Realtime)))
	router.Handle("POST /api/v1/rooms/{roomId}/join-requests/{requestId}/approve", handlers.NewApproveJoinRequestHandler(roomService, options.Realtime))
	router.Handle("POST /api/v1/rooms/{roomId}/join-requests/{requestId}/reject", handlers.NewRejectJoinRequestHandler(roomService, options.Realtime))
	if options.Realtime != nil {
		router.Handle("GET /api/v1/rooms/{roomId}/events", handlers.NewRealtimeHandler(roomService, options.Realtime, options.AllowedOrigins))
	}
	if queueService != nil {
		router.Handle("POST /api/v1/queue/join", handlers.NewQueueJoinHandler(queueService))
		router.Handle("GET /api/v1/queue/{queueSessionId}/status", handlers.NewQueueStatusHandler(queueService))
		router.Handle("POST /api/v1/queue/{queueSessionId}/heartbeat", handlers.NewQueueHeartbeatHandler(queueService))
		router.Handle("POST /api/v1/queue/{queueSessionId}/leave", handlers.NewQueueLeaveHandler(queueService))
		router.Handle("POST /api/v1/queue/{queueSessionId}/claim", validateDisplayName(handlers.NewQueueClaimHandler(roomService)))
	}
	logger := options.Logger
	if logger == nil {
		logger = slog.Default()
	}
	return requestMiddleware(limitBodies(rateLimit(router, options.Redis, options.TrustProxyHeaders)), logger)
}

func validateDisplayName(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			if isBodyTooLarge(err) {
				http.Error(w, "request too large", http.StatusRequestEntityTooLarge)
				return
			}
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}
		r.Body = io.NopCloser(bytes.NewReader(body))
		if !utf8.Valid(body) {
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}
		var input struct {
			DisplayName string `json:"displayName"`
		}
		if err := json.Unmarshal(body, &input); err != nil {
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}
		length := utf8.RuneCountInString(strings.TrimSpace(input.DisplayName))
		if length < 1 || length > 50 {
			http.Error(w, "invalid display name", http.StatusBadRequest)
			return
		}
		next.ServeHTTP(w, r)
	})
}
