package queue

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	queueKey            = "reso:queue"
	reservationsKey     = "reso:reservations"
	sessionTTL          = 60 * time.Second
	reservationDuration = 5 * time.Minute
)

var promoteScript = redis.NewScript(`
local reservations = redis.call("ZRANGE", KEYS[3], 0, -1, "WITHSCORES")
for i = 1, #reservations, 2 do
  local reservation_id = reservations[i]
  local session_id = redis.call("GET", ARGV[6] .. reservation_id)
  local stale = tonumber(reservations[i + 1]) <= tonumber(ARGV[1]) or not session_id
  if not stale then
    local session_key = ARGV[5] .. session_id
    local heartbeat = redis.call("HGET", session_key, "heartbeat")
    stale = not heartbeat or tonumber(heartbeat) < tonumber(ARGV[2])
      or redis.call("HGET", session_key, "status") ~= "reserved"
      or redis.call("HGET", session_key, "reservationId") ~= reservation_id
  end
  if stale then
    redis.call("ZREM", KEYS[3], reservation_id)
    redis.call("DEL", ARGV[6] .. reservation_id)
    if session_id then
      redis.call("ZREM", KEYS[1], session_id)
      redis.call("DEL", ARGV[5] .. session_id)
    end
  end
end

if redis.call("SCARD", KEYS[2]) + redis.call("ZCARD", KEYS[3]) >= tonumber(ARGV[4]) then
  return 0
end
while true do
  local candidate = redis.call("ZPOPMIN", KEYS[1], 1)
  if #candidate == 0 then
    return 0
  end
  local session_id = candidate[1]
  local session_key = ARGV[5] .. session_id
  local heartbeat = redis.call("HGET", session_key, "heartbeat")
  if heartbeat and tonumber(heartbeat) >= tonumber(ARGV[2])
    and redis.call("HGET", session_key, "status") == "waiting" then
    redis.call("HSET", session_key, "status", "reserved", "reservationId", ARGV[3])
    redis.call("SET", ARGV[6] .. ARGV[3], session_id, "PX", ARGV[7])
    redis.call("ZADD", KEYS[3], ARGV[1] + ARGV[7], ARGV[3])
    return 1
  end
  redis.call("DEL", session_key)
end
`)

var ErrNotFound = errors.New("queue session not found")
var ErrUnauthorized = errors.New("queue session unauthorized")

type Status string

const (
	Waiting  Status = "waiting"
	Reserved Status = "reserved"
)

type Session struct {
	ID            string
	Status        Status
	Position      int64
	ReservationID string
	ExpiresAt     time.Time
}

type Service struct{ client redis.UniversalClient }

func NewService(client redis.UniversalClient) *Service { return &Service{client: client} }

func (s *Service) Join(ctx context.Context) (Session, string, error) {
	id, err := secret(16)
	if err != nil {
		return Session{}, "", err
	}
	token, err := secret(32)
	if err != nil {
		return Session{}, "", err
	}
	now := time.Now()
	pipe := s.client.TxPipeline()
	pipe.HSet(ctx, sessionKey(id), "tokenHash", hash(token), "status", string(Waiting), "joinedAt", now.UnixMilli(), "heartbeat", now.UnixMilli())
	pipe.Expire(ctx, sessionKey(id), sessionTTL)
	pipe.ZAdd(ctx, queueKey, redis.Z{Score: float64(now.UnixMilli()), Member: id})
	_, err = pipe.Exec(ctx)
	if err != nil {
		return Session{}, "", err
	}
	position, err := s.client.ZRank(ctx, queueKey, id).Result()
	if err != nil {
		return Session{}, "", err
	}
	return Session{ID: id, Status: Waiting, Position: position + 1}, token, nil
}

func (s *Service) Status(ctx context.Context, id, token string) (Session, error) {
	values, err := s.client.HGetAll(ctx, sessionKey(id)).Result()
	if err != nil {
		return Session{}, err
	}
	if len(values) == 0 {
		return Session{}, ErrNotFound
	}
	if values["tokenHash"] != hash(token) {
		return Session{}, ErrUnauthorized
	}
	heartbeat, err := strconv.ParseInt(values["heartbeat"], 10, 64)
	if err != nil || heartbeat < time.Now().Add(-sessionTTL).UnixMilli() {
		_ = s.removeSession(ctx, id, values["reservationId"])
		_ = s.PromoteNext(ctx)
		return Session{}, ErrNotFound
	}
	status := Status(values["status"])
	session := Session{ID: id, Status: status, ReservationID: values["reservationId"]}
	if status == Waiting {
		position, err := s.client.ZRank(ctx, queueKey, id).Result()
		if err == redis.Nil {
			return Session{}, ErrNotFound
		}
		if err != nil {
			return Session{}, err
		}
		session.Position = position + 1
	}
	if status == Reserved {
		expiry, err := s.client.PTTL(ctx, reservationKey(session.ReservationID)).Result()
		if err != nil || expiry <= 0 {
			_ = s.removeSession(ctx, id, session.ReservationID)
			_ = s.PromoteNext(ctx)
			return Session{}, ErrNotFound
		}
		session.ExpiresAt = time.Now().Add(expiry)
	}
	return session, nil
}

func (s *Service) Heartbeat(ctx context.Context, id, token string) error {
	if _, err := s.Status(ctx, id, token); err != nil {
		return err
	}
	pipe := s.client.TxPipeline()
	pipe.HSet(ctx, sessionKey(id), "heartbeat", time.Now().UnixMilli())
	pipe.Expire(ctx, sessionKey(id), sessionTTL)
	_, err := pipe.Exec(ctx)
	if err != nil {
		return err
	}
	return s.PromoteNext(ctx)
}

func (s *Service) Leave(ctx context.Context, id, token string) error {
	status, err := s.Status(ctx, id, token)
	if err != nil {
		return err
	}
	if err := s.removeSession(ctx, id, status.ReservationID); err != nil {
		return err
	}
	return s.PromoteNext(ctx)
}

func (s *Service) PromoteNext(ctx context.Context) error {
	reservationID, err := secret(16)
	if err != nil {
		return err
	}
	now := time.Now()
	return promoteScript.Run(ctx, s.client, []string{queueKey, "reso:rooms:active", reservationsKey}, now.UnixMilli(), now.Add(-sessionTTL).UnixMilli(), reservationID, 3, "reso:queue:", "reso:reservation:", reservationDuration.Milliseconds()).Err()
}

func (s *Service) removeSession(ctx context.Context, id, reservationID string) error {
	_, err := s.client.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
		pipe.ZRem(ctx, queueKey, id)
		pipe.Del(ctx, sessionKey(id))
		if reservationID != "" {
			pipe.ZRem(ctx, reservationsKey, reservationID)
			pipe.Del(ctx, reservationKey(reservationID))
		}
		return nil
	})
	return err
}

func sessionKey(id string) string     { return "reso:queue:" + id }
func reservationKey(id string) string { return "reso:reservation:" + id }
func secret(size int) (string, error) {
	b := make([]byte, size)
	_, err := rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b), err
}
func hash(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}
