package rooms

import (
	"context"
	"sync"
)

type MemoryStore struct {
	mu              sync.RWMutex
	roomsByCodeHash map[string]Room
	roomsByID       map[string]Room
	joinRequests    map[string]JoinRequest
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		roomsByCodeHash: make(map[string]Room),
		roomsByID:       make(map[string]Room),
		joinRequests:    make(map[string]JoinRequest),
	}
}

func (store *MemoryStore) CreateRoom(_ context.Context, room Room) error {
	store.mu.Lock()
	defer store.mu.Unlock()

	active := 0
	for _, existing := range store.roomsByID {
		if existing.State != RoomStateEnded {
			active++
		}
	}
	if active >= 3 {
		return ErrRoomCapacityReached
	}

	store.roomsByCodeHash[room.CodeHash] = room
	store.roomsByID[room.ID] = room

	return nil
}

func (store *MemoryStore) ClaimRoom(_ context.Context, _ Room, _, _ string) error {
	return ErrReservationUnavailable
}

func (store *MemoryStore) EndRoom(ctx context.Context, room Room, _ string) error {
	return store.UpdateRoom(ctx, room)
}

func (store *MemoryStore) UpdateRoom(_ context.Context, room Room) error {
	store.mu.Lock()
	defer store.mu.Unlock()

	if _, found := store.roomsByID[room.ID]; !found {
		return ErrRoomNotFound
	}
	store.roomsByID[room.ID] = room
	store.roomsByCodeHash[room.CodeHash] = room
	return nil
}

func (store *MemoryStore) FindRoomByCodeHash(
	_ context.Context,
	codeHash string,
) (Room, error) {
	store.mu.RLock()
	room, found := store.roomsByCodeHash[codeHash]
	store.mu.RUnlock()

	if !found {
		return Room{}, ErrRoomNotFound
	}

	return room, nil
}

func (store *MemoryStore) FindRoomByID(
	_ context.Context,
	roomID string,
) (Room, error) {
	store.mu.RLock()
	room, found := store.roomsByID[roomID]
	store.mu.RUnlock()

	if !found {
		return Room{}, ErrRoomNotFound
	}

	return room, nil
}

func (store *MemoryStore) CreateJoinRequest(
	_ context.Context,
	request JoinRequest,
) error {
	store.mu.Lock()
	store.joinRequests[request.ID] = request
	store.mu.Unlock()

	return nil
}

func (store *MemoryStore) FindJoinRequest(
	_ context.Context,
	requestID string,
) (JoinRequest, error) {
	store.mu.RLock()
	request, found := store.joinRequests[requestID]
	store.mu.RUnlock()

	if !found {
		return JoinRequest{}, ErrJoinRequestNotFound
	}

	return request, nil
}

func (store *MemoryStore) UpdateJoinRequest(
	_ context.Context,
	request JoinRequest,
) error {
	store.mu.Lock()
	defer store.mu.Unlock()

	if _, found := store.joinRequests[request.ID]; !found {
		return ErrJoinRequestNotFound
	}

	store.joinRequests[request.ID] = request
	return nil
}

func (store *MemoryStore) ListPendingJoinRequests(
	_ context.Context,
	roomID string,
) ([]JoinRequest, error) {
	store.mu.RLock()
	defer store.mu.RUnlock()

	var requests []JoinRequest

	for _, request := range store.joinRequests {
		if request.RoomID != roomID {
			continue
		}

		if request.Status != JoinRequestPending {
			continue
		}

		requests = append(requests, request)
	}

	return requests, nil
}

func (store *MemoryStore) ListJoinRequests(_ context.Context, roomID string) ([]JoinRequest, error) {
	store.mu.RLock()
	defer store.mu.RUnlock()

	requests := make([]JoinRequest, 0)
	for _, request := range store.joinRequests {
		if request.RoomID == roomID {
			requests = append(requests, request)
		}
	}
	return requests, nil
}
