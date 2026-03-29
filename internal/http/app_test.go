package http_test

import (
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	apphttp "pokerlab/internal/http"
	"pokerlab/internal/templates"
)

func TestDashboardRenders(t *testing.T) {
	app := newTestApp(t)
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
	if !strings.Contains(body, `id="dashboard-content"`) {
		t.Fatalf("response missing dashboard content wrapper: %q", body)
	}
	if !strings.Contains(body, `id="tables-grid"`) {
		t.Fatalf("response missing tables grid container: %q", body)
	}
	if !strings.Contains(body, "0 / 8") {
		t.Fatalf("response missing initial table count: %q", body)
	}
}

func TestStaticCSSServed(t *testing.T) {
	app := newTestApp(t)
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

func TestCreateTableSetsCookieAndReturnsUpdatedDashboardContent(t *testing.T) {
	app := newTestApp(t)

	req := httptest.NewRequest(http.MethodPost, "/tables", nil)
	req.Header.Set("HX-Request", "true")
	rec := httptest.NewRecorder()

	app.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusCreated)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "Table created.") {
		t.Fatalf("response missing success flash: %q", body)
	}
	if !strings.Contains(body, `class="table-card`) {
		t.Fatalf("response missing table card markup: %q", body)
	}
	if !strings.Contains(body, "1 / 8") {
		t.Fatalf("response missing updated table count: %q", body)
	}
	if len(rec.Result().Cookies()) == 0 {
		t.Fatal("expected create request to set session cookie")
	}
}

func TestDashboardRestoresTablesForExistingSession(t *testing.T) {
	app := newTestApp(t)

	createReq := httptest.NewRequest(http.MethodPost, "/tables", nil)
	createRec := httptest.NewRecorder()
	app.Routes().ServeHTTP(createRec, createReq)

	cookies := createRec.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("expected session cookie from create request")
	}

	getReq := httptest.NewRequest(http.MethodGet, "/", nil)
	getReq.AddCookie(cookies[0])
	getRec := httptest.NewRecorder()

	app.Routes().ServeHTTP(getRec, getReq)

	if getRec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", getRec.Code, http.StatusOK)
	}
	if !strings.Contains(getRec.Body.String(), `class="table-card`) {
		t.Fatalf("expected restored table card, got: %q", getRec.Body.String())
	}
}

func TestDeleteTableRemovesOwnedTable(t *testing.T) {
	app := newTestApp(t)

	createReq := httptest.NewRequest(http.MethodPost, "/tables", nil)
	createRec := httptest.NewRecorder()
	app.Routes().ServeHTTP(createRec, createReq)

	cookies := createRec.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("expected session cookie from create request")
	}

	tableID := extractTableID(t, createRec.Body.String())

	deleteReq := httptest.NewRequest(http.MethodDelete, "/tables/"+tableID, nil)
	deleteReq.AddCookie(cookies[0])
	deleteRec := httptest.NewRecorder()

	app.Routes().ServeHTTP(deleteRec, deleteReq)

	if deleteRec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", deleteRec.Code, http.StatusOK)
	}

	body := deleteRec.Body.String()
	if !strings.Contains(body, "Table removed.") {
		t.Fatalf("response missing removal flash: %q", body)
	}
	if !strings.Contains(body, "No tables yet") {
		t.Fatalf("expected empty state after delete, got: %q", body)
	}
}

func TestCreateTableRejectsNinthTable(t *testing.T) {
	app := newTestApp(t)

	var sessionCookie *http.Cookie
	for i := 0; i < 8; i++ {
		req := httptest.NewRequest(http.MethodPost, "/tables", nil)
		if sessionCookie != nil {
			req.AddCookie(sessionCookie)
		}
		rec := httptest.NewRecorder()

		app.Routes().ServeHTTP(rec, req)

		if i == 0 {
			cookies := rec.Result().Cookies()
			if len(cookies) == 0 {
				t.Fatal("expected session cookie from first create request")
			}
			sessionCookie = cookies[0]
		}

		if rec.Code != http.StatusCreated {
			t.Fatalf("create request %d status = %d, want %d", i+1, rec.Code, http.StatusCreated)
		}
	}

	ninthReq := httptest.NewRequest(http.MethodPost, "/tables", nil)
	ninthReq.AddCookie(sessionCookie)
	ninthRec := httptest.NewRecorder()

	app.Routes().ServeHTTP(ninthRec, ninthReq)

	if ninthRec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want %d", ninthRec.Code, http.StatusConflict)
	}
	if !strings.Contains(ninthRec.Body.String(), "up to 8 active tables") {
		t.Fatalf("response missing max-table message: %q", ninthRec.Body.String())
	}
}

func newTestApp(t *testing.T) *apphttp.App {
	t.Helper()

	renderer, err := templates.New("web/templates/**/*.gohtml")
	if err != nil {
		t.Fatalf("templates.New() error = %v", err)
	}

	return apphttp.NewApp(renderer)
}

func extractTableID(t *testing.T, html string) string {
	t.Helper()

	re := regexp.MustCompile(`data-table-id="([^"]+)"`)
	matches := re.FindStringSubmatch(html)
	if len(matches) != 2 {
		t.Fatalf("failed to extract table ID from response: %q", html)
	}

	return matches[1]
}
