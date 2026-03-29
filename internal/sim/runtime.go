package sim

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"sync"
	"time"
)

const (
	// DefaultHistoryLimit bounds in-memory event history per table runtime.
	DefaultHistoryLimit = 64
	// DefaultSubscriberBuffer bounds each subscriber channel.
	DefaultSubscriberBuffer = 16
)

var (
	// ErrRuntimeStopped is returned when a caller tries to subscribe after cancellation.
	ErrRuntimeStopped = errors.New("table runtime stopped")
)

// PokerEvent is a simulated event emitted by a table runtime.
type PokerEvent struct {
	ID      string         `json:"id"`
	TableID string         `json:"table_id"`
	Type    string         `json:"type"`
	Payload map[string]any `json:"payload,omitempty"`
	At      time.Time      `json:"at"`
}

// PlayerSeat stores one player slot in the materialized table state.
type PlayerSeat struct {
	Seat   int
	Name   string
	Stack  int
	Status string
}

// TableState stores the latest table view derived from events.
type TableState struct {
	TableID        string
	GameNumber     int
	BlindLevel     string
	Players        []PlayerSeat
	CommunityCards []string
	LastEventAt    time.Time
	Status         string
}

// RuntimeSnapshot is an immutable view of one runtime at a point in time.
type RuntimeSnapshot struct {
	ID              string
	SessionID       string
	CreatedAt       time.Time
	State           TableState
	History         []PokerEvent
	TotalEvents     int
	SubscriberCount int
	Running         bool
}

// TableRuntime owns one table's lifecycle and mutable runtime state.
type TableRuntime struct {
	ID        string
	SessionID string
	CreatedAt time.Time

	ctx    context.Context
	cancel context.CancelFunc

	startOnce        sync.Once
	stopOnce         sync.Once
	mu               sync.RWMutex
	state            TableState
	history          []PokerEvent
	totalEvents      int
	historyCap       int
	subscriberBuffer int
	subscribers      map[string]chan PokerEvent
}

// NewTableRuntime constructs a cancellable runtime for one active table.
func NewTableRuntime(id, sessionID string, createdAt time.Time) *TableRuntime {
	return NewTableRuntimeWithConfig(id, sessionID, createdAt, RuntimeConfig{})
}

// RuntimeConfig controls bounded in-memory runtime behavior.
type RuntimeConfig struct {
	HistoryLimit     int
	SubscriberBuffer int
}

