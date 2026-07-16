package realtime_test

import (
	"context"
	"testing"
	"time"

	"github.com/subham12r/reso/internal/realtime"
)

type memoryPresence struct{ sessions map[string]time.Time }

func (p *memoryPresence) Join(_ context.Context, _, sessionID string, expiresAt time.Time, maximum int) (bool, error) {
	if p.sessions == nil {
		p.sessions = make(map[string]time.Time)
	}
	now := time.Now()
	for id, expiry := range p.sessions {
		if !expiry.After(now) {
			delete(p.sessions, id)
		}
	}
	if len(p.sessions) >= maximum {
		return false, nil
	}
	p.sessions[sessionID] = expiresAt
	return true, nil
}

func (p *memoryPresence) Heartbeat(_ context.Context, _, sessionID string, expiresAt time.Time) error {
	p.sessions[sessionID] = expiresAt
	return nil
}

func (p *memoryPresence) Leave(_ context.Context, _, sessionID string) error {
	delete(p.sessions, sessionID)
	return nil
}

func TestHubLimitsRoomToEightActiveSessions(t *testing.T) {
	hub := realtime.NewHubWithPresence(&memoryPresence{})
	clients := make([]*realtime.Client, 0, 8)
	for index := 0; index < 8; index++ {
		client, err := hub.Join(context.Background(), "room", "participant", "person")
		if err != nil {
			t.Fatalf("join %d: %v", index, err)
		}
		clients = append(clients, client)
	}
	if _, err := hub.Join(context.Background(), "room", "participant", "ninth"); err != realtime.ErrRoomFull {
		t.Fatalf("ninth join error = %v, want %v", err, realtime.ErrRoomFull)
	}
	hub.Leave(context.Background(), clients[0])
	if _, err := hub.Join(context.Background(), "room", "participant", "replacement"); err != nil {
		t.Fatalf("replacement join: %v", err)
	}
}

func TestEnvelopeIsVersionedAndTimestamped(t *testing.T) {
	event := realtime.NewEnvelope("join.requested", "request-1", map[string]string{"name": "Ada"})
	if event.Version != 1 || event.Type != "join.requested" || event.RequestID != "request-1" {
		t.Fatalf("envelope = %#v", event)
	}
	if _, err := time.Parse(time.RFC3339Nano, event.Timestamp); err != nil {
		t.Fatalf("timestamp = %q: %v", event.Timestamp, err)
	}
}
