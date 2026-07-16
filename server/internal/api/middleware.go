package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	maxJSONBodySize = 16 << 10
	rateWindow      = time.Minute
)

type requestIDKey struct{}

type statusWriter struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func (writer *statusWriter) WriteHeader(status int) {
	if writer.wroteHeader {
		return
	}
	writer.wroteHeader = true
	writer.status = status
	writer.ResponseWriter.WriteHeader(status)
}

func requestMiddleware(next http.Handler, logger *slog.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := r.Header.Get("X-Request-ID")
		if !validRequestID(requestID) {
			requestID = randomRequestID()
		}
		w.Header().Set("X-Request-ID", requestID)
		w.Header().Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")

		started := time.Now()
		writer := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(writer, r.WithContext(context.WithValue(r.Context(), requestIDKey{}, requestID)))
		logger.Info("http request",
			"request_id", requestID,
			"method", r.Method,
			"status", writer.status,
			"duration_ms", time.Since(started).Milliseconds(),
		)
	})
}

func validRequestID(value string) bool {
	if len(value) != 32 {
		return false
	}
	for _, character := range value {
		if character >= '0' && character <= '9' || character >= 'a' && character <= 'f' {
			continue
		}
		return false
	}
	return true
}

func randomRequestID() string {
	value := make([]byte, 16)
	if _, err := rand.Read(value); err != nil {
		panic(fmt.Errorf("generate request ID: %w", err))
	}
	return hex.EncodeToString(value)
}

func limitBodies(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.ContentLength > maxJSONBodySize {
			http.Error(w, "request too large", http.StatusRequestEntityTooLarge)
			return
		}
		if r.Body != nil {
			r.Body = http.MaxBytesReader(w, r.Body, maxJSONBodySize)
		}
		next.ServeHTTP(w, r)
	})
}

const rateLimitScript = `
local current = redis.call('INCR', KEYS[1])
if current == 1 then
  redis.call('EXPIRE', KEYS[1], ARGV[1])
end
return {current, redis.call('TTL', KEYS[1])}
`

func rateLimit(next http.Handler, redisClient RedisClient, trustProxyHeaders bool) http.Handler {
	if redisClient == nil {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		category, limit := requestLimit(r)
		if limit == 0 {
			next.ServeHTTP(w, r)
			return
		}

		key := "reso:rate:" + category + ":" + clientIP(r, trustProxyHeaders)
		values, err := redisClient.Eval(r.Context(), rateLimitScript, []string{key}, int64(rateWindow/time.Second)).Slice()
		if err != nil || len(values) != 2 {
			http.Error(w, "rate limiter unavailable", http.StatusServiceUnavailable)
			return
		}
		count, countOK := redisInt(values[0])
		ttl, ttlOK := redisInt(values[1])
		if !countOK || !ttlOK {
			http.Error(w, "rate limiter unavailable", http.StatusServiceUnavailable)
			return
		}
		if count > limit {
			if ttl < 1 {
				ttl = 1
			}
			w.Header().Set("Retry-After", strconv.FormatInt(ttl, 10))
			http.Error(w, "too many requests", http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func requestLimit(r *http.Request) (string, int64) {
	switch {
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/rooms":
		return "room-create", 5
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/rooms/join-requests":
		return "room-join", 10
	case strings.HasPrefix(r.URL.Path, "/api/v1/queue/"):
		return "queue", 60
	default:
		return "", 0
	}
}

func clientIP(r *http.Request, trustProxyHeaders bool) string {
	if trustProxyHeaders {
		forwarded := strings.TrimSpace(strings.Split(r.Header.Get("X-Forwarded-For"), ",")[0])
		if net.ParseIP(forwarded) != nil {
			return forwarded
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return r.RemoteAddr
}

func redisInt(value interface{}) (int64, bool) {
	switch value := value.(type) {
	case int64:
		return value, true
	case string:
		parsed, err := strconv.ParseInt(value, 10, 64)
		return parsed, err == nil
	case []byte:
		parsed, err := strconv.ParseInt(string(value), 10, 64)
		return parsed, err == nil
	default:
		return 0, false
	}
}

func isBodyTooLarge(err error) bool {
	var maxBytesError *http.MaxBytesError
	return errors.As(err, &maxBytesError)
}
