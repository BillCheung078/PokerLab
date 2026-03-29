package http

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	stdhttp "net/http"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"pokerlab/internal/session"
	"pokerlab/internal/sim"
	"pokerlab/internal/table"
	"pokerlab/internal/templates"
)

// DashboardPageData is the view model for the dashboard shell.
type DashboardPageData struct {
	Title      string
	Tables     []TableViewData
	TableCount int
	MaxTables  int
	Flash      *FlashMessage
}

// TableViewData is the server-rendered table card view model.
type TableViewData struct {
	ID          string
	Label       string
	Status      string
	Owner       string
	ShortID     string
	GameNumber  int
	BlindLevel  string
	EventCount  int
	LastEventAt string
}

// FlashMessage is a small request-scoped UI message.
type FlashMessage struct {
	Kind    string
	Message string
}

// App wires the renderer and HTTP routes together.
type App struct {
	renderer  *templates.Renderer
	staticDir string
	sessions  *session.Manager
	tables    *table.Manager
	config    Config
}

// Config controls HTTP-layer stream behavior.
type Config struct {
	StreamHeartbeatInterval time.Duration
	StreamReplayLimit       int
	StreamSubscriberBuffer  int
}

// NewApp constructs the Phase 1 application shell.
func NewApp(renderer *templates.Renderer) *App {
	return NewAppWithServicesAndConfig(renderer, session.NewManager(), table.NewManager(), Config{})
}

// NewAppWithServices constructs the application with explicit backing services.
func NewAppWithServices(renderer *templates.Renderer, sessions *session.Manager, tables *table.Manager) *App {
	return NewAppWithServicesAndConfig(renderer, sessions, tables, Config{})
}

// NewAppWithServicesAndConfig constructs the application with explicit backing services and stream tuning.
func NewAppWithServicesAndConfig(renderer *templates.Renderer, sessions *session.Manager, tables *table.Manager, config Config) *App {
	if sessions == nil {
		sessions = session.NewManager()
	}
	if tables == nil {
		tables = table.NewManager()
	}
	if config.StreamHeartbeatInterval <= 0 {
		config.StreamHeartbeatInterval = 15 * time.Second
	}
	if config.StreamReplayLimit <= 0 {
		config.StreamReplayLimit = sim.DefaultHistoryLimit
	}
	if config.StreamSubscriberBuffer <= 0 {
		config.StreamSubscriberBuffer = sim.DefaultSubscriberBuffer
	}

	return &App{
		renderer:  renderer,
		staticDir: filepath.Join(projectRoot(), "web", "static"),
		sessions:  sessions,
		tables:    tables,
		config:    config,
	}
}

// Routes returns the application HTTP handler tree.
func (a *App) Routes() stdhttp.Handler {
	mux := stdhttp.NewServeMux()
	mux.HandleFunc("GET /", a.handleDashboard)
	mux.HandleFunc("POST /tables", a.handleCreateTable)
	mux.HandleFunc("DELETE /tables/{id}", a.handleDeleteTable)
	mux.HandleFunc("GET /stream", a.handleSessionStream)
	mux.Handle("GET /static/", stdhttp.StripPrefix("/static/", stdhttp.FileServer(stdhttp.Dir(a.staticDir))))

	return mux
}

