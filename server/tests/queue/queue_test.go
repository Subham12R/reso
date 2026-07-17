package queue_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/subham12r/ruse/internal/api"
	"github.com/subham12r/ruse/internal/api/handlers"
	"github.com/subham12r/ruse/internal/queue"
	"github.com/subham12r/ruse/internal/rooms"
)

func TestClaimCreatesRoomFromCallersReservation(t *testing.T) {
	client := redisClient(t)
	roomService := rooms.NewRoomServiceWithStore(rooms.NewRedisStore(client))
	queueService := queue.NewService(client)

	created := make([]rooms.CreatedRoom, 3)
	for i := range created {
		var err error
		created[i], err = roomService.CreateRoom("Owner")
		if err != nil {
			t.Fatalf("CreateRoom() error = %v", err)
		}
	}
	session, token, err := queueService.Join(context.Background())
	if err != nil {
		t.Fatalf("Join() error = %v", err)
	}
	if _, err := roomService.EndRoom(created[0].Room.ID, created[0].OwnerSessionToken); err != nil {
		t.Fatalf("EndRoom() error = %v", err)
	}

	router := api.NewRouter(roomService, queueService, handlers.MediaConfig{})
	request := httptest.NewRequest(http.MethodPost, "/api/v1/queue/"+session.ID+"/claim", bytes.NewBufferString(`{"displayName":"Queued owner"}`))
	request.Header.Set("Content-Type", "application/json")
	request.AddCookie(&http.Cookie{Name: "ruse_queue_session", Value: token})
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d: %s", recorder.Code, http.StatusCreated, recorder.Body.String())
	}
	var response struct {
		RoomID string `json:"roomId"`
		Code   string `json:"code"`
	}
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.RoomID == "" || response.Code == "" {
		t.Fatalf("response = %#v, want room ID and code", response)
	}
	cookies := recorder.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Name != "ruse_owner_session" || cookies[0].Value == "" || !cookies[0].HttpOnly || !cookies[0].Secure {
		t.Fatalf("owner cookies = %#v, want secure owner cookie", cookies)
	}
	if _, err := queueService.Status(context.Background(), session.ID, token); !errors.Is(err, queue.ErrNotFound) {
		t.Fatalf("claimed Status() error = %v, want ErrNotFound", err)
	}
}

func TestEndingRoomPromotesOldestLiveQueueSession(t *testing.T) {
	client := redisClient(t)
	roomService := rooms.NewRoomServiceWithStore(rooms.NewRedisStore(client))
	queueService := queue.NewService(client)

	created := make([]rooms.CreatedRoom, 3)
	for i := range created {
		var err error
		created[i], err = roomService.CreateRoom("Owner")
		if err != nil {
			t.Fatalf("CreateRoom() error = %v", err)
		}
	}

	stale, staleToken, err := queueService.Join(context.Background())
	if err != nil {
		t.Fatalf("first Join() error = %v", err)
	}
	live, liveToken, err := queueService.Join(context.Background())
	if err != nil {
		t.Fatalf("second Join() error = %v", err)
	}
	if err := client.HSet(context.Background(), "ruse:queue:"+stale.ID, "heartbeat", time.Now().Add(-61*time.Second).UnixMilli()).Err(); err != nil {
		t.Fatalf("age heartbeat: %v", err)
	}

	if _, err := roomService.EndRoom(created[0].Room.ID, created[0].OwnerSessionToken); err != nil {
		t.Fatalf("EndRoom() error = %v", err)
	}

	if _, err := queueService.Status(context.Background(), stale.ID, staleToken); !errors.Is(err, queue.ErrNotFound) {
		t.Fatalf("stale Status() error = %v, want ErrNotFound", err)
	}
	status, err := queueService.Status(context.Background(), live.ID, liveToken)
	if err != nil {
		t.Fatalf("live Status() error = %v", err)
	}
	if status.Status != queue.Reserved || status.ReservationID == "" {
		t.Fatalf("live status = %#v, want reservation", status)
	}
	if remaining := time.Until(status.ExpiresAt); remaining < 4*time.Minute || remaining > 5*time.Minute {
		t.Fatalf("reservation remaining = %v, want about five minutes", remaining)
	}

	if _, err := roomService.CreateRoom("Direct creator"); !errors.Is(err, rooms.ErrRoomCapacityReached) {
		t.Fatalf("CreateRoom() error = %v, want ErrRoomCapacityReached", err)
	}
}

