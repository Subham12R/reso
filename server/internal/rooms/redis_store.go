package rooms

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

const activeRoomsKey = "reso:rooms:active"
const reservationsKey = "reso:reservations"

var createRoomScript = redis.NewScript(`
local reservations = redis.call("ZRANGE", KEYS[4], 0, -1, "WITHSCORES")
for i = 1, #reservations, 2 do
  local reservation_id = reservations[i]
  local session_id = redis.call("GET", ARGV[7] .. reservation_id)
  local stale = tonumber(reservations[i + 1]) <= tonumber(ARGV[4]) or not session_id
  if not stale then
    local session_key = ARGV[6] .. session_id
    local heartbeat = redis.call("HGET", session_key, "heartbeat")
    stale = not heartbeat or tonumber(heartbeat) < tonumber(ARGV[5])
      or redis.call("HGET", session_key, "status") ~= "reserved"
      or redis.call("HGET", session_key, "reservationId") ~= reservation_id
  end
  if stale then
    redis.call("ZREM", KEYS[4], reservation_id)
    redis.call("DEL", ARGV[7] .. reservation_id)
    if session_id then
      redis.call("ZREM", KEYS[5], session_id)
      redis.call("DEL", ARGV[6] .. session_id)
    end
  end
end

if redis.call("SCARD", KEYS[1]) + redis.call("ZCARD", KEYS[4]) >= tonumber(ARGV[3]) then
  return 0
end

redis.call("SADD", KEYS[1], ARGV[1])
redis.call("SET", KEYS[2], ARGV[2])
redis.call("SET", KEYS[3], ARGV[1])
return 1
`)

var claimRoomScript = redis.NewScript(`
local reservations = redis.call("ZRANGE", KEYS[6], 0, -1, "WITHSCORES")
for i = 1, #reservations, 2 do
  local reservation_id = reservations[i]
  local session_id = redis.call("GET", ARGV[8] .. reservation_id)
  local stale = tonumber(reservations[i + 1]) <= tonumber(ARGV[4]) or not session_id
  if not stale then
    local session_key = ARGV[7] .. session_id
    local heartbeat = redis.call("HGET", session_key, "heartbeat")
    stale = not heartbeat or tonumber(heartbeat) < tonumber(ARGV[5])
      or redis.call("HGET", session_key, "status") ~= "reserved"
      or redis.call("HGET", session_key, "reservationId") ~= reservation_id
  end
  if stale then
    redis.call("ZREM", KEYS[6], reservation_id)
    redis.call("DEL", ARGV[8] .. reservation_id)
    if session_id then
      redis.call("ZREM", KEYS[5], session_id)
      redis.call("DEL", ARGV[7] .. session_id)
    end
  end
end

if redis.call("HGET", KEYS[4], "tokenHash") ~= ARGV[6] then
  return -1
end
local reservation_id = redis.call("HGET", KEYS[4], "reservationId")
if redis.call("HGET", KEYS[4], "status") ~= "reserved" or not reservation_id then
  return -2
end
local expiry = redis.call("ZSCORE", KEYS[6], reservation_id)
if not expiry or tonumber(expiry) <= tonumber(ARGV[4])
  or redis.call("GET", ARGV[8] .. reservation_id) ~= ARGV[3] then
  return -2
end
if redis.call("SCARD", KEYS[1]) + redis.call("ZCARD", KEYS[6]) > tonumber(ARGV[9]) then
  return 0
end

redis.call("ZREM", KEYS[6], reservation_id)
redis.call("DEL", ARGV[8] .. reservation_id)
redis.call("DEL", KEYS[4])
redis.call("ZREM", KEYS[5], ARGV[3])
redis.call("SADD", KEYS[1], ARGV[1])
redis.call("SET", KEYS[2], ARGV[2])
redis.call("SET", KEYS[3], ARGV[1])
return 1
`)

