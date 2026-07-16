package media

import (
	"time"

	"github.com/livekit/protocol/auth"
)

func IssueToken(apiKey, apiSecret, roomID, identity string, canPublish bool) (string, error) {
	token := auth.NewAccessToken(apiKey, apiSecret)
	grant := &auth.VideoGrant{RoomJoin: true, Room: roomID}
	grant.SetCanSubscribe(true)
	grant.SetCanPublish(canPublish)
	token.SetVideoGrant(grant).SetIdentity(identity).SetValidFor(15 * time.Minute)
	return token.ToJWT()
}
