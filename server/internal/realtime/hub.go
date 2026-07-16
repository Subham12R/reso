package realtime

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	maxRoomSessions = 8
	presenceTTL     = 45 * time.Second
	outboundBuffer  = 32
)

var ErrRoomFull = errors.New("room has eight active sessions")

type Envelope struct {
	Version   int    `json:"version"`
	Type      string `json:"type"`
	Timestamp string `json:"timestamp"`
	RequestID string `json:"requestId,omitempty"`
	Payload   any    `json:"payload,omitempty"`
}

func NewEnvelope(eventType, requestID string, payload any) Envelope {
	return Envelope{Version: 1, Type: eventType, Timestamp: time.Now().UTC().Format(time.RFC3339Nano), RequestID: requestID, Payload: payload}
}

type Presence interface {
	Join(context.Context, string, string, time.Time, int) (bool, error)
	Heartbeat(context.Context, string, string, time.Time) error
	Leave(context.Context, string, string) error
}

type redisEvaler interface {
	Eval(context.Context, string, []string, ...interface{}) *redis.Cmd
}

type RedisPresence struct{ client redisEvaler }

func NewRedisPresence(client redisEvaler) *RedisPresence { return &RedisPresence{client: client} }

const joinScript = `
redis.call('ZREMRANGEBYSCORE', KEYS[1], '-inf', ARGV[1])
if redis.call('ZCARD', KEYS[1]) >= tonumber(ARGV[4]) then return 0 end
redis.call('ZADD', KEYS[1], ARGV[2], ARGV[3])
redis.call('EXPIRE', KEYS[1], ARGV[5])
return 1`

func (presence *RedisPresence) Join(ctx context.Context, roomID, sessionID string, expiresAt time.Time, maximum int) (bool, error) {
	result, err := presence.client.Eval(ctx, joinScript, []string{presenceKey(roomID)}, time.Now().UnixMilli(), expiresAt.UnixMilli(), sessionID, maximum, int64(presenceTTL/time.Second)).Int()
	return result == 1, err
}

const heartbeatScript = `
if not redis.call('ZSCORE', KEYS[1], ARGV[2]) then return 0 end
redis.call('ZADD', KEYS[1], ARGV[1], ARGV[2])
redis.call('EXPIRE', KEYS[1], ARGV[3])
return 1`

func (presence *RedisPresence) Heartbeat(ctx context.Context, roomID, sessionID string, expiresAt time.Time) error {
	updated, err := presence.client.Eval(ctx, heartbeatScript, []string{presenceKey(roomID)}, expiresAt.UnixMilli(), sessionID, int64(presenceTTL/time.Second)).Int()
	if err != nil {
		return err
	}
	if updated != 1 {
		return errors.New("presence expired")
	}
	return nil
}

func (presence *RedisPresence) Leave(ctx context.Context, roomID, sessionID string) error {
	return presence.client.Eval(ctx, `return redis.call('ZREM', KEYS[1], ARGV[1])`, []string{presenceKey(roomID)}, sessionID).Err()
}

func presenceKey(roomID string) string { return "reso:realtime:presence:" + roomID }

type Client struct {
	RoomID    string
	Role      string
	Identity  string
	SessionID string
	send      chan Envelope
	done      chan struct{}
	heartbeat chan struct{}
	once      sync.Once
}

func (client *Client) Events() <-chan Envelope { return client.send }
func (client *Client) Done() <-chan struct{}   { return client.done }
func (client *Client) Close()                  { client.once.Do(func() { close(client.done) }) }

func (client *Client) Send(event Envelope) bool {
	select {
	case <-client.done:
		return false
	case client.send <- event:
		return true
	default:
		client.Close()
		return false
	}
}

type Hub struct {
	mu          sync.Mutex
	presence    Presence
	rooms       map[string]map[*Client]struct{}
	ttl         time.Duration
	emptyTTL    time.Duration
	emptyTimers map[string]*time.Timer
	onEmpty     func(string)
}

func NewHub(client redisEvaler) *Hub { return NewHubWithPresence(NewRedisPresence(client)) }

func NewHubWithEmptyRoomCallback(client redisEvaler, emptyTTL time.Duration, onEmpty func(string)) *Hub {
	return newHub(NewRedisPresence(client), presenceTTL, emptyTTL, onEmpty)
}

func NewHubWithPresence(presence Presence) *Hub {
	return NewHubWithPresenceTTL(presence, presenceTTL)
}

func NewHubWithPresenceTTL(presence Presence, ttl time.Duration) *Hub {
	return newHub(presence, ttl, 0, nil)
}

func newHub(presence Presence, ttl, emptyTTL time.Duration, onEmpty func(string)) *Hub {
	return &Hub{presence: presence, rooms: make(map[string]map[*Client]struct{}), ttl: ttl, emptyTTL: emptyTTL, emptyTimers: make(map[string]*time.Timer), onEmpty: onEmpty}
}

