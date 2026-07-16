package api_test

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/subham12r/reso/internal/api"
	"github.com/subham12r/reso/internal/api/handlers"
	"github.com/subham12r/reso/internal/rooms"
)

type fakeRedis struct {
	mu      sync.Mutex
	counts  map[string]int64
	pingErr error
}

func (f *fakeRedis) Eval(ctx context.Context, _ string, keys []string, _ ...interface{}) *redis.Cmd {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.counts == nil {
		f.counts = make(map[string]int64)
	}
	f.counts[keys[0]]++
	cmd := redis.NewCmd(ctx)
	cmd.SetVal([]interface{}{f.counts[keys[0]], int64(60)})
	return cmd
}

func (f *fakeRedis) Ping(ctx context.Context) *redis.StatusCmd {
	cmd := redis.NewStatusCmd(ctx)
	if f.pingErr != nil {
		cmd.SetErr(f.pingErr)
	} else {
		cmd.SetVal("PONG")
	}
	return cmd
}

func TestRouterAddsRequestIDAndSecurityHeaders(t *testing.T) {
	router := api.NewRouter(rooms.NewRoomService(), nil, handlers.MediaConfig{})

	request := httptest.NewRequest(http.MethodGet, "/health", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	requestID := recorder.Header().Get("X-Request-ID")
	decoded, err := hex.DecodeString(requestID)
	if err != nil || len(decoded) != 16 {
		t.Fatalf("X-Request-ID = %q, want 16 random bytes encoded as hex", requestID)
	}
	for header, want := range map[string]string{
		"Content-Security-Policy": "default-src 'none'; frame-ancestors 'none'",
		"X-Content-Type-Options":  "nosniff",
		"Referrer-Policy":         "no-referrer",
		"Permissions-Policy":      "camera=(), microphone=(), geolocation=()",
	} {
		if got := recorder.Header().Get(header); got != want {
			t.Errorf("%s = %q, want %q", header, got, want)
		}
	}

	request = httptest.NewRequest(http.MethodGet, "/health", nil)
	request.Header.Set("X-Request-ID", "caller-request-id")
	recorder = httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	if got := recorder.Header().Get("X-Request-ID"); got != "caller-request-id" {
		t.Fatalf("propagated X-Request-ID = %q", got)
	}
}

func TestRouterReplacesUnsafeRequestIDsBeforeLogging(t *testing.T) {
	for _, unsafeID := range []string{"SECRET TOKEN", "SECRET\nTOKEN", strings.Repeat("a", 65)} {
		t.Run(unsafeID[:6], func(t *testing.T) {
			var logs bytes.Buffer
			router := api.NewRouterWithOptions(
				rooms.NewRoomService(),
				nil,
				handlers.MediaConfig{},
				api.RouterOptions{Logger: slog.New(slog.NewTextHandler(&logs, nil))},
			)
			request := httptest.NewRequest(http.MethodGet, "/health", nil)
			request.Header.Set("X-Request-ID", unsafeID)
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, request)

			requestID := recorder.Header().Get("X-Request-ID")
			decoded, err := hex.DecodeString(requestID)
			if err != nil || len(decoded) != 16 {
				t.Fatalf("replacement X-Request-ID = %q, want 16 random bytes encoded as hex", requestID)
			}
			if strings.Contains(logs.String(), unsafeID) {
				t.Fatalf("request log contains unsafe caller ID: %s", logs.String())
			}
			if !strings.Contains(logs.String(), requestID) {
				t.Fatalf("request log does not contain replacement ID %q: %s", requestID, logs.String())
			}
		})
	}
}

func TestRoomCreationRateLimitUsesRemoteAddrAndReturnsRetryAfter(t *testing.T) {
	redisClient := &fakeRedis{}
	router := api.NewRouterWithOptions(rooms.NewRoomService(), nil, handlers.MediaConfig{}, api.RouterOptions{Redis: redisClient})

	for attempt := 1; attempt <= 6; attempt++ {
		request := httptest.NewRequest(http.MethodPost, "/api/v1/rooms", strings.NewReader(`{}`))
		request.RemoteAddr = "203.0.113.8:4321"
		request.Header.Set("X-Forwarded-For", "198.51.100."+string(rune('0'+attempt)))
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, request)
		if attempt == 6 {
			if recorder.Code != http.StatusTooManyRequests {
				t.Fatalf("attempt %d status = %d, want 429", attempt, recorder.Code)
			}
			if recorder.Header().Get("Retry-After") == "" {
				t.Fatal("Retry-After header is missing")
			}
		}
	}

	request := httptest.NewRequest(http.MethodPost, "/api/v1/rooms", strings.NewReader(`{}`))
	request.RemoteAddr = "203.0.113.9:4321"
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	if recorder.Code == http.StatusTooManyRequests {
		t.Fatal("different RemoteAddr shared a rate-limit bucket")
	}
}

