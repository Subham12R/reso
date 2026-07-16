package media

import (
	"context"
	"errors"

	"github.com/livekit/protocol/livekit"
	lksdk "github.com/livekit/server-sdk-go/v2"
	"github.com/twitchtv/twirp"
)

var ErrRoomAbsent = errors.New("livekit room absent")

type RoomCleaner interface {
	DeleteRoom(context.Context, string) error
}

type LiveKitCleaner struct{ client *lksdk.RoomServiceClient }

func NewLiveKitCleaner(url, apiKey, apiSecret string) *LiveKitCleaner {
	return &LiveKitCleaner{client: lksdk.NewRoomServiceClient(url, apiKey, apiSecret)}
}

func (cleaner *LiveKitCleaner) DeleteRoom(ctx context.Context, roomID string) error {
	_, err := cleaner.client.DeleteRoom(ctx, &livekit.DeleteRoomRequest{Room: roomID})
	var twirpError twirp.Error
	if errors.As(err, &twirpError) && twirpError.Code() == twirp.NotFound {
		return ErrRoomAbsent
	}
	return err
}
