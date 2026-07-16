package media

import (
	"crypto/sha256"
	"encoding/hex"
	"time"

	"github.com/livekit/protocol/auth"
	"github.com/livekit/protocol/livekit"
)

func SessionHash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func IssueToken(apiKey, apiSecret, roomID, identity, displayName string, canPublish bool) (string, error) {
	token := auth.NewAccessToken(apiKey, apiSecret)
	grant := &auth.VideoGrant{RoomJoin: true, Room: roomID}
	grant.SetCanSubscribe(true)
	grant.SetCanPublish(true)
	sources := []livekit.TrackSource{livekit.TrackSource_CAMERA, livekit.TrackSource_MICROPHONE}
	if canPublish { sources = append(sources, livekit.TrackSource_SCREEN_SHARE, livekit.TrackSource_SCREEN_SHARE_AUDIO) }
	grant.SetCanPublishSources(sources)
	grant.SetCanPublishData(true)
	token.SetVideoGrant(grant).SetIdentity(identity).SetName(displayName).SetValidFor(15 * time.Minute)
	return token.ToJWT()
}
