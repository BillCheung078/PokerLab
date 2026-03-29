package http_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	apphttp "pokerlab/internal/http"
	"pokerlab/internal/session"
	"pokerlab/internal/sim"
	"pokerlab/internal/table"
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
	if !strings.Contains(body, `/static/js/app.js`) {
		t.Fatalf("response missing app bootstrap script: %q", body)
	}
	if !strings.Contains(body, "alpinejs") {
		t.Fatalf("response missing Alpine.js script include: %q", body)
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
	if !strings.Contains(body, `x-data="tableStream(`) {
		t.Fatalf("response missing Alpine table stream component: %q", body)
	}
	if !strings.Contains(body, "Live Feed") {
		t.Fatalf("response missing live feed section: %q", body)
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

func TestSessionStreamReplaysHistoryAndReceivesLiveEvents(t *testing.T) {
	app, _, tables := newFastTestApp(t)

	firstTableID, cookie := createTableWithHandler(t, app)
	secondTableID := createTableWithCookie(t, app, cookie)

	firstRuntime, ok := tables.Runtime(firstTableID)
	if !ok {
		t.Fatalf("expected runtime for table %q", firstTableID)
	}
	secondRuntime, ok := tables.Runtime(secondTableID)
	if !ok {
		t.Fatalf("expected runtime for table %q", secondTableID)
	}
	waitFor(t, time.Second, func() bool {
		return len(firstRuntime.Snapshot().History) >= 2 && len(secondRuntime.Snapshot().History) >= 2
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := httptest.NewRequest(http.MethodGet, "/stream", nil).WithContext(ctx)
	req.AddCookie(cookie)
	rec := newStreamingRecorder()

	done := make(chan struct{})
	go func() {
		app.Routes().ServeHTTP(rec, req)
		close(done)
	}()

	waitFor(t, time.Second, func() bool { return rec.MessageCount() >= 4 })

	if rec.StatusCode() != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.StatusCode(), http.StatusOK)
	}
	if got := rec.Header().Get("Content-Type"); !strings.Contains(got, "text/event-stream") {
		t.Fatalf("Content-Type = %q, want text/event-stream", got)
	}

	messages := parseSSEMessages(rec.BodyString())
	seenTableIDs := make(map[string]bool)
	for _, message := range messages {
		var replayEvent streamedPokerEvent
		if err := json.Unmarshal([]byte(message.Data), &replayEvent); err != nil {
			t.Fatalf("json.Unmarshal replay event error = %v", err)
		}
		seenTableIDs[replayEvent.TableID] = true
	}
	if !seenTableIDs[firstTableID] || !seenTableIDs[secondTableID] {
		t.Fatalf("shared stream replay missing table IDs: %#v", seenTableIDs)
	}

	initialCount := rec.MessageCount()
	waitFor(t, time.Second, func() bool { return rec.MessageCount() > initialCount })

	cancel()
	waitForClosed(t, done, time.Second)
}

func TestSessionStreamOnlyIncludesCurrentSessionTables(t *testing.T) {
	app, _, _ := newFastTestApp(t)

	firstTableID, _ := createTableWithHandler(t, app)
	secondTableID, otherCookie := createTableWithHandler(t, app)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := httptest.NewRequest(http.MethodGet, "/stream", nil).WithContext(ctx)
	req.AddCookie(otherCookie)
	rec := newStreamingRecorder()

	done := make(chan struct{})
	go func() {
		app.Routes().ServeHTTP(rec, req)
		close(done)
	}()

	waitFor(t, time.Second, func() bool { return rec.MessageCount() >= 1 })
	messages := parseSSEMessages(rec.BodyString())
	seenSecondTable := false
	for _, message := range messages {
		var event streamedPokerEvent
		if err := json.Unmarshal([]byte(message.Data), &event); err != nil {
			t.Fatalf("json.Unmarshal stream event error = %v", err)
		}
		if event.TableID == firstTableID {
			t.Fatalf("shared stream leaked first session table %q into second session stream", firstTableID)
		}
		if event.TableID == secondTableID {
			seenSecondTable = true
		}
	}
	if !seenSecondTable {
		t.Fatalf("expected second session table %q in stream body: %q", secondTableID, rec.BodyString())
	}

	cancel()
	waitForClosed(t, done, time.Second)
}

func TestSessionStreamRejectsMissingSessionWithoutSettingCookie(t *testing.T) {
	app, _, _ := newFastTestApp(t)

	createTableWithHandler(t, app)

	req := httptest.NewRequest(http.MethodGet, "/stream", nil)
	rec := httptest.NewRecorder()

	app.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
	if len(rec.Result().Cookies()) != 0 {
		t.Fatalf("unexpected cookies set on rejected stream request: %#v", rec.Result().Cookies())
	}
}

func TestSessionStreamDisconnectCleansUpSubscribers(t *testing.T) {
	app, _, tables := newFastTestApp(t)

	firstTableID, cookie := createTableWithHandler(t, app)
	secondTableID := createTableWithCookie(t, app, cookie)

	firstRuntime, ok := tables.Runtime(firstTableID)
	if !ok {
		t.Fatalf("expected runtime for table %q", firstTableID)
	}
	secondRuntime, ok := tables.Runtime(secondTableID)
	if !ok {
		t.Fatalf("expected runtime for table %q", secondTableID)
	}

	ctx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequest(http.MethodGet, "/stream", nil).WithContext(ctx)
	req.AddCookie(cookie)
	rec := newStreamingRecorder()

	done := make(chan struct{})
	go func() {
		app.Routes().ServeHTTP(rec, req)
		close(done)
	}()

	waitFor(t, time.Second, func() bool { return rec.MessageCount() >= 1 })

	waitFor(t, time.Second, func() bool {
		return firstRuntime.Snapshot().SubscriberCount == 1 && secondRuntime.Snapshot().SubscriberCount == 1
	})

	cancel()
	waitForClosed(t, done, time.Second)

	waitFor(t, time.Second, func() bool {
		return firstRuntime.Snapshot().SubscriberCount == 0 && secondRuntime.Snapshot().SubscriberCount == 0
	})
}

func TestRefreshRestoresTablesAndReconnectsSharedStream(t *testing.T) {
	app, _, tables := newFastTestApp(t)

	tableID, cookie := createTableWithHandler(t, app)

	runtime, ok := tables.Runtime(tableID)
	if !ok {
		t.Fatalf("expected runtime for table %q", tableID)
	}
	waitFor(t, time.Second, func() bool { return len(runtime.Snapshot().History) >= 2 })

	getReq := httptest.NewRequest(http.MethodGet, "/", nil)
	getReq.AddCookie(cookie)
	getRec := httptest.NewRecorder()

	app.Routes().ServeHTTP(getRec, getReq)

	if getRec.Code != http.StatusOK {
		t.Fatalf("GET / status = %d, want %d", getRec.Code, http.StatusOK)
	}
	if !strings.Contains(getRec.Body.String(), `data-table-id="`+tableID+`"`) {
		t.Fatalf("restored dashboard missing table %q: %q", tableID, getRec.Body.String())
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := httptest.NewRequest(http.MethodGet, "/stream", nil).WithContext(ctx)
	req.AddCookie(cookie)
	rec := newStreamingRecorder()

	done := make(chan struct{})
	go func() {
		app.Routes().ServeHTTP(rec, req)
		close(done)
	}()

	waitFor(t, time.Second, func() bool { return rec.MessageCount() >= 2 })

	initialCount := rec.MessageCount()
	waitFor(t, time.Second, func() bool { return rec.MessageCount() > initialCount })

	cancel()
	waitForClosed(t, done, time.Second)
}

func newTestApp(t *testing.T) *apphttp.App {
	t.Helper()

	renderer, err := templates.New("web/templates/**/*.gohtml")
	if err != nil {
		t.Fatalf("templates.New() error = %v", err)
	}

	return apphttp.NewApp(renderer)
}

func newFastTestApp(t *testing.T) (*apphttp.App, *session.Manager, *table.Manager) {
	t.Helper()

	renderer, err := templates.New("web/templates/**/*.gohtml")
	if err != nil {
		t.Fatalf("templates.New() error = %v", err)
	}

	sessions := session.NewManager()
	tables := table.NewManagerWithEngine(sim.NewEngineWithConfig(sim.EngineConfig{
		IntraHandDelay: 5 * time.Millisecond,
		HandPause:      5 * time.Millisecond,
	}))

	return apphttp.NewAppWithServices(renderer, sessions, tables), sessions, tables
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

func createTableWithHandler(t *testing.T, app *apphttp.App) (string, *http.Cookie) {
	t.Helper()

	req := httptest.NewRequest(http.MethodPost, "/tables", nil)
	rec := httptest.NewRecorder()

	app.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("POST /tables status = %d, want %d, body = %q", rec.Code, http.StatusCreated, rec.Body.String())
	}

	cookies := rec.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("expected session cookie from create request")
	}

	return extractTableID(t, rec.Body.String()), cookies[0]
}

func createTableWithCookie(t *testing.T, app *apphttp.App, cookie *http.Cookie) string {
	t.Helper()

	req := httptest.NewRequest(http.MethodPost, "/tables", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()

	app.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("POST /tables with cookie status = %d, want %d, body = %q", rec.Code, http.StatusCreated, rec.Body.String())
	}

	return extractTableID(t, rec.Body.String())
}

type streamedPokerEvent struct {
	ID      string         `json:"id"`
	TableID string         `json:"table_id"`
	Type    string         `json:"type"`
	Payload map[string]any `json:"payload"`
	At      time.Time      `json:"at"`
}

func waitFor(t *testing.T, timeout time.Duration, predicate func() bool) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if predicate() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}

	t.Fatal("timed out waiting for condition")
}

