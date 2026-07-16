package rooms

import "context"

type Store interface {
	CreateRoom(context.Context, Room) error
	ClaimRoom(context.Context, Room, string, string) error
	EndRoom(context.Context, Room, string) error
	UpdateRoom(context.Context, Room) error

	FindRoomByCodeHash(context.Context, string) (Room, error)
	FindRoomByID(context.Context, string) (Room, error)

	CreateJoinRequest(context.Context, JoinRequest) error
	FindJoinRequest(context.Context, string) (JoinRequest, error)
	UpdateJoinRequest(context.Context, JoinRequest) error

	ListPendingJoinRequests(context.Context, string) ([]JoinRequest, error)
	ListJoinRequests(context.Context, string) ([]JoinRequest, error)
}