var endRoomScript = redis.NewScript(`
redis.call("SET", KEYS[2], ARGV[2])
redis.call("SREM", KEYS[1], ARGV[1])

local reservations = redis.call("ZRANGE", KEYS[4], 0, -1, "WITHSCORES")
for i = 1, #reservations, 2 do
  local reservation_id = reservations[i]
  local session_id = redis.call("GET", ARGV[8] .. reservation_id)
  local stale = tonumber(reservations[i + 1]) <= tonumber(ARGV[3]) or not session_id
  if not stale then
    local session_key = ARGV[7] .. session_id
    local heartbeat = redis.call("HGET", session_key, "heartbeat")
    stale = not heartbeat or tonumber(heartbeat) < tonumber(ARGV[4])
      or redis.call("HGET", session_key, "status") ~= "reserved"
      or redis.call("HGET", session_key, "reservationId") ~= reservation_id
  end
  if stale then
    redis.call("ZREM", KEYS[4], reservation_id)
    redis.call("DEL", ARGV[8] .. reservation_id)
    if session_id then
      redis.call("ZREM", KEYS[3], session_id)
      redis.call("DEL", ARGV[7] .. session_id)
    end
  end
end

if redis.call("SCARD", KEYS[1]) + redis.call("ZCARD", KEYS[4]) >= tonumber(ARGV[9]) then
  return 0
end
while true do
  local candidate = redis.call("ZPOPMIN", KEYS[3], 1)
  if #candidate == 0 then
    return 0
  end
  local session_id = candidate[1]
  local session_key = ARGV[7] .. session_id
  local heartbeat = redis.call("HGET", session_key, "heartbeat")
  if heartbeat and tonumber(heartbeat) >= tonumber(ARGV[4])
    and redis.call("HGET", session_key, "status") == "waiting" then
    redis.call("HSET", session_key, "status", "reserved", "reservationId", ARGV[5])
    redis.call("SET", ARGV[8] .. ARGV[5], session_id, "PX", ARGV[6])
    redis.call("ZADD", KEYS[4], ARGV[3] + ARGV[6], ARGV[5])
    return 1
  end
  redis.call("DEL", session_key)
end
`)

type RedisStore struct {
	client redis.UniversalClient
}

func NewRedisStore(client redis.UniversalClient) *RedisStore {
	return &RedisStore{client: client}
}

func (store *RedisStore) CreateRoom(ctx context.Context, room Room) error {
	encodedRoom, err := json.Marshal(room)
	if err != nil {
		return err
	}

	result, err := createRoomScript.Run(
		ctx,
		store.client,
		[]string{activeRoomsKey, roomKey(room.ID), roomCodeKey(room.CodeHash), reservationsKey, "reso:queue"},
		room.ID,
		string(encodedRoom),
		3,
		time.Now().UnixMilli(),
		time.Now().Add(-60*time.Second).UnixMilli(),
		"reso:queue:",
		"reso:reservation:",
	).Int()
	if err != nil {
		return err
	}

	if result == 0 {
		return ErrRoomCapacityReached
	}

	return nil
}

func (store *RedisStore) ClaimRoom(ctx context.Context, room Room, queueSessionID, tokenHash string) error {
	encodedRoom, err := json.Marshal(room)
	if err != nil {
		return err
	}
	now := time.Now()
	result, err := claimRoomScript.Run(ctx, store.client, []string{
		activeRoomsKey,
		roomKey(room.ID),
		roomCodeKey(room.CodeHash),
		"reso:queue:" + queueSessionID,
		"reso:queue",
		reservationsKey,
	}, room.ID, string(encodedRoom), queueSessionID, now.UnixMilli(), now.Add(-60*time.Second).UnixMilli(), tokenHash, "reso:queue:", "reso:reservation:", 3).Int()
	if err != nil {
		return err
	}
	switch result {
	case 1:
		return nil
	case -1:
		return ErrUnauthorized
	case -2:
		return ErrReservationUnavailable
	default:
		return ErrRoomCapacityReached
	}
}

func (store *RedisStore) EndRoom(ctx context.Context, room Room, reservationID string) error {
	encodedRoom, err := json.Marshal(room)
	if err != nil {
		return err
	}
	now := time.Now()
	return endRoomScript.Run(ctx, store.client, []string{
		activeRoomsKey,
		roomKey(room.ID),
		"reso:queue",
		reservationsKey,
	}, room.ID, string(encodedRoom), now.UnixMilli(), now.Add(-60*time.Second).UnixMilli(), reservationID, (5 * time.Minute).Milliseconds(), "reso:queue:", "reso:reservation:", 3).Err()
}

func (store *RedisStore) UpdateRoom(ctx context.Context, room Room) error {
	encodedRoom, err := json.Marshal(room)
	if err != nil {
		return err
	}

	_, err = store.client.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
		pipe.Set(ctx, roomKey(room.ID), encodedRoom, 0)
		if room.State == RoomStateEnded {
			pipe.SRem(ctx, activeRoomsKey, room.ID)
		}
		return nil
	})
	return err
}