// NewTableRuntimeWithConfig constructs a runtime with explicit in-memory limits.
func NewTableRuntimeWithConfig(id, sessionID string, createdAt time.Time, config RuntimeConfig) *TableRuntime {
	if config.HistoryLimit <= 0 {
		config.HistoryLimit = DefaultHistoryLimit
	}
	if config.SubscriberBuffer <= 0 {
		config.SubscriberBuffer = DefaultSubscriberBuffer
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &TableRuntime{
		ID:        id,
		SessionID: sessionID,
		CreatedAt: createdAt.UTC(),
		ctx:       ctx,
		cancel:    cancel,
		state: TableState{
			TableID:    id,
			BlindLevel: "1/2",
			Status:     "runtime_initialized",
		},
		historyCap:       config.HistoryLimit,
		subscriberBuffer: config.SubscriberBuffer,
		subscribers:      make(map[string]chan PokerEvent),
	}
}

// Snapshot returns an immutable copy of the runtime's current state.
func (r *TableRuntime) Snapshot() RuntimeSnapshot {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return RuntimeSnapshot{
		ID:              r.ID,
		SessionID:       r.SessionID,
		CreatedAt:       r.CreatedAt,
		State:           cloneState(r.state),
		History:         cloneHistory(r.history),
		TotalEvents:     r.totalEvents,
		SubscriberCount: len(r.subscribers),
		Running:         r.ctx.Err() == nil,
	}
}

// Context returns the runtime lifecycle context.
func (r *TableRuntime) Context() context.Context {
	return r.ctx
}

// Cancel stops the runtime and releases subscriber channels.
func (r *TableRuntime) Cancel() {
	r.stopOnce.Do(func() {
		r.cancel()

		r.mu.Lock()
		r.state.Status = "runtime_stopped"
		for id := range r.subscribers {
			delete(r.subscribers, id)
		}
		r.mu.Unlock()
	})
}

// UpdateState mutates the runtime state under lock.
func (r *TableRuntime) UpdateState(update func(*TableState)) {
	r.mu.Lock()
	defer r.mu.Unlock()

	update(&r.state)
}

// AppendEvent records one event, updates basic state markers, and fans out to subscribers.
func (r *TableRuntime) AppendEvent(event PokerEvent) {
	if event.ID == "" {
		id, err := newID("evt_", 12)
		if err == nil {
			event.ID = id
		}
	}
	if event.TableID == "" {
		event.TableID = r.ID
	}
	if event.At.IsZero() {
		event.At = time.Now().UTC()
	}

	r.mu.Lock()
	r.state.LastEventAt = event.At
	if r.state.Status == "" || r.state.Status == "runtime_initialized" {
		r.state.Status = "runtime_ready"
	}
	r.totalEvents++
	r.history = append(r.history, cloneEvent(event))
	if len(r.history) > r.historyCap {
		r.history = append([]PokerEvent(nil), r.history[len(r.history)-r.historyCap:]...)
	}

	subscribers := make([]chan PokerEvent, 0, len(r.subscribers))
	for _, ch := range r.subscribers {
		subscribers = append(subscribers, ch)
	}
	r.mu.Unlock()

	for _, ch := range subscribers {
		select {
		case ch <- cloneEvent(event):
		default:
		}
	}
}

// Subscribe registers one buffered subscriber channel for future streaming.
func (r *TableRuntime) Subscribe(buffer int) (string, <-chan PokerEvent, error) {
	if r.ctx.Err() != nil {
		return "", nil, ErrRuntimeStopped
	}
	if buffer <= 0 {
		buffer = r.subscriberBuffer
	}

	id, err := newID("sub_", 8)
	if err != nil {
		return "", nil, err
	}

	ch := make(chan PokerEvent, buffer)

	r.mu.Lock()
	defer r.mu.Unlock()

	if r.ctx.Err() != nil {
		close(ch)
		return "", nil, ErrRuntimeStopped
	}

	r.subscribers[id] = ch
	return id, ch, nil
}

// Unsubscribe removes one subscriber by identifier.
func (r *TableRuntime) Unsubscribe(id string) {
	r.mu.Lock()
	if _, ok := r.subscribers[id]; ok {
		delete(r.subscribers, id)
	}
	r.mu.Unlock()
}

func cloneState(state TableState) TableState {
	return TableState{
		TableID:        state.TableID,
		GameNumber:     state.GameNumber,
		BlindLevel:     state.BlindLevel,
		Players:        append([]PlayerSeat(nil), state.Players...),
		CommunityCards: append([]string(nil), state.CommunityCards...),
		LastEventAt:    state.LastEventAt,
		Status:         state.Status,
	}
}

func cloneHistory(history []PokerEvent) []PokerEvent {
	cloned := make([]PokerEvent, 0, len(history))
	for _, event := range history {
		cloned = append(cloned, cloneEvent(event))
	}

	return cloned
}

func cloneEvent(event PokerEvent) PokerEvent {
	cloned := PokerEvent{
		ID:      event.ID,
		TableID: event.TableID,
		Type:    event.Type,
		At:      event.At,
	}

	if len(event.Payload) != 0 {
		cloned.Payload = make(map[string]any, len(event.Payload))
		for key, value := range event.Payload {
			cloned.Payload[key] = value
		}
	}

	return cloned
}

func newID(prefix string, bytesLen int) (string, error) {
	buf := make([]byte, bytesLen)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}

	return prefix + hex.EncodeToString(buf), nil
}
