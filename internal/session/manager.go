package session

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/http"
	"slices"
	"sync"
	"time"
)

const (
	// DefaultCookieName is the browser cookie used to bind a dashboard session.
	DefaultCookieName = "pokerlab_session"
	// DefaultMaxTables is the assignment-imposed cap per browser session.
	DefaultMaxTables = 8
)

var (
	// ErrMaxTablesReached is returned when a session attempts to exceed the cap.
	ErrMaxTablesReached = errors.New("maximum active table limit reached")
	// ErrSessionNotFound is returned when a session identifier is unknown.
	ErrSessionNotFound = errors.New("session not found")
)

// Session tracks the ordered set of active tables for one browser session.
type Session struct {
	ID        string
	TableIDs  []string
	UpdatedAt time.Time
}

// Manager stores sessions in memory and manages the session cookie lifecycle.
type Manager struct {
	mu         sync.RWMutex
	sessions   map[string]*Session
	cookieName string
	maxTables  int
	now        func() time.Time
}

// NewManager constructs a session manager with local-development defaults.
func NewManager() *Manager {
	return &Manager{
		sessions:   make(map[string]*Session),
		cookieName: DefaultCookieName,
		maxTables:  DefaultMaxTables,
		now:        time.Now,
	}
}

// MaxTables returns the per-session cap enforced by the manager.
func (m *Manager) MaxTables() int {
	return m.maxTables
}

// GetOrCreate returns the current session and creates a cookie-backed session if needed.
func (m *Manager) GetOrCreate(w http.ResponseWriter, r *http.Request) (*Session, error) {
	if cookie, err := r.Cookie(m.cookieName); err == nil && cookie.Value != "" {
		if sess, ok := m.get(cookie.Value); ok {
			return sess, nil
		}
	}

	id, err := newID("sess_", 16)
	if err != nil {
		return nil, err
	}

	sess := &Session{
		ID:        id,
		TableIDs:  []string{},
		UpdatedAt: m.now().UTC(),
	}

	m.mu.Lock()
	m.sessions[id] = sess
	m.mu.Unlock()

	http.SetCookie(w, &http.Cookie{
		Name:     m.cookieName,
		Value:    id,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})

	return cloneSession(sess), nil
}

// AddTable appends a table to the ordered session list if capacity allows.
func (m *Manager) AddTable(sessionID, tableID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	sess, ok := m.sessions[sessionID]
	if !ok {
		return ErrSessionNotFound
	}
	if slices.Contains(sess.TableIDs, tableID) {
		sess.UpdatedAt = m.now().UTC()
		return nil
	}
	if len(sess.TableIDs) >= m.maxTables {
		return ErrMaxTablesReached
	}

	sess.TableIDs = append(sess.TableIDs, tableID)
	sess.UpdatedAt = m.now().UTC()

	return nil
}

// RemoveTable removes a table from the session ordering if present.
func (m *Manager) RemoveTable(sessionID, tableID string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	sess, ok := m.sessions[sessionID]
	if !ok {
		return false
	}

	index := slices.Index(sess.TableIDs, tableID)
	if index == -1 {
		return false
	}

	sess.TableIDs = append(sess.TableIDs[:index], sess.TableIDs[index+1:]...)
	sess.UpdatedAt = m.now().UTC()

	return true
}

// ListTableIDs returns the session table identifiers in creation order.
func (m *Manager) ListTableIDs(sessionID string) []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	sess, ok := m.sessions[sessionID]
	if !ok {
		return nil
	}

	return append([]string(nil), sess.TableIDs...)
}

// Count returns how many active tables belong to the session.
func (m *Manager) Count(sessionID string) int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	sess, ok := m.sessions[sessionID]
	if !ok {
		return 0
	}

	return len(sess.TableIDs)
}

// OwnsTable reports whether the table belongs to the given session.
func (m *Manager) OwnsTable(sessionID, tableID string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	sess, ok := m.sessions[sessionID]
	if !ok {
		return false
	}

	return slices.Contains(sess.TableIDs, tableID)
}

func (m *Manager) get(sessionID string) (*Session, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	sess, ok := m.sessions[sessionID]
	if !ok {
		return nil, false
	}

	return cloneSession(sess), true
}

func cloneSession(sess *Session) *Session {
	if sess == nil {
		return nil
	}

	return &Session{
		ID:        sess.ID,
		TableIDs:  append([]string(nil), sess.TableIDs...),
		UpdatedAt: sess.UpdatedAt,
	}
}

func newID(prefix string, bytesLen int) (string, error) {
	buf := make([]byte, bytesLen)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}

	return prefix + hex.EncodeToString(buf), nil
}