func TestRouterRateLimitsJoinAttemptsAndQueueActions(t *testing.T) {
	for _, test := range []struct {
		name     string
		method   string
		path     string
		attempts int
	}{
		{name: "join attempts", method: http.MethodPost, path: "/api/v1/rooms/join-requests", attempts: 11},
		{name: "queue actions", method: http.MethodPost, path: "/api/v1/queue/join", attempts: 61},
	} {
		t.Run(test.name, func(t *testing.T) {
			router := api.NewRouterWithOptions(rooms.NewRoomService(), nil, handlers.MediaConfig{}, api.RouterOptions{Redis: &fakeRedis{}})
			for attempt := 1; attempt <= test.attempts; attempt++ {
				request := httptest.NewRequest(test.method, test.path, strings.NewReader(`{}`))
				request.RemoteAddr = "203.0.113.8:4321"
				recorder := httptest.NewRecorder()
				router.ServeHTTP(recorder, request)
				if attempt == test.attempts && recorder.Code != http.StatusTooManyRequests {
					t.Fatalf("attempt %d status = %d, want 429", attempt, recorder.Code)
				}
			}
		})
	}
}

func TestRouterBoundsJSONAndValidatesTrimmedUnicodeDisplayNames(t *testing.T) {
	router := api.NewRouter(rooms.NewRoomService(), nil, handlers.MediaConfig{})

	oversized := `{"displayName":"` + strings.Repeat("a", 17*1024) + `"}`
	request := httptest.NewRequest(http.MethodPost, "/api/v1/rooms", strings.NewReader(oversized))
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversized status = %d, want %d", recorder.Code, http.StatusRequestEntityTooLarge)
	}

	body, err := json.Marshal(map[string]string{"displayName": strings.Repeat("\u754c", 51)})
	if err != nil {
		t.Fatal(err)
	}
	request = httptest.NewRequest(http.MethodPost, "/api/v1/rooms", bytes.NewReader(body))
	recorder = httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("51-rune name status = %d, want %d", recorder.Code, http.StatusBadRequest)
	}

	request = httptest.NewRequest(http.MethodPost, "/api/v1/rooms", strings.NewReader(`{"displayName":"  \u754c  "}`))
	recorder = httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusCreated {
		t.Fatalf("trimmed Unicode name status = %d, want %d", recorder.Code, http.StatusCreated)
	}
}

func TestRouterRejectsInvalidUTF8JSON(t *testing.T) {
	router := api.NewRouter(rooms.NewRoomService(), nil, handlers.MediaConfig{})
	body := append([]byte(`{"displayName":"`), 0xff)
	body = append(body, []byte(`"}`)...)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/rooms", bytes.NewReader(body))
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("invalid UTF-8 status = %d, want %d", recorder.Code, http.StatusBadRequest)
	}
}

func TestRequestLoggingOmitsSecrets(t *testing.T) {
	var logs bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logs, nil))
	router := api.NewRouterWithOptions(rooms.NewRoomService(), nil, handlers.MediaConfig{}, api.RouterOptions{Logger: logger})
	request := httptest.NewRequest(http.MethodPost, "/api/v1/rooms/join-requests", strings.NewReader(`{"code":"SECRET-CODE","displayName":"Alex"}`))
	request.AddCookie(&http.Cookie{Name: "session", Value: "SECRET-TOKEN"})
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	got := logs.String()
	if strings.Contains(got, "SECRET-CODE") || strings.Contains(got, "SECRET-TOKEN") || strings.Contains(got, "Alex") {
		t.Fatalf("request log leaked request data: %s", got)
	}
	for _, field := range []string{"method=POST", "status=", "request_id="} {
		if !strings.Contains(got, field) {
			t.Fatalf("request log %q missing %q", got, field)
		}
	}
}

func TestReadyChecksRedisAndLiveKit(t *testing.T) {
	liveKit := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(liveKit.Close)

	handler := handlers.NewReadyHandler(&fakeRedis{}, liveKit.URL, &http.Client{Timeout: time.Second})
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/ready", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("ready status = %d, want %d: %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}

	handler = handlers.NewReadyHandler(&fakeRedis{pingErr: errors.New("redis down")}, liveKit.URL, &http.Client{Timeout: time.Second})
	recorder = httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/ready", nil))
	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("Redis failure status = %d, want %d", recorder.Code, http.StatusServiceUnavailable)
	}

	handler = handlers.NewReadyHandler(&fakeRedis{}, "http://127.0.0.1:1", &http.Client{Timeout: 50 * time.Millisecond})
	recorder = httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/ready", nil))
	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("LiveKit failure status = %d, want %d", recorder.Code, http.StatusServiceUnavailable)
	}
}