func (a *App) handleDashboard(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	sess, err := a.sessions.GetOrCreate(w, r)
	if err != nil {
		stdhttp.Error(w, stdhttp.StatusText(stdhttp.StatusInternalServerError), stdhttp.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	data := a.dashboardData(sess.ID, nil)

	if err := a.renderer.Render(w, "dashboard", data); err != nil {
		log.Printf("render dashboard: %v", err)
		stdhttp.Error(w, stdhttp.StatusText(stdhttp.StatusInternalServerError), stdhttp.StatusInternalServerError)
	}
}

func (a *App) handleCreateTable(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	sess, err := a.sessions.GetOrCreate(w, r)
	if err != nil {
		stdhttp.Error(w, stdhttp.StatusText(stdhttp.StatusInternalServerError), stdhttp.StatusInternalServerError)
		return
	}

	if err := a.createTable(sess.ID); err != nil {
		if errors.Is(err, session.ErrMaxTablesReached) {
			a.renderDashboardContent(w, stdhttp.StatusConflict, sess.ID, &FlashMessage{
				Kind:    "error",
				Message: "You can only keep up to 8 active tables in one dashboard session.",
			})
			return
		}

		log.Printf("create table: %v", err)
		stdhttp.Error(w, stdhttp.StatusText(stdhttp.StatusInternalServerError), stdhttp.StatusInternalServerError)
		return
	}

	a.renderDashboardContent(w, stdhttp.StatusCreated, sess.ID, &FlashMessage{
		Kind:    "success",
		Message: "Table created.",
	})
}

func (a *App) handleDeleteTable(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	sess, err := a.sessions.GetOrCreate(w, r)
	if err != nil {
		stdhttp.Error(w, stdhttp.StatusText(stdhttp.StatusInternalServerError), stdhttp.StatusInternalServerError)
		return
	}

	tableID := strings.TrimSpace(r.PathValue("id"))
	if tableID == "" {
		stdhttp.NotFound(w, r)
		return
	}

	existing, ok := a.tables.Get(tableID)
	if !ok {
		a.renderDashboardContent(w, stdhttp.StatusNotFound, sess.ID, &FlashMessage{
			Kind:    "error",
			Message: "That table no longer exists.",
		})
		return
	}
	if existing.SessionID != sess.ID || !a.sessions.OwnsTable(sess.ID, tableID) {
		a.renderDashboardContent(w, stdhttp.StatusForbidden, sess.ID, &FlashMessage{
			Kind:    "error",
			Message: "You can only remove tables owned by the current dashboard session.",
		})
		return
	}

	a.tables.Delete(tableID)
	a.sessions.RemoveTable(sess.ID, tableID)

	a.renderDashboardContent(w, stdhttp.StatusOK, sess.ID, &FlashMessage{
		Kind:    "success",
		Message: "Table removed.",
	})
}

func (a *App) handleSessionStream(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	sess, ok := a.sessions.Current(r)
	if !ok {
		stdhttp.Error(w, stdhttp.StatusText(stdhttp.StatusForbidden), stdhttp.StatusForbidden)
		return
	}

	flusher, ok := w.(stdhttp.Flusher)
	if !ok {
		stdhttp.Error(w, "streaming unsupported", stdhttp.StatusInternalServerError)
		return
	}

	// SSE responses are intentionally long-lived, so they must not inherit the
	// server-wide write deadline used for ordinary request/response handlers.
	controller := stdhttp.NewResponseController(w)
	if err := controller.SetWriteDeadline(time.Time{}); err != nil && !errors.Is(err, stdhttp.ErrNotSupported) {
		log.Printf("disable stream write deadline: %v", err)
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	type runtimeSubscription struct {
		id      string
		runtime *sim.TableRuntime
		ch      <-chan sim.PokerEvent
	}

	tableIDs := a.sessions.ListTableIDs(sess.ID)
	subscriptions := make([]runtimeSubscription, 0, len(tableIDs))
	replay := make([]sim.PokerEvent, 0, len(tableIDs)*a.config.StreamReplayLimit)

	for _, tableID := range tableIDs {
		tbl, ok := a.tables.Get(tableID)
		if !ok || tbl.SessionID != sess.ID {
			continue
		}

		runtime, ok := a.tables.Runtime(tableID)
		if !ok {
			continue
		}

		subID, ch, err := runtime.Subscribe(a.config.StreamSubscriberBuffer)
		if err != nil {
			continue
		}

		subscriptions = append(subscriptions, runtimeSubscription{
			id:      subID,
			runtime: runtime,
			ch:      ch,
		})
		history := runtime.Snapshot().History
		if len(history) > a.config.StreamReplayLimit {
			history = history[len(history)-a.config.StreamReplayLimit:]
		}
		replay = append(replay, history...)
	}

	defer func() {
		for _, subscription := range subscriptions {
			subscription.runtime.Unsubscribe(subscription.id)
		}
	}()

	sort.SliceStable(replay, func(i, j int) bool {
		if replay[i].At.Equal(replay[j].At) {
			return replay[i].ID < replay[j].ID
		}
		return replay[i].At.Before(replay[j].At)
	})

	replayed := make(map[string]struct{}, len(replay))
	for _, event := range replay {
		if err := writeSSEEvent(w, event); err != nil {
			return
		}
		replayed[event.ID] = struct{}{}
	}
	flusher.Flush()

	liveEvents := make(chan sim.PokerEvent, max(1, len(subscriptions))*a.config.StreamSubscriberBuffer)
	var forwarders sync.WaitGroup
	for _, subscription := range subscriptions {
		forwarders.Add(1)
		go func(subscription runtimeSubscription) {
			defer forwarders.Done()

			for {
				select {
				case <-r.Context().Done():
					return
				case <-subscription.runtime.Context().Done():
					return
				case event, ok := <-subscription.ch:
					if !ok {
						return
					}
					select {
					case liveEvents <- event:
					case <-r.Context().Done():
						return
					case <-subscription.runtime.Context().Done():
						return
					}
				}
			}
		}(subscription)
	}

	heartbeat := time.NewTicker(a.config.StreamHeartbeatInterval)
	defer heartbeat.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-heartbeat.C:
			if _, err := fmt.Fprint(w, ": keep-alive\n\n"); err != nil {
				return
			}
			flusher.Flush()
		case event, ok := <-liveEvents:
			if !ok {
				return
			}
			if _, seen := replayed[event.ID]; seen {
				delete(replayed, event.ID)
				continue
			}
			if err := writeSSEEvent(w, event); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

func (a *App) createTable(sessionID string) error {
	if a.sessions.Count(sessionID) >= a.sessions.MaxTables() {
		return session.ErrMaxTablesReached
	}

	created, err := a.tables.Create(sessionID)
	if err != nil {
		return err
	}
	if err := a.sessions.AddTable(sessionID, created.ID); err != nil {
		a.tables.Delete(created.ID)
		return err
	}

	return nil
}

func (a *App) renderDashboardContent(w stdhttp.ResponseWriter, status int, sessionID string, flash *FlashMessage) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)

	if err := a.renderer.Render(w, "dashboard-content", a.dashboardData(sessionID, flash)); err != nil {
		log.Printf("render dashboard content: %v", err)
	}
}

func (a *App) dashboardData(sessionID string, flash *FlashMessage) DashboardPageData {
	return DashboardPageData{
		Title:      "Poker Table Streaming Dashboard",
		Tables:     a.collectTables(sessionID),
		TableCount: a.sessions.Count(sessionID),
		MaxTables:  a.sessions.MaxTables(),
		Flash:      flash,
	}
}

func (a *App) collectTables(sessionID string) []TableViewData {
	tableIDs := a.sessions.ListTableIDs(sessionID)
	tables := make([]TableViewData, 0, len(tableIDs))

	for _, tableID := range tableIDs {
		tbl, ok := a.tables.Get(tableID)
		if !ok || tbl.SessionID != sessionID {
			a.sessions.RemoveTable(sessionID, tableID)
			continue
		}

		status := "Runtime unavailable"
		gameNumber := 0
		blindLevel := ""
		eventCount := 0
		lastEventAt := ""
		if runtime, ok := a.tables.Runtime(tableID); ok {
			snapshot := runtime.Snapshot()
			status = humanizeRuntimeStatus(snapshot.State.Status)
			gameNumber = snapshot.State.GameNumber
			blindLevel = snapshot.State.BlindLevel
			eventCount = snapshot.TotalEvents
			if !snapshot.State.LastEventAt.IsZero() {
				lastEventAt = snapshot.State.LastEventAt.Format(time.RFC3339Nano)
			}
		}

		tables = append(tables, TableViewData{
			ID:          tbl.ID,
			Label:       "Table " + strings.ToUpper(shortID(tbl.ID)),
			Status:      status,
			Owner:       "Current session",
			ShortID:     strings.ToUpper(shortID(tbl.ID)),
			GameNumber:  gameNumber,
			BlindLevel:  blindLevel,
			EventCount:  eventCount,
			LastEventAt: lastEventAt,
		})
	}

	return tables
}

func shortID(value string) string {
	value = strings.TrimPrefix(value, "tbl_")
	if len(value) <= 6 {
		return value
	}

	return value[:6]
}

func humanizeRuntimeStatus(status string) string {
	switch status {
	case "runtime_initialized":
		return "Runtime initialized"
	case "runtime_ready":
		return "Runtime ready"
	case "starting_hand":
		return "Starting hand"
	case "players_ready":
		return "Players seated"
	case "preflop":
		return "Preflop cards dealt"
	case "preflop_betting":
		return "Preflop betting"
	case "flop":
		return "Flop dealt"
	case "flop_betting":
		return "Flop betting"
	case "turn":
		return "Turn dealt"
	case "turn_betting":
		return "Turn betting"
	case "river":
		return "River dealt"
	case "runtime_stopped":
		return "Runtime stopped"
	case "hand_complete":
		return "Hand complete"
	default:
		return "Runtime pending"
	}
}

func writeSSEEvent(w stdhttp.ResponseWriter, event sim.PokerEvent) error {
	payload, err := json.Marshal(event)
	if err != nil {
		return err
	}

	if _, err := fmt.Fprintf(w, "id: %s\nevent: %s\ndata: %s\n\n", event.ID, event.Type, payload); err != nil {
		return err
	}

	return nil
}

func projectRoot() string {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return "."
	}

	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}
