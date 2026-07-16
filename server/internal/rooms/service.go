package rooms

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"time"
)

var ErrRoomNotFound = errors.New("room not found")
var ErrRoomCapacityReached = errors.New("room capacity reached")
var ErrJoinRequestNotPending = errors.New("join request not pending")
var ErrJoinRequestExpired = errors.New("join request expired")
var ErrUnauthorized = errors.New("unauthorized")
var ErrJoinRequestNotFound = errors.New("join request not found")
var ErrRoomEnded = errors.New("room ended")
var ErrRoomExpired = errors.New("room expired")
var ErrReservationUnavailable = errors.New("reservation unavailable")

type RoomService struct {
	store Store
}

type CreatedRoom struct {
	Room              Room
	Code              string
	OwnerSessionToken string
}

type SessionRole string

const (
	SessionRoleOwner       SessionRole = "owner"
	SessionRoleParticipant SessionRole = "participant"
)

func NewRoomService() *RoomService {
	return NewRoomServiceWithStore(NewMemoryStore())
}

func NewRoomServiceWithStore(store Store) *RoomService {
	return &RoomService{store: store}
}

func (service *RoomService) CreateRoom(ownerName string) (CreatedRoom, error) {
	created, err := newRoom(ownerName)
	if err != nil {
		return CreatedRoom{}, err
	}

	if err := service.store.CreateRoom(context.Background(), created.Room); err != nil {
		return CreatedRoom{}, err
	}

	return created, nil
}

func (service *RoomService) ClaimRoom(ownerName, queueSessionID, queueToken string) (CreatedRoom, error) {
	created, err := newRoom(ownerName)
	if err != nil {
		return CreatedRoom{}, err
	}
	if err := service.store.ClaimRoom(context.Background(), created.Room, queueSessionID, hashSecret(queueToken)); err != nil {
		return CreatedRoom{}, err
	}
	return created, nil
}

func newRoom(ownerName string) (CreatedRoom, error) {
	roomID, err := generateSecret(16)
	if err != nil {
		return CreatedRoom{}, err
	}

	code, err := generateSecret(8)
	if err != nil {
		return CreatedRoom{}, err
	}

	ownerSessionToken, err := generateSecret(32)
	if err != nil {
		return CreatedRoom{}, err
	}

	room := Room{
		ID:               roomID,
		CodeHash:         hashSecret(code),
		OwnerSessionHash: hashSecret(ownerSessionToken),
		OwnerName:        ownerName,
		State:            RoomStateActive,
		CreatedAt:        time.Now(),
		ExpiresAt:        time.Now().Add(8 * time.Hour),
	}

	return CreatedRoom{
		Room:              room,
		Code:              code,
		OwnerSessionToken: ownerSessionToken,
	}, nil
}

func (service *RoomService) FindRoomByCode(code string) (Room, error) {
	room, err := service.store.FindRoomByCodeHash(
		context.Background(),
		hashSecret(code),
	)
	if err != nil {
		return Room{}, err
	}
	if room.State == RoomStateEnded {
		return Room{}, ErrRoomEnded
	}
	if time.Now().After(room.ExpiresAt) {
		return Room{}, ErrRoomExpired
	}
	return room, nil
}

func (service *RoomService) EndRoom(roomID, ownerSessionToken string) (Room, error) {
	ctx := context.Background()
	room, err := service.store.FindRoomByID(ctx, roomID)
	if err != nil {
		return Room{}, err
	}
	if hashSecret(ownerSessionToken) != room.OwnerSessionHash {
		return Room{}, ErrUnauthorized
	}
	if room.State == RoomStateEnded {
		return room, nil
	}
	room.State = RoomStateEnded
	room.EndedAt = time.Now()
	if err := service.endRoom(ctx, room); err != nil {
		return Room{}, err
	}
	return room, nil
}

func (service *RoomService) endRoom(ctx context.Context, room Room) error {
	reservationID, err := generateSecret(16)
	if err != nil {
		return err
	}
	return service.store.EndRoom(ctx, room, reservationID)
}

func (service *RoomService) RoomState(roomID string) (Room, error) {
	room, err := service.store.FindRoomByID(context.Background(), roomID)
	if err != nil {
		return Room{}, err
	}
	if room.State == RoomStateActive && time.Now().After(room.ExpiresAt) {
		room.State = RoomStateEnded
		room.EndedAt = time.Now()
		if err := service.endRoom(context.Background(), room); err != nil {
			return Room{}, err
		}
	}
	return room, nil
}

