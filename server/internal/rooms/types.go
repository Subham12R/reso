package rooms

import "time"

type Room struct {
	ID               string
	CodeHash         string
	OwnerSessionHash string
	OwnerName        string
	CreatedAt        time.Time
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