func (hub *Hub) Join(ctx context.Context, roomID, role, identity string) (*Client, error) {
	sessionID, err := randomID()
	if err != nil {
		return nil, err
	}
	admitted, err := hub.presence.Join(ctx, roomID, sessionID, time.Now().Add(hub.ttl), maxRoomSessions)
	if err != nil {
		return nil, err
	}
	if !admitted {
		return nil, ErrRoomFull
	}
	client := &Client{RoomID: roomID, Role: role, Identity: identity, SessionID: sessionID, send: make(chan Envelope, outboundBuffer), done: make(chan struct{}), heartbeat: make(chan struct{}, 1)}
	hub.mu.Lock()
	if timer := hub.emptyTimers[roomID]; timer != nil {
		timer.Stop()
		delete(hub.emptyTimers, roomID)
	}
	if hub.rooms[roomID] == nil {
		hub.rooms[roomID] = make(map[*Client]struct{})
	}
	hub.rooms[roomID][client] = struct{}{}
	removed := hub.broadcastLocked(roomID, NewEnvelope("participant.joined", "", map[string]string{"participantId": identity, "role": role}))
	if role == "owner" {
		removed = append(removed, hub.broadcastLocked(roomID, NewEnvelope("stream.host.changed", "", map[string]string{"participantId": identity}))...)
	}
	hub.mu.Unlock()
	hub.cleanup(context.Background(), removed)
	go hub.watch(client)
	return client, nil
}

func (hub *Hub) Heartbeat(ctx context.Context, client *Client) error {
	if err := hub.presence.Heartbeat(ctx, client.RoomID, client.SessionID, time.Now().Add(hub.ttl)); err != nil {
		return err
	}
	select {
	case client.heartbeat <- struct{}{}:
	default:
	}
	return nil
}

func (hub *Hub) Leave(ctx context.Context, client *Client) {
	hub.remove(ctx, client)
}

func (hub *Hub) watch(client *Client) {
	timer := time.NewTimer(hub.ttl)
	defer timer.Stop()
	for {
		select {
		case <-client.Done():
			return
		case <-client.heartbeat:
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			timer.Reset(hub.ttl)
		case <-timer.C:
			hub.remove(context.Background(), client)
			return
		}
	}
}

func (hub *Hub) remove(ctx context.Context, client *Client) {
	hub.mu.Lock()
	removed := hub.removeLocked(client)
	hub.mu.Unlock()
	hub.cleanup(ctx, removed)
}

func (hub *Hub) removeLocked(client *Client) []*Client {
	clients := hub.rooms[client.RoomID]
	if _, found := clients[client]; !found {
		return nil
	}
	delete(clients, client)
	client.Close()
	if len(clients) == 0 {
		delete(hub.rooms, client.RoomID)
		hub.scheduleEmptyRoomLocked(client.RoomID)
	}
	removed := []*Client{client}
	removed = append(removed, hub.broadcastLocked(client.RoomID, NewEnvelope("participant.left", "", map[string]string{"participantId": client.Identity, "role": client.Role}))...)
	if client.Role == "owner" {
		for candidate := range clients {
			removed = append(removed, hub.broadcastLocked(client.RoomID, NewEnvelope("stream.host.changed", "", map[string]string{"participantId": candidate.Identity}))...)
			break
		}
	}
	return removed
}

func (hub *Hub) scheduleEmptyRoomLocked(roomID string) {
	if hub.emptyTTL <= 0 || hub.onEmpty == nil {
		return
	}
	hub.emptyTimers[roomID] = time.AfterFunc(hub.emptyTTL, func() {
		hub.mu.Lock()
		_, occupied := hub.rooms[roomID]
		delete(hub.emptyTimers, roomID)
		hub.mu.Unlock()
		if !occupied {
			hub.onEmpty(roomID)
		}
	})
}

func (hub *Hub) cleanup(ctx context.Context, clients []*Client) {
	for _, client := range clients {
		_ = hub.presence.Leave(ctx, client.RoomID, client.SessionID)
	}
}

func (hub *Hub) Publish(roomID, eventType, requestID string, payload any) {
	hub.mu.Lock()
	removed := hub.broadcastLocked(roomID, NewEnvelope(eventType, requestID, payload))
	hub.mu.Unlock()
	hub.cleanup(context.Background(), removed)
}

func (hub *Hub) broadcastLocked(roomID string, event Envelope) []*Client {
	var removed []*Client
	for client := range hub.rooms[roomID] {
		if !client.Send(event) {
			removed = append(removed, hub.removeLocked(client)...)
		}
	}
	return removed
}

func randomID() (string, error) {
	value := make([]byte, 16)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return hex.EncodeToString(value), nil
}
