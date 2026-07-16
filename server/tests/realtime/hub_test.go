package realtime_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/subham12r/reso/internal/realtime"
)

type memoryPresence struct {
	mu       sync.Mutex
	sessions map[string]time.Time
}

func (p *memoryPresence) Join(_ context.Context, _, sessionID string, expiresAt time.Time, maximum int) (bool, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
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
	p.mu.Lock()
	defer p.mu.Unlock()
	p.sessions[sessionID] = expiresAt
	return nil
}

func (p *memoryPresence) Leave(_ context.Context, _, sessionID string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
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

func TestHubExpiresClientWithoutApplicationHeartbeat(t *testing.T) {
	hub := realtime.NewHubWithPresenceTTL(&memoryPresence{}, 50*time.Millisecond)
	observer, err := hub.Join(context.Background(), "room", "participant", "observer")
	if err != nil {
		t.Fatal(err)
	}
	defer hub.Leave(context.Background(), observer)
	<-observer.Events()
	expiring, err := hub.Join(context.Background(), "room", "participant", "expiring")
	if err != nil {
		t.Fatal(err)
	}
	<-observer.Events()
	<-expiring.Events()
	keepAlive, cancel := context.WithCancel(context.Background())
	defer cancel()
	heartbeatErr := make(chan error, 1)
	go func() {
		ticker := time.NewTicker(10 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-keepAlive.Done():
				return
			case <-ticker.C:
				if err := hub.Heartbeat(context.Background(), observer); err != nil {
					heartbeatErr <- err
					return
				}
			}
		}
	}()

	select {
	case <-expiring.Done():
	case <-time.After(time.Second):
		t.Fatal("client remained connected after application heartbeat expiry")
	}
	select {
	case event := <-observer.Events():
		if event.Type != "participant.left" {
			t.Fatalf("expiry event = %q, want participant.left", event.Type)
		}
	case err := <-heartbeatErr:
		t.Fatalf("keep observer alive: %v", err)
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for participant.left")
	}
}

func TestSlowConsumerEvictionUsesParticipantLifecycle(t *testing.T) {
	hub := realtime.NewHubWithPresence(&memoryPresence{})
	slowOwner, err := hub.Join(context.Background(), "room", "owner", "owner-id")
	if err != nil {
		t.Fatal(err)
	}
	observer, err := hub.Join(context.Background(), "room", "participant", "observer-id")
	if err != nil {
		t.Fatal(err)
	}
	defer hub.Leave(context.Background(), observer)
	<-observer.Events()

	seenLeft := false
	seenHostChange := false
	for index := 0; index < 40; index++ {
		hub.Publish("room", "test.event", "", index)
		event := <-observer.Events()
		seenLeft = seenLeft || event.Type == "participant.left"
		seenHostChange = seenHostChange || event.Type == "stream.host.changed"
	}
	for {
		select {
		case event := <-observer.Events():
			seenLeft = seenLeft || event.Type == "participant.left"
			seenHostChange = seenHostChange || event.Type == "stream.host.changed"
		default:
			goto drained
		}
	}
drained:
	select {
	case <-slowOwner.Done():
	default:
		t.Fatal("slow owner was not disconnected")
	}
	if !seenLeft || !seenHostChange {
		t.Fatalf("slow eviction events: participant.left=%v stream.host.changed=%v", seenLeft, seenHostChange)
	}
}
