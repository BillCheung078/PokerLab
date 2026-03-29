package http

import (
	"errors"
	"log"
	stdhttp "net/http"
	"path/filepath"
	"runtime"
	"strings"

	"pokerlab/internal/session"
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
	ID         string
	Label      string
	Status     string
	Owner      string
	ShortID    string
	GameNumber int
	BlindLevel string
	EventCount int
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
}

// NewApp constructs the Phase 1 application shell.
func NewApp(renderer *templates.Renderer) *App {
	return &App{
		renderer:  renderer,
		staticDir: filepath.Join(projectRoot(), "web", "static"),
		sessions:  session.NewManager(),
		tables:    table.NewManager(),
	}
}

// Routes returns the application HTTP handler tree.
func (a *App) Routes() stdhttp.Handler {
	mux := stdhttp.NewServeMux()
	mux.HandleFunc("GET /", a.handleDashboard)
	mux.HandleFunc("POST /tables", a.handleCreateTable)
	mux.HandleFunc("DELETE /tables/{id}", a.handleDeleteTable)
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
		if runtime, ok := a.tables.Runtime(tableID); ok {
			snapshot := runtime.Snapshot()
			status = humanizeRuntimeStatus(snapshot.State.Status)
			gameNumber = snapshot.State.GameNumber
			blindLevel = snapshot.State.BlindLevel
			eventCount = len(snapshot.History)
		}

		tables = append(tables, TableViewData{
			ID:         tbl.ID,
			Label:      "Table " + strings.ToUpper(shortID(tbl.ID)),
			Status:     status,
			Owner:      "Current session",
			ShortID:    strings.ToUpper(shortID(tbl.ID)),
			GameNumber: gameNumber,
			BlindLevel: blindLevel,
			EventCount: eventCount,
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

func projectRoot() string {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return "."
	}

	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}