func TestExpiringRoomPromotesQueueSession(t *testing.T) {
	client := redisClient(t)
	store := rooms.NewRedisStore(client)
	roomService := rooms.NewRoomServiceWithStore(store)
	queueService := queue.NewService(client)
	expired := rooms.Room{ID: "expired-room", CodeHash: "expired-code", State: rooms.RoomStateActive, ExpiresAt: time.Now().Add(-time.Minute)}
	if err := store.CreateRoom(context.Background(), expired); err != nil {
		t.Fatalf("CreateRoom() error = %v", err)
	}
	session, token, err := queueService.Join(context.Background())
	if err != nil {
		t.Fatalf("Join() error = %v", err)
	}

	if _, err := roomService.RoomState(expired.ID); err != nil {
		t.Fatalf("RoomState() error = %v", err)
	}
	status, err := queueService.Status(context.Background(), session.ID, token)
	if err != nil {
		t.Fatalf("Status() error = %v", err)
	}
	if status.Status != queue.Reserved {
		t.Fatalf("status = %q, want %q", status.Status, queue.Reserved)
	}
}

func TestMemoryRoomEndReleasesCapacity(t *testing.T) {
	service := rooms.NewRoomService()
	created := make([]rooms.CreatedRoom, 3)
	for i := range created {
		var err error
		created[i], err = service.CreateRoom("Owner")
		if err != nil {
			t.Fatalf("CreateRoom() error = %v", err)
		}
	}
	if _, err := service.EndRoom(created[0].Room.ID, created[0].OwnerSessionToken); err != nil {
		t.Fatalf("EndRoom() error = %v", err)
	}
	if _, err := service.CreateRoom("Replacement"); err != nil {
		t.Fatalf("replacement CreateRoom() error = %v", err)
	}
}

func TestHeartbeatPromotesAfterReservedSessionDisconnects(t *testing.T) {
	client := redisClient(t)
	roomService := rooms.NewRoomServiceWithStore(rooms.NewRedisStore(client))
	queueService := queue.NewService(client)
	created := make([]rooms.CreatedRoom, 3)
	for i := range created {
		var err error
		created[i], err = roomService.CreateRoom("Owner")
		if err != nil {
			t.Fatalf("CreateRoom() error = %v", err)
		}
	}
	reserved, _, err := queueService.Join(context.Background())
	if err != nil {
		t.Fatalf("first Join() error = %v", err)
	}
	next, nextToken, err := queueService.Join(context.Background())
	if err != nil {
		t.Fatalf("second Join() error = %v", err)
	}
	if _, err := roomService.EndRoom(created[0].Room.ID, created[0].OwnerSessionToken); err != nil {
		t.Fatalf("EndRoom() error = %v", err)
	}
	if err := client.HSet(context.Background(), "ruse:queue:"+reserved.ID, "heartbeat", time.Now().Add(-61*time.Second).UnixMilli()).Err(); err != nil {
		t.Fatalf("age heartbeat: %v", err)
	}

	if err := queueService.Heartbeat(context.Background(), next.ID, nextToken); err != nil {
		t.Fatalf("Heartbeat() error = %v", err)
	}
	status, err := queueService.Status(context.Background(), next.ID, nextToken)
	if err != nil {
		t.Fatalf("Status() error = %v", err)
	}
	if status.Status != queue.Reserved {
		t.Fatalf("status = %q, want %q", status.Status, queue.Reserved)
	}
}

