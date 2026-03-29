package table

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"
)

// Table is the server-owned representation of an active dashboard table.
type Table struct {
	ID        string
	SessionID string
	CreatedAt time.Time
}

// Manager stores active table metadata in memory.
type Manager struct {
	mu     sync.RWMutex
	tables map[string]*Table
	now    func() time.Time
}

// NewManager constructs an in-memory table registry.
func NewManager() *Manager {
	return &Manager{
		tables: make(map[string]*Table),
		now:    time.Now,
	}
}

// Create allocates a new table for the supplied session.
func (m *Manager) Create(sessionID string) (*Table, error) {
	id, err := newID("tbl_", 12)
	if err != nil {
		return nil, err
	}

	table := &Table{
		ID:        id,
		SessionID: sessionID,
		CreatedAt: m.now().UTC(),
	}

	m.mu.Lock()
	m.tables[id] = table
	m.mu.Unlock()

	return cloneTable(table), nil
}

// Get returns one table by identifier.
func (m *Manager) Get(id string) (*Table, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	table, ok := m.tables[id]
	if !ok {
		return nil, false
	}

	return cloneTable(table), true
}

// Delete removes a table from the registry.
func (m *Manager) Delete(id string) (*Table, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	table, ok := m.tables[id]
	if !ok {
		return nil, false
	}

	delete(m.tables, id)
	return cloneTable(table), true
}

func cloneTable(table *Table) *Table {
	if table == nil {
		return nil
	}

	return &Table{
		ID:        table.ID,
		SessionID: table.SessionID,
		CreatedAt: table.CreatedAt,
	}
}

func newID(prefix string, bytesLen int) (string, error) {
	buf := make([]byte, bytesLen)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}

	return prefix + hex.EncodeToString(buf), nil
}
