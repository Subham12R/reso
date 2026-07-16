package media

import (
	"crypto/sha256"
	"encoding/hex"
	"time"

	"github.com/livekit/protocol/auth"
)

func SessionHash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func IssueToken(apiKey, apiSecret, roomID, identity string, canPublish bool) (string, error) {
	token := auth.NewAccessToken(apiKey, apiSecret)
	grant := &auth.VideoGrant{RoomJoin: true, Room: roomID}
	grant.SetCanSubscribe(true)
	grant.SetCanPublish(canPublish)
	token.SetVideoGrant(grant).SetIdentity(identity).SetValidFor(15 * time.Minute)
	return token.ToJWT()
}
