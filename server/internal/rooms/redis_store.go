package rooms

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

const activeRoomsKey = "reso:rooms:active"

var createRoomScript = redis.NewScript(`
if redis.call("SCARD", KEYS[1]) >= tonumber(ARGV[4]) then
  return 0
end

redis.call("SADD", KEYS[1], ARGV[1])
redis.call("SET", KEYS[2], ARGV[2])
redis.call("SET", KEYS[3], ARGV[1])
return 1
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
		[]string{activeRoomsKey, roomKey(room.ID), roomCodeKey(room.CodeHash)},
		room.ID,
		string(encodedRoom),
		room.CodeHash,
		3,
	).Int()
	if err != nil {
		return err
	}

	if result == 0 {
		return ErrRoomCapacityReached
	}

	return nil
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
		if request.Status == JoinRequestPending {
			requests = append(requests, request)
		}
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
