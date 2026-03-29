package http_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	apphttp "pokerlab/internal/http"
	"pokerlab/internal/templates"
)

func TestDashboardRenders(t *testing.T) {
	renderer, err := templates.New("web/templates/**/*.gohtml")
	if err != nil {
		t.Fatalf("templates.New() error = %v", err)
	}

	app := apphttp.NewApp(renderer)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	app.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "Add Table") {
		t.Fatalf("response missing Add Table button: %q", body)
	}
	if !strings.Contains(body, `id="tables-grid"`) {
		t.Fatalf("response missing tables grid container: %q", body)
	}
}

func TestStaticCSSServed(t *testing.T) {
	renderer, err := templates.New("web/templates/**/*.gohtml")
	if err != nil {
		t.Fatalf("templates.New() error = %v", err)
	}

	app := apphttp.NewApp(renderer)
	req := httptest.NewRequest(http.MethodGet, "/static/css/app.css", nil)
	rec := httptest.NewRecorder()

	app.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	body := rec.Body.String()
	if !strings.Contains(body, ".dashboard-shell") {
		t.Fatalf("response missing expected css content: %q", body)
	}
}
