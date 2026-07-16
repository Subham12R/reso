package rooms

import "time"

type RoomState string

const (
	RoomStateActive RoomState = "active"
	RoomStateEnded  RoomState = "ended"
)

type Room struct {
	ID                    string
	CodeHash              string
	OwnerSessionHash      string
	StreamHostSessionHash string
	OwnerName             string
	State                 RoomState
	CreatedAt             time.Time
	ExpiresAt             time.Time
	EndedAt               time.Time
}

type JoinRequestStatus string

const (
	JoinRequestPending  JoinRequestStatus = "pending"
	JoinRequestApproved JoinRequestStatus = "approved"
	JoinRequestRejected JoinRequestStatus = "rejected"
)

type JoinRequest struct {
	ID               string
	Name             string
	RoomID           string
	GuestSessionHash string
	Status           JoinRequestStatus
	CreatedAt        time.Time
	ExpiresAt        time.Time
}

type ApprovedJoinRequest struct {
	Request      JoinRequest
	SessionToken string
}