func TestCapacityPrunesUnpolledExpiredRoom(t *testing.T) {
	client := redisClient(t)
	store := rooms.NewRedisStore(client)
	ctx := context.Background()
	expired := rooms.Room{ID: "expired", CodeHash: "expired-code", State: rooms.RoomStateActive, ExpiresAt: time.Now().Add(-time.Minute)}
	if err := store.CreateRoom(ctx, expired); err != nil {
		t.Fatalf("expired CreateRoom() error = %v", err)
	}
	for i := range 2 {
		room := rooms.Room{ID: string(rune('a' + i)), CodeHash: string(rune('x' + i)), State: rooms.RoomStateActive, ExpiresAt: time.Now().Add(time.Hour)}
		if err := store.CreateRoom(ctx, room); err != nil {
			t.Fatalf("active CreateRoom() error = %v", err)
		}
	}

	replacement := rooms.Room{ID: "replacement", CodeHash: "replacement-code", State: rooms.RoomStateActive, ExpiresAt: time.Now().Add(time.Hour)}
	if err := store.CreateRoom(ctx, replacement); err != nil {
		t.Fatalf("replacement CreateRoom() error = %v", err)
	}
	got, err := store.FindRoomByID(ctx, expired.ID)
	if err != nil {
		t.Fatalf("FindRoomByID() error = %v", err)
	}
	if got.State != rooms.RoomStateEnded {
		t.Fatalf("expired state = %q, want %q", got.State, rooms.RoomStateEnded)
	}
}

func TestHeartbeatPrunesUnpolledExpiredRoomAndPromotes(t *testing.T) {
	client := redisClient(t)
	store := rooms.NewRedisStore(client)
	queueService := queue.NewService(client)
	ctx := context.Background()
	roomsToCreate := []rooms.Room{
		{ID: "expired", CodeHash: "expired-code", State: rooms.RoomStateActive, ExpiresAt: time.Now().Add(-time.Minute)},
		{ID: "active-1", CodeHash: "active-code-1", State: rooms.RoomStateActive, ExpiresAt: time.Now().Add(time.Hour)},
		{ID: "active-2", CodeHash: "active-code-2", State: rooms.RoomStateActive, ExpiresAt: time.Now().Add(time.Hour)},
	}
	for _, room := range roomsToCreate {
		if err := store.CreateRoom(ctx, room); err != nil {
			t.Fatalf("CreateRoom() error = %v", err)
		}
	}
	session, token, err := queueService.Join(ctx)
	if err != nil {
		t.Fatalf("Join() error = %v", err)
	}
	if err := queueService.Heartbeat(ctx, session.ID, token); err != nil {
		t.Fatalf("Heartbeat() error = %v", err)
	}
	status, err := queueService.Status(ctx, session.ID, token)
	if err != nil {
		t.Fatalf("Status() error = %v", err)
	}
	if status.Status != queue.Reserved {
		t.Fatalf("status = %q, want %q", status.Status, queue.Reserved)
	}
}

func TestDirectCapacityCheckPromotesQueueAfterRoomExpiry(t *testing.T) {
	client := redisClient(t)
	store := rooms.NewRedisStore(client)
	queueService := queue.NewService(client)
	ctx := context.Background()
	roomsToCreate := []rooms.Room{
		{ID: "expired", CodeHash: "expired-code", State: rooms.RoomStateActive, ExpiresAt: time.Now().Add(-time.Minute)},
		{ID: "active-1", CodeHash: "active-code-1", State: rooms.RoomStateActive, ExpiresAt: time.Now().Add(time.Hour)},
		{ID: "active-2", CodeHash: "active-code-2", State: rooms.RoomStateActive, ExpiresAt: time.Now().Add(time.Hour)},
	}
	for _, room := range roomsToCreate {
		if err := store.CreateRoom(ctx, room); err != nil {
			t.Fatalf("CreateRoom() error = %v", err)
		}
	}
	session, token, err := queueService.Join(ctx)
	if err != nil {
		t.Fatalf("Join() error = %v", err)
	}
	replacement := rooms.Room{ID: "replacement", CodeHash: "replacement-code", State: rooms.RoomStateActive, ExpiresAt: time.Now().Add(time.Hour)}
	if err := store.CreateRoom(ctx, replacement); !errors.Is(err, rooms.ErrRoomCapacityReached) {
		t.Fatalf("replacement error = %v, want ErrRoomCapacityReached", err)
	}
	status, err := queueService.Status(ctx, session.ID, token)
	if err != nil {
		t.Fatalf("Status() error = %v", err)
	}
	if status.Status != queue.Reserved {
		t.Fatalf("status = %q, want %q", status.Status, queue.Reserved)
	}
}