func (service *RoomService) AuthorizeRoomSession(roomID, sessionToken string) (SessionRole, error) {
	ctx := context.Background()
	room, err := service.store.FindRoomByID(ctx, roomID)
	if err != nil {
		return "", err
	}
	if room.State != RoomStateActive || time.Now().After(room.ExpiresAt) {
		return "", ErrRoomEnded
	}
	if hashSecret(sessionToken) == room.OwnerSessionHash {
		return SessionRoleOwner, nil
	}
	requests, err := service.store.ListJoinRequests(ctx, roomID)
	if err != nil {
		return "", err
	}
	for _, request := range requests {
		if request.Status == JoinRequestApproved && hashSecret(sessionToken) == request.GuestSessionHash {
			return SessionRoleParticipant, nil
		}
	}
	return "", ErrUnauthorized
}

func (service *RoomService) CreateJoinRequest(
	code string,
	name string,
) (JoinRequest, error) {
	room, err := service.FindRoomByCode(code)
	if err != nil {
		return JoinRequest{}, err
	}

	requestID, err := generateSecret(16)
	if err != nil {
		return JoinRequest{}, err
	}

	createdAt := time.Now()
	request := JoinRequest{
		ID:        requestID,
		Name:      name,
		RoomID:    room.ID,
		Status:    JoinRequestPending,
		CreatedAt: createdAt,
		ExpiresAt: createdAt.Add(2 * time.Minute),
	}

	if err := service.store.CreateJoinRequest(
		context.Background(),
		request,
	); err != nil {
		return JoinRequest{}, err
	}

	return request, nil
}

func (service *RoomService) ApproveJoinRequest(
	roomID string,
	requestID string,
	ownerSessionToken string,
) (ApprovedJoinRequest, error) {
	ctx := context.Background()

	room, err := service.store.FindRoomByID(ctx, roomID)
	if err != nil {
		return ApprovedJoinRequest{}, err
	}

	if hashSecret(ownerSessionToken) != room.OwnerSessionHash {
		return ApprovedJoinRequest{}, ErrUnauthorized
	}

	request, err := service.store.FindJoinRequest(ctx, requestID)
	if err != nil || request.RoomID != roomID {
		return ApprovedJoinRequest{}, ErrJoinRequestNotFound
	}

	if request.Status != JoinRequestPending {
		return ApprovedJoinRequest{}, ErrJoinRequestNotPending
	}

	if time.Now().After(request.ExpiresAt) {
		return ApprovedJoinRequest{}, ErrJoinRequestExpired
	}

	sessionToken, err := generateSecret(32)
	if err != nil {
		return ApprovedJoinRequest{}, err
	}

	request.Status = JoinRequestApproved
	request.GuestSessionHash = hashSecret(sessionToken)

	if err := service.store.UpdateJoinRequest(ctx, request); err != nil {
		return ApprovedJoinRequest{}, err
	}

	return ApprovedJoinRequest{
		Request:      request,
		SessionToken: sessionToken,
	}, nil
}

func (service *RoomService) RejectJoinRequest(
	roomID string,
	requestID string,
	ownerSessionToken string,
) (JoinRequest, error) {
	ctx := context.Background()

	room, err := service.store.FindRoomByID(ctx, roomID)
	if err != nil {
		return JoinRequest{}, err
	}

	if hashSecret(ownerSessionToken) != room.OwnerSessionHash {
		return JoinRequest{}, ErrUnauthorized
	}

	request, err := service.store.FindJoinRequest(ctx, requestID)
	if err != nil || request.RoomID != roomID {
		return JoinRequest{}, ErrJoinRequestNotFound
	}

	if request.Status != JoinRequestPending {
		return JoinRequest{}, ErrJoinRequestNotPending
	}

	if time.Now().After(request.ExpiresAt) {
		return JoinRequest{}, ErrJoinRequestExpired
	}

	request.Status = JoinRequestRejected

	if err := service.store.UpdateJoinRequest(ctx, request); err != nil {
		return JoinRequest{}, err
	}

	return request, nil
}

func (service *RoomService) ListPendingJoinRequests(
	roomID string,
	ownerSessionToken string,
) ([]JoinRequest, error) {
	ctx := context.Background()

	room, err := service.store.FindRoomByID(ctx, roomID)
	if err != nil {
		return nil, err
	}

	if hashSecret(ownerSessionToken) != room.OwnerSessionHash {
		return nil, ErrUnauthorized
	}

	requests, err := service.store.ListPendingJoinRequests(ctx, roomID)
	if err != nil {
		return nil, err
	}

	var active []JoinRequest
	for _, request := range requests {
		if time.Now().Before(request.ExpiresAt) {
			active = append(active, request)
		}
	}

	return active, nil
}

func generateSecret(size int) (string, error) {
	bytes := make([]byte, size)

	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}

	return base64.RawURLEncoding.EncodeToString(bytes), nil
}

func hashSecret(secret string) string {
	sum := sha256.Sum256([]byte(secret))
	return hex.EncodeToString(sum[:])
}