func waitForClosed(t *testing.T, done <-chan struct{}, timeout time.Duration) {
	t.Helper()

	select {
	case <-done:
	case <-time.After(timeout):
		t.Fatal("timed out waiting for handler to stop")
	}
}

type streamingRecorder struct {
	header http.Header
	body   bytes.Buffer
	code   int
	mu     sync.Mutex
}

func newStreamingRecorder() *streamingRecorder {
	return &streamingRecorder{
		header: make(http.Header),
	}
}

func (r *streamingRecorder) Header() http.Header {
	return r.header
}

func (r *streamingRecorder) Write(data []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.code == 0 {
		r.code = http.StatusOK
	}

	return r.body.Write(data)
}

func (r *streamingRecorder) WriteHeader(statusCode int) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.code = statusCode
}

func (r *streamingRecorder) Flush() {}

func (r *streamingRecorder) StatusCode() int {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.code == 0 {
		return http.StatusOK
	}

	return r.code
}

func (r *streamingRecorder) BodyString() string {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.body.String()
}

func (r *streamingRecorder) MessageCount() int {
	return len(parseSSEMessages(r.BodyString()))
}

type sseMessage struct {
	ID    string
	Event string
	Data  string
}

func parseSSEMessages(raw string) []sseMessage {
	blocks := strings.Split(raw, "\n\n")
	messages := make([]sseMessage, 0, len(blocks))

	for _, block := range blocks {
		block = strings.TrimSpace(block)
		if block == "" || strings.HasPrefix(block, ":") {
			continue
		}

		var msg sseMessage
		for _, line := range strings.Split(block, "\n") {
			switch {
			case strings.HasPrefix(line, "id: "):
				msg.ID = strings.TrimPrefix(line, "id: ")
			case strings.HasPrefix(line, "event: "):
				msg.Event = strings.TrimPrefix(line, "event: ")
			case strings.HasPrefix(line, "data: "):
				msg.Data = strings.TrimPrefix(line, "data: ")
			}
		}
		if msg.ID != "" || msg.Event != "" || msg.Data != "" {
			messages = append(messages, msg)
		}
	}

	return messages
}