func TestClaimRejectsWrongExpiredAndReplayedReservations(t *testing.T) {
	t.Run("different token", func(t *testing.T) {
		_, queueService, router, session, _ := reservedQueue(t)
		_, otherToken, err := queueService.Join(context.Background())
		if err != nil {
			t.Fatalf("other Join() error = %v", err)
		}
		if status := claim(router, session.ID, otherToken).Code; status != http.StatusUnauthorized {
			t.Fatalf("status = %d, want %d", status, http.StatusUnauthorized)
		}
	})

	t.Run("expired reservation", func(t *testing.T) {
		client, _, router, session, token := reservedQueue(t)
		status, err := client.HGet(context.Background(), "ruse:queue:"+session.ID, "reservationId").Result()
		if err != nil {
			t.Fatalf("reservation ID: %v", err)
		}
		if err := client.ZAdd(context.Background(), "ruse:reservations", redis.Z{Score: float64(time.Now().Add(-time.Second).UnixMilli()), Member: status}).Err(); err != nil {
			t.Fatalf("expire reservation: %v", err)
		}
		if code := claim(router, session.ID, token).Code; code == http.StatusCreated {
			t.Fatal("expired reservation was claimed")
		}
	})

	t.Run("replay", func(t *testing.T) {
		_, _, router, session, token := reservedQueue(t)
		if status := claim(router, session.ID, token).Code; status != http.StatusCreated {
			t.Fatalf("first status = %d, want %d", status, http.StatusCreated)
		}
		if status := claim(router, session.ID, token).Code; status == http.StatusCreated {
			t.Fatal("claimed reservation was replayed")
		}
	})
}

func reservedQueue(t *testing.T) (*redis.Client, *queue.Service, http.Handler, queue.Session, string) {
	t.Helper()
	client := redisClient(t)
	roomService := rooms.NewRoomServiceWithStore(rooms.NewRedisStore(client))
	queueService := queue.NewService(client)
	created := make([]rooms.CreatedRoom, 3)
	for i := range created {
		var err error
		created[i], err = roomService.CreateRoom("Owner")
		if err != nil {
			t.Fatalf("CreateRoom() error = %v", err)
		}
	}
	session, token, err := queueService.Join(context.Background())
	if err != nil {
		t.Fatalf("Join() error = %v", err)
	}
	if _, err := roomService.EndRoom(created[0].Room.ID, created[0].OwnerSessionToken); err != nil {
		t.Fatalf("EndRoom() error = %v", err)
	}
	return client, queueService, api.NewRouter(roomService, queueService, handlers.MediaConfig{}), session, token
}

func claim(router http.Handler, sessionID, token string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(http.MethodPost, "/api/v1/queue/"+sessionID+"/claim", bytes.NewBufferString(`{"displayName":"Queued owner"}`))
	request.Header.Set("Content-Type", "application/json")
	request.AddCookie(&http.Cookie{Name: "ruse_queue_session", Value: token})
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	return recorder
}

func redisClient(t *testing.T) *redis.Client {
	t.Helper()
	redisURL := os.Getenv("REDIS_TEST_URL")
	if redisURL == "" {
		t.Skip("REDIS_TEST_URL is not set")
	}
	options, err := redis.ParseURL(redisURL)
	if err != nil {
		t.Fatalf("ParseURL() error = %v", err)
	}
	client := redis.NewClient(options)
	if err := client.FlushDB(context.Background()).Err(); err != nil {
		t.Fatalf("FlushDB() error = %v", err)
	}
	t.Cleanup(func() {
		_ = client.FlushDB(context.Background()).Err()
		_ = client.Close()
	})
	return client
}
