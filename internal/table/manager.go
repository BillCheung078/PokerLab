package table

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"

	"pokerlab/internal/sim"
)

// Table is the server-owned representation of an active dashboard table.
type Table struct {
	ID        string
	SessionID string
	CreatedAt time.Time
}

type managedTable struct {
	table   *Table
	runtime *sim.TableRuntime
}

// Manager stores active table metadata in memory.
type Manager struct {
	mu            sync.RWMutex
	tables        map[string]*managedTable
	now           func() time.Time
	engine        *sim.Engine
	runtimeConfig sim.RuntimeConfig
}

// NewManager constructs an in-memory table registry.
func NewManager() *Manager {
	return NewManagerWithConfig(sim.NewEngine(), sim.RuntimeConfig{})
}

// NewManagerWithEngine constructs an in-memory table registry with an explicit simulator.
func NewManagerWithEngine(engine *sim.Engine) *Manager {
	return NewManagerWithConfig(engine, sim.RuntimeConfig{})
}

// NewManagerWithConfig constructs an in-memory table registry with explicit runtime settings.
func NewManagerWithConfig(engine *sim.Engine, runtimeConfig sim.RuntimeConfig) *Manager {
	if engine == nil {
		engine = sim.NewEngine()
	}

	return &Manager{
		tables:        make(map[string]*managedTable),
		now:           time.Now,
		engine:        engine,
		runtimeConfig: runtimeConfig,
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
	runtime := sim.NewTableRuntimeWithConfig(table.ID, table.SessionID, table.CreatedAt, m.runtimeConfig)

	m.mu.Lock()
	m.tables[id] = &managedTable{
		table:   table,
		runtime: runtime,
	}
	m.mu.Unlock()

	m.engine.Start(runtime)

	return cloneTable(table), nil
}

// Get returns one table by identifier.
func (m *Manager) Get(id string) (*Table, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	entry, ok := m.tables[id]
	if !ok {
		return nil, false
	}

	return cloneTable(entry.table), true
}

// Runtime returns the owned runtime for one table.
func (m *Manager) Runtime(id string) (*sim.TableRuntime, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	entry, ok := m.tables[id]
	if !ok {
		return nil, false
	}

	return entry.runtime, true
}

// Delete removes a table from the registry.
func (m *Manager) Delete(id string) (*Table, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	entry, ok := m.tables[id]
	if !ok {
		return nil, false
	}

	entry.runtime.Cancel()
	delete(m.tables, id)
	return cloneTable(entry.table), true
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