func (store *RedisStore) FindRoomByCodeHash(
	ctx context.Context,
	codeHash string,
) (Room, error) {
	roomID, err := store.client.Get(ctx, roomCodeKey(codeHash)).Result()
	if err == redis.Nil {
		return Room{}, ErrRoomNotFound
	}
	if err != nil {
		return Room{}, err
	}

	return store.FindRoomByID(ctx, roomID)
}

func (store *RedisStore) FindRoomByID(
	ctx context.Context,
	roomID string,
) (Room, error) {
	encodedRoom, err := store.client.Get(ctx, roomKey(roomID)).Bytes()
	if err == redis.Nil {
		return Room{}, ErrRoomNotFound
	}
	if err != nil {
		return Room{}, err
	}

	var room Room
	if err := json.Unmarshal(encodedRoom, &room); err != nil {
		return Room{}, err
	}

	return room, nil
}

func (store *RedisStore) CreateJoinRequest(
	ctx context.Context,
	request JoinRequest,
) error {
	ttl := time.Until(request.ExpiresAt)
	if ttl <= 0 {
		return ErrJoinRequestExpired
	}

	encodedRequest, err := json.Marshal(request)
	if err != nil {
		return err
	}

	_, err = store.client.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
		pipe.Set(ctx, joinRequestKey(request.ID), encodedRequest, ttl)
		pipe.SAdd(ctx, roomJoinRequestsKey(request.RoomID), request.ID)
		return nil
	})
	return err
}

func (store *RedisStore) FindJoinRequest(
	ctx context.Context,
	requestID string,
) (JoinRequest, error) {
	encodedRequest, err := store.client.Get(ctx, joinRequestKey(requestID)).Bytes()
	if err == redis.Nil {
		return JoinRequest{}, ErrJoinRequestNotFound
	}
	if err != nil {
		return JoinRequest{}, err
	}

	var request JoinRequest
	if err := json.Unmarshal(encodedRequest, &request); err != nil {
		return JoinRequest{}, err
	}

	return request, nil
}

func (store *RedisStore) UpdateJoinRequest(
	ctx context.Context,
	request JoinRequest,
) error {
	ttl, err := store.client.TTL(ctx, joinRequestKey(request.ID)).Result()
	if err == redis.Nil || ttl == -2 {
		return ErrJoinRequestNotFound
	}
	if err != nil {
		return err
	}

	if ttl <= 0 {
		return ErrJoinRequestExpired
	}

	encodedRequest, err := json.Marshal(request)
	if err != nil {
		return err
	}

	return store.client.Set(ctx, joinRequestKey(request.ID), encodedRequest, ttl).Err()
}

func (store *RedisStore) ListPendingJoinRequests(
	ctx context.Context,
	roomID string,
) ([]JoinRequest, error) {
	all, err := store.ListJoinRequests(ctx, roomID)
	if err != nil {
		return nil, err
	}

	requests := make([]JoinRequest, 0, len(all))
	for _, request := range all {
		if request.Status == JoinRequestPending {
			requests = append(requests, request)
		}
	}
	return requests, nil
}

func (store *RedisStore) ListJoinRequests(
	ctx context.Context,
	roomID string,
) ([]JoinRequest, error) {
	requestIDs, err := store.client.SMembers(ctx, roomJoinRequestsKey(roomID)).Result()
	if err != nil {
		return nil, err
	}

	requests := make([]JoinRequest, 0, len(requestIDs))
	for _, requestID := range requestIDs {
		request, err := store.FindJoinRequest(ctx, requestID)
		if err == ErrJoinRequestNotFound {
			continue
		}
		if err != nil {
			return nil, err
		}
		requests = append(requests, request)
	}

	return requests, nil
}

func roomKey(roomID string) string {
	return fmt.Sprintf("reso:room:%s", roomID)
}

func roomCodeKey(codeHash string) string {
	return fmt.Sprintf("reso:room:code:%s", codeHash)
}

func joinRequestKey(requestID string) string {
	return fmt.Sprintf("reso:join-request:%s", requestID)
}

func roomJoinRequestsKey(roomID string) string {
	return fmt.Sprintf("reso:room:%s:join-requests", roomID)
}
