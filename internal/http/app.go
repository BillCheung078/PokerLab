package http

import (
	"log"
	stdhttp "net/http"
	"path/filepath"
	"runtime"

	"pokerlab/internal/templates"
)

// DashboardPageData is the view model for the dashboard shell.
type DashboardPageData struct {
	Title string
}

// App wires the renderer and HTTP routes together.
type App struct {
	renderer  *templates.Renderer
	staticDir string
}

// NewApp constructs the Phase 1 application shell.
func NewApp(renderer *templates.Renderer) *App {
	return &App{
		renderer:  renderer,
		staticDir: filepath.Join(projectRoot(), "web", "static"),
	}
}

// Routes returns the application HTTP handler tree.
func (a *App) Routes() stdhttp.Handler {
	mux := stdhttp.NewServeMux()
	mux.HandleFunc("/", a.handleDashboard)
	mux.Handle("/static/", stdhttp.StripPrefix("/static/", stdhttp.FileServer(stdhttp.Dir(a.staticDir))))

	return mux
}

func (a *App) handleDashboard(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	if r.URL.Path != "/" {
		stdhttp.NotFound(w, r)
		return
	}
	if r.Method != stdhttp.MethodGet {
		w.Header().Set("Allow", stdhttp.MethodGet)
		stdhttp.Error(w, stdhttp.StatusText(stdhttp.StatusMethodNotAllowed), stdhttp.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	data := DashboardPageData{
		Title: "Poker Table Streaming Dashboard",
	}

	if err := a.renderer.Render(w, "dashboard", data); err != nil {
		log.Printf("render dashboard: %v", err)
		stdhttp.Error(w, stdhttp.StatusText(stdhttp.StatusInternalServerError), stdhttp.StatusInternalServerError)
	}
}

func projectRoot() string {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return "."
	}

	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}
