package queue

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	activeRoomsKey = "reso:rooms:active"
	queueKey       = "reso:queue"
)

var ErrNotFound = errors.New("queue session not found")
var ErrUnauthorized = errors.New("queue session unauthorized")
var ErrNotReserved = errors.New("queue session not reserved")

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
	pipe.HSet(ctx, sessionKey(id), "tokenHash", hash(token), "status", Waiting, "joinedAt", now.UnixMilli(), "heartbeat", now.UnixMilli())
	pipe.Expire(ctx, sessionKey(id), 10*time.Minute)
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
	if expiry, err := s.client.PTTL(ctx, reservationKey(session.ReservationID)).Result(); err == nil && expiry > 0 {
		session.ExpiresAt = time.Now().Add(expiry)
	}
	return session, nil
}

func (s *Service) Heartbeat(ctx context.Context, id, token string) error {
	if _, err := s.Status(ctx, id, token); err != nil {
		return err
	}
	return s.client.HSet(ctx, sessionKey(id), "heartbeat", time.Now().UnixMilli()).Err()
}

func (s *Service) Leave(ctx context.Context, id, token string) error {
	if _, err := s.Status(ctx, id, token); err != nil {
		return err
	}
	_, err := s.client.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
		pipe.ZRem(ctx, queueKey, id)
		pipe.Del(ctx, sessionKey(id))
		return nil
	})
	return err
}

func (s *Service) PromoteNext(ctx context.Context) error {
	result, err := s.client.ZPopMin(ctx, queueKey, 1).Result()
	if err != nil || len(result) == 0 {
		return err
	}
	id := result[0].Member.(string)
	reservationID, err := secret(16)
	if err != nil {
		return err
	}
	_, err = s.client.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
		pipe.HSet(ctx, sessionKey(id), "status", Reserved, "reservationId", reservationID)
		pipe.Set(ctx, reservationKey(reservationID), id, 5*time.Minute)
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
