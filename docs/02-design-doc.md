# Design Doc

## Title

Poker Table Streaming Dashboard

## Status

Draft v1

## Purpose

This document presents the proposed architecture for the Poker Table Streaming Dashboard technical assignment. It defines the engineering problem, establishes the goals and non-goals, describes the recommended design, compares alternative approaches, and records the trade-offs that justify the final solution.

---

## 1. Problem Statement

The assignment requires a lightweight web dashboard capable of displaying multiple live poker tables simultaneously, where each table receives a continuous sequence of simulated poker events from a Go backend. The frontend must use HTMX and Alpine.js. The system must support dynamic table creation and removal, enforce a maximum of eight tables, provide reasonable reconnect or state restoration behavior after page refresh, and maintain clean, testable handling of concurrent runtimes and resource cleanup.

This is fundamentally a problem of managing multiple independent server-side event producers and delivering their output efficiently and safely to a browser UI.

---

## 2. Goals

### 2.1 Primary goals

- Support dynamic creation of poker tables from the dashboard.
- Enforce a maximum of 8 active tables per browser session.
- Provide an independent event sequence for each table while delivering browser updates through one shared SSE connection per session.
- Clean up all associated server-side resources when a table is removed.
- Restore active tables after page refresh and reconnect their streams or show last known state.
- Keep the implementation readable, maintainable, and well documented.

### 2.2 Secondary goals

- Make the communication choice easy to defend in review.
- Keep the frontend lightweight and aligned with the required stack.
- Preserve a clean separation between HTTP handling, simulation, runtime management, and rendering.

---

## 3. Non-Goals

The following are intentionally out of scope:

- real poker hand evaluation
- accurate betting or pot calculation rules
- persistent database storage
- production-grade deployment infrastructure
- horizontal scaling across multiple server instances
- authentication and authorization
- advanced visual table rendering beyond a clean minimal interface

These exclusions are deliberate because the assignment explicitly prioritizes architecture, streaming, cleanup, and code quality over UI polish and full game logic.

---

## 4. Design Constraints

The assignment creates several important constraints that guide the architecture.

### 4.1 Required stack constraint

The solution must use:

- Go for the backend
- HTMX for server-driven interactions
- Alpine.js for lightweight client-side state

### 4.2 Read-heavy communication constraint

The system is described as read-heavy. That implies the dominant traffic pattern is server-to-client event delivery rather than client-driven state mutation.

### 4.3 Isolation constraint

Each table must stream an independent sequence of events. Therefore, each table should be modeled as a runtime unit that can be created, canceled, and reconnected independently.

### 4.4 Local execution constraint

The solution only needs to run locally. This reduces the need for durable persistence or distributed coordination.

### 4.5 Cleanup visibility constraint

Because resource management is explicitly called out, the design should expose cancellation and cleanup semantics clearly enough to test and explain.

---

## 5. Success Criteria

A successful solution should demonstrate:

- correct add/remove table behavior
- independent table streams
- correct cleanup of goroutines and subscribers
- practical refresh/reconnect behavior
- a justified transport choice
- clean package boundaries and testability
- documentation that explains the architecture and alternatives clearly

---

## 6. Architecture Summary

The recommended design is a **server-owned table runtime architecture** using **Server-Sent Events (SSE)** for streaming.

At a high level:

- the browser renders a dashboard shell
- HTMX handles structural interactions such as creating and removing table cards
- Alpine.js manages per-table client state and subscribes cards to one shared session-level `EventSource`
- the server creates one runtime per active table
- each runtime owns its simulation loop, current state, recent history, and subscriber registry
- SSE delivers a one-way multiplexed live stream for the current session, with each event carrying its `table_id`

This architecture intentionally favors clarity and lifecycle control over unnecessary flexibility.

---

## 7. System Context

```text
Browser
  ├─ GET /
  ├─ HTMX POST /tables
  ├─ HTMX DELETE /tables/{id}
  ├─ Alpine component per table
  └─ Shared EventSource per session

Go Server
  ├─ HTTP router and handlers
  ├─ template rendering
  ├─ SessionManager
  ├─ TableManager
  ├─ TableRuntime (one per table)
  │    ├─ simulation loop
  │    ├─ current table state
  │    ├─ recent event history
  │    └─ subscriber registry
  └─ session-scoped SSE stream endpoint
```

---

## 8. Detailed Problem Breakdown and Proposed Solutions

### 8.1 Problem: How should live events be delivered from server to browser?

#### Chosen solution
Use **Server-Sent Events (SSE)**.

#### Why this solves the problem well
- The event flow is naturally server → client.
- The system is read-heavy.
- SSE provides browser-native streaming with automatic reconnection support.
- SSE avoids the overhead of implementing a custom bidirectional messaging protocol.
- A single multiplexed SSE connection avoids browser HTTP/1.1 per-origin connection limits that would otherwise be hit by one EventSource per table.
- This keeps the design robust in local development without depending on HTTP/2 availability.

#### Alternative 1: WebSockets

**Advantages**
- full duplex communication
- extensible for future interactive table controls

**Disadvantages**
- more lifecycle complexity
- protocol flexibility not required by the assignment
- additional code and testing burden for little practical gain

**Trade-off**
- WebSockets would provide more generality, but the added generality is not justified by the current requirements.

#### Alternative 2: Polling / long polling

**Advantages**
- simple request/response model

**Disadvantages**
- inefficient for continuous event feeds
- either stale UI or excessive request volume
- poorer user experience for frequent updates

**Trade-off**
- Polling would reduce implementation complexity in the smallest sense, but at the cost of correctness of interaction style and efficiency for streaming.

#### Final decision
SSE is the best balance of correctness, simplicity, and reviewability for this assignment.

### 8.1.1 Problem: Should the browser open one stream per table or multiplex through one connection?

#### Chosen solution
Use one shared SSE connection per browser session and multiplex events by `table_id`.

#### Why this solves the problem well
- Browser implementations commonly limit concurrent HTTP/1.1 connections per origin to a small number such as six.
- The assignment requires support for up to eight tables, so one EventSource per table would make the browser-side limit part of the system design.
- A shared connection preserves independent server-side runtimes while removing the client-side transport bottleneck.
- It is more robust than assuming HTTP/2 will always be available in local development.

#### Alternative 1: One EventSource per table

**Advantages**
- very direct mapping from browser component to backend route
- simpler fan-out logic on the client

**Disadvantages**
- fragile under HTTP/1.1 browser connection limits
- can starve normal HTMX requests once enough tables are open
- makes the assignment's eight-table cap unreliable in practice

#### Alternative 2: Depend on HTTP/2 multiplexing

**Advantages**
- would allow many concurrent streams over one transport connection

**Disadvantages**
- adds deployment and local-environment assumptions
- weakens the design narrative because correctness depends on transport negotiation rather than application structure

#### Final decision
Use a single browser SSE connection and multiplex events in the application layer. This is the more robust design and easier to defend in review.

---

### 8.2 Problem: How should a table be represented on the backend?

#### Chosen solution
Represent each active table as a **TableRuntime** object with its own cancellation context, simulator loop, subscriber registry, current state, and recent history.

#### Why this solves the problem well
- makes lifecycle explicit
- preserves table isolation
- maps directly to add/remove/reconnect operations
- makes cleanup testable

#### Alternative 1: Central global simulator with routing logic

A single central loop could simulate all tables and route events to table-specific clients.

**Advantages**
- fewer goroutines
- centralized event generation

**Disadvantages**
- harder to reason about isolation
- cancellation becomes indirect
- increased routing and bookkeeping complexity
- harder to explain to reviewers

**Trade-off**
- This may be more efficient at large scale, but large scale is not the target here. Clear table-local ownership is more valuable.

#### Alternative 2: Table state without long-lived runtimes

The system could store table metadata and generate events only when clients poll or connect.

**Advantages**
- potentially fewer background processes

**Disadvantages**
- less faithful to continuous live table behavior
- reconnect semantics become more awkward
- more complex event continuity logic

**Trade-off**
- Avoiding long-lived runtimes would reduce background execution but make the behavior less natural and lifecycle logic harder to model.

#### Final decision
A dedicated runtime per table is the clearest abstraction and best demonstrates concurrency management.

---

### 8.3 Problem: How should active tables be associated with a browser session?

#### Chosen solution
Use a server-issued session cookie plus an in-memory **SessionManager** that maps session IDs to active table IDs.

#### Why this solves the problem well
- keeps the server as source of truth
- supports refresh restoration naturally
- allows ownership validation on stream and delete requests
- simplifies enforcement of the max-table rule

#### Alternative 1: Client-only table tracking

Use browser storage only and treat the browser as source of truth.

**Advantages**
- simple to implement
- no dedicated server session management

**Disadvantages**
- stale table IDs are harder to reconcile
- ownership validation is weaker
- reconnect to existing server runtimes becomes less reliable

**Trade-off**
- Slightly simpler initial implementation, but worse lifecycle clarity and correctness.

#### Alternative 2: Persistent database-backed sessions

**Advantages**
- durable across process restarts
- more production-like

**Disadvantages**
- unnecessary for a local-only assignment
- introduces schema, storage, and failure complexity

**Trade-off**
- Better durability, but much worse complexity-to-value ratio for this exercise.

#### Final decision
In-memory server-side session management is the most appropriate scope fit.

---

### 8.4 Problem: How should reconnect or page refresh be handled?

#### Chosen solution
Use a **hybrid continuity model**:

- server-side session state as source of truth for active tables
- table runtime keeps current state and bounded recent history
- optional client-side `sessionStorage` for cosmetic fallback while reconnect occurs

#### Why this solves the problem well
- active tables can be restored reliably after refresh
- reconnect does not require persistent storage
- recent history can reduce visible discontinuity
- last-known-state can be displayed quickly even before full reconnect

#### Alternative 1: Pure server-side restoration only

**Advantages**
- cleaner source of truth
- simpler client logic

**Disadvantages**
- brief blank/loading state on refresh until reconnect completes

**Trade-off**
- Simpler and probably acceptable. This is a valid fallback if time becomes tight.

#### Alternative 2: Pure client-side restoration only

**Advantages**
- minimal server logic

**Disadvantages**
- stale state risk
- weaker ownership guarantees
- difficult reconciliation with removed/expired table runtimes

**Trade-off**
- Faster to build, but worse system integrity.

#### Final decision
Hybrid continuity offers the best balance of UX and architectural clarity, but pure server-side restoration remains the preferred fallback if scope must be reduced.

---

### 8.5 Problem: How should client-side UI responsibilities be divided between HTMX and Alpine.js?

#### Chosen solution
Use a **hybrid HTMX + Alpine.js model**:

- HTMX for table creation/removal and server-rendered partial insertion
- Alpine.js for table-local event state and connection status on top of one shared session stream manager

#### Why this solves the problem well
- HTMX excels at request/response DOM updates
- Alpine.js is more natural for local reactive state tied to an EventSource
- avoids forcing HTMX into long-lived streaming state management
- avoids introducing a heavier SPA framework

#### Alternative 1: HTMX only

**Advantages**
- fewer moving parts on the client

**Disadvantages**
- long-lived stream state becomes awkward
- connection status and local event list handling become less natural

**Trade-off**
- Lower conceptual surface area, but poorer ergonomics for streaming UI behavior.

#### Alternative 2: Larger SPA framework

**Advantages**
- rich client state management

**Disadvantages**
- violates the spirit of the assignment stack
- unnecessary complexity

**Trade-off**
- Better tooling for large applications, but clearly out of scope here.

#### Final decision
The HTMX + Alpine.js split is the strongest match for the required stack and the interaction model.

---

### 8.6 Problem: How should slow subscribers be prevented from blocking a table?

#### Chosen solution
Use a buffered channel per subscriber and non-blocking or bounded fan-out behavior.

#### Why this solves the problem well
- protects simulation loop throughput
- avoids allowing one client to stall the whole table
- supports explicit disconnect/drop behavior for unhealthy subscribers

#### Alternative 1: Direct synchronous write to all subscribers

**Advantages**
- simpler implementation

**Disadvantages**
- slow subscriber can stall the runtime
- backpressure can cascade into simulation delay

**Trade-off**
- Simpler code at the cost of a serious lifecycle risk.

#### Alternative 2: Dedicated goroutine per subscriber with unbounded buffering

**Advantages**
- decouples writes

**Disadvantages**
- risk of memory growth
- more complexity to manage and clean up

**Trade-off**
- Better isolation, but too expensive for a local exercise if unbounded.

#### Final decision
Buffered bounded channels are the most practical compromise.

---

## 9. Proposed Runtime and Data Model

### 9.1 Session

```go
// Session tracks active tables for a browser session.
type Session struct {
    ID        string
    TableIDs  []string
    UpdatedAt time.Time
}
```

### 9.2 PokerEvent

```go
// PokerEvent is a simulated event emitted by a table runtime.
type PokerEvent struct {
    ID      string
    TableID string
    Type    string
    Payload map[string]any
    At      time.Time
}
```

### 9.3 TableState

```go
// TableState stores the most recent materialized table view.
type TableState struct {
    TableID        string
    GameNumber     int
    BlindLevel     string
    Players        []PlayerSeat
    CommunityCards []string
    LastEventAt    time.Time
    Status         string
}
```

### 9.4 TableRuntime

```go
// TableRuntime owns one table's lifecycle and live stream state.
type TableRuntime struct {
    ID          string
    SessionID   string
    Ctx         context.Context
    Cancel      context.CancelFunc

    State       TableState
    History     []PokerEvent
    Subscribers map[string]chan PokerEvent

    Mu          sync.RWMutex
}
```

### Why this model is appropriate

This model keeps operational ownership explicit. Table lifecycle, state evolution, event production, and streaming delivery all converge around one object, making the design easier to test and explain.

---

## 10. Backend Components

### 10.1 HTTP layer

Responsibilities:

- serve the dashboard page
- create tables
- remove tables
- serve SSE stream endpoints
- optionally serve snapshot or restore endpoints

Suggested routes:

| Method | Path | Responsibility |
|---|---|---|
| GET | `/` | render dashboard and restored tables |
| POST | `/tables` | create a new table |
| DELETE | `/tables/{id}` | remove a table |
| GET | `/stream` | open the shared SSE stream for the current session |
| GET | `/session/tables` | optional restore endpoint |

### 10.2 SessionManager

Responsibilities:

- create/retrieve session cookie
- map session → active tables
- enforce max-table rule
- support session restoration

### 10.3 TableManager

Responsibilities:

- create runtimes
- look up runtimes
- remove runtimes
- validate ownership

### 10.4 Simulation engine

Responsibilities:

- emit repeated event sequences
- apply randomized timing
- update materialized table state
- append to bounded history

---

## 11. Client-Side Design

### 11.1 HTMX responsibilities

- add table action
- remove table action
- insert server-rendered table card partials
- optionally refresh restored table fragments

### 11.2 Alpine.js responsibilities

- initialize table-local state
- subscribe table cards to the shared session `EventSource`
- append incoming events to UI state
- show connection state such as `connecting`, `live`, and `disconnected`
- optionally cache last-known-state in `sessionStorage`

### 11.3 Why this split is correct

This division keeps the browser code lightweight but purposeful. HTMX handles server-shaped HTML workflows; Alpine handles local state that is continuous and event-driven.

---

## 12. Core Runtime Flows

### 12.1 Add table flow

1. User clicks **Add Table**.
2. HTMX sends `POST /tables`.
3. Server validates session and table count.
4. Server creates `TableRuntime`.
5. Simulation loop starts.
6. Server returns rendered table card HTML.
7. Browser inserts the table card.
8. Alpine registers the new table card with the shared SSE manager, which refreshes the session stream if needed.

### 12.2 Stream flow

1. Browser opens `GET /stream`.
2. Server validates the current session.
3. Subscriber channels are registered with each active table runtime for that session.
4. Optional recent history replay is sent for all active tables.
5. New events stream in real time over one shared connection.
6. The browser dispatches each event to the correct table card using `table_id`.
7. Client disconnect unregisters the session stream subscribers.

### 12.3 Remove table flow

1. User clicks remove.
2. HTMX sends `DELETE /tables/{id}`.
3. Server validates ownership.
4. Runtime cancellation is triggered.
5. Simulation loop exits.
6. Subscribers are dropped.
7. Table is removed from manager and session state.
8. HTMX removes the table card from the DOM.

### 12.4 Refresh / reconnect flow

1. Browser refreshes page.
2. Session cookie is preserved.
3. Server restores active tables for the session.
4. Table cards are re-rendered.
5. Alpine reconnects the shared session EventSource.
6. Recent history or last-known-state fills the gap.

---

## 13. Simulation Design

The simulation engine does not need real poker correctness. It only needs believable event flow.

### Proposed repeated sequence

1. `game_started`
2. `players_joined`
3. one or more `card_dealt`
4. one or more `bet_action`
5. `community_cards` for flop
6. further `bet_action`
7. `community_cards` for turn
8. further `bet_action`
9. `community_cards` for river
10. `hand_result`
11. short delay
12. repeat with next game number

### Timing strategy

- short randomized intervals between intra-hand events
- slightly longer delay between hands

### Trade-off

A more complex simulator could produce more realistic flow, but realism does not materially improve the evaluation outcome. The better investment is reliability, clarity, and test coverage.

---

## 14. Error Handling Strategy

### Primary error cases

- creation of a 9th table
- request for non-existent table
- request for table outside current session ownership
- reconnect to expired/removed runtime

### Proposed HTTP behavior

- `409 Conflict` for max-table limit
- `404 Not Found` for missing table
- `403 Forbidden` for ownership violations when relevant
- small, user-readable HTML fragments for HTMX requests

### Why this is appropriate

The assignment is local and UI-minimal. Error handling should remain simple but explicit.

---

## 15. Concurrency Model

### 15.1 Per-table runtime concurrency

Each table runtime gets a dedicated simulation goroutine.

### 15.2 Per-stream connection concurrency

Each session stream request uses its own request lifecycle and fan-in registration across that session's active runtimes.

### 15.3 Shared state protection

A mutex should protect:

- current table state
- recent history
- subscriber registry

### 15.4 Why this model is acceptable

The hard cap of eight tables keeps the runtime scale small. Therefore the most important property is not maximizing efficiency; it is making concurrent ownership and cleanup obvious and safe.

---

## 16. Resource Management Design

Resource management is a first-class concern in this assignment.

### 16.1 Cleanup guarantees on table removal

When a table is removed:

- its context is canceled
- the simulation loop exits
- subscriber channels are deregistered
- the runtime is removed from the manager
- the table ID is removed from session state

### 16.2 Cleanup guarantees on client disconnect

When a stream connection drops:

- the HTTP request context completes
- the subscriber is removed from the registry
- the table runtime remains alive if the table still exists

### 16.3 Leak prevention

The implementation should avoid:

- orphaned goroutines
- unbounded event history
- stale subscriber channels
- blocked simulation writes

### Trade-off

Adding stronger lifecycle instrumentation increases code volume slightly, but it substantially improves review confidence and testability.

---

## 17. Testing Strategy

### 17.1 Unit tests

#### SessionManager
- creates sessions
- restores sessions
- enforces max 8 tables

#### TableManager
- creates runtimes
- removes runtimes cleanly
- validates ownership

#### Simulation engine
- emits valid event sequence
- updates table state
- stops on cancellation

#### Streaming registry
- registers subscribers
- unregisters on disconnect
- does not block table runtime on slow subscriber

### 17.2 Handler tests

Using `httptest`:

- `GET /`
- `POST /tables`
- `DELETE /tables/{id}`
- validation around `GET /stream`

### 17.3 Optional integration tests

- add table → receive events
- remove table → stream terminates
- refresh with session cookie → tables restored

### Trade-off

Deep end-to-end testing would be useful, but for an assignment the highest value comes from strong unit coverage of lifecycle logic plus a few key handler tests.

---

## 18. Package Structure

Recommended structure:

```text
cmd/server
internal/http
internal/session
internal/sim
internal/table
internal/templates
web/static
web/templates
docs
```

### Why this structure works

It separates transport concerns from domain/runtime concerns and keeps simulation isolated from HTTP logic. This makes the code easier to read and easier to test.

---

## 19. Trade-off Summary Table

| Decision Area | Chosen Approach | Rejected Alternative(s) | Main Trade-off |
|---|---|---|---|
| Streaming transport | SSE | WebSockets, polling | Less flexible than WebSockets, but much simpler and better aligned with one-way streaming |
| Table lifecycle | Runtime per table | Central global simulator | Slightly more goroutines, but far better isolation and cleanup clarity |
| Session continuity | Server-side session + optional client fallback | Client-only or DB-backed session state | Less durable than DB-backed persistence, but much simpler and sufficient for local execution |
| Frontend state split | HTMX + Alpine.js hybrid | HTMX-only or SPA framework | Slightly more moving parts than HTMX-only, but much better ergonomics for stream state |
| State storage | In-memory | Persistent database | No restart durability, but much lower complexity |
| Stream topology | One shared session stream with application-level multiplexing | One stream per table, transport-level HTTP/2 multiplexing | Slightly more client fan-out logic, but far more robust under browser HTTP/1.1 connection limits |

---

## 20. Known Limitations

- State is in-memory and lost on server restart.
- Reconnect continuity is limited to a running backend process.
- The design targets local execution and clarity rather than horizontal scale.
- Event replay is bounded rather than durable.
- UI remains intentionally minimal.

---

## 21. Future Enhancements

If the implementation were extended beyond the assignment:

- persistent session and table state
- richer poker-table visual rendering
- replay using last event IDs
- metrics and diagnostics for active runtimes/subscribers
- Redis-backed pub/sub for distributed streaming
- stronger observability and tracing

---

## 22. Final Recommendation

The recommended solution is:

- SSE for live event streaming
- a server-owned `TableRuntime` per table
- a `SessionManager` to restore active tables after refresh
- HTMX for structural UI interactions
- Alpine.js for table-local streaming state on top of one shared session SSE connection
- in-memory state with bounded history and explicit cleanup paths

This design is the strongest fit for the assignment because it solves the required problem directly, keeps complexity proportional to scope, and provides a clear narrative around concurrency, resource management, and maintainability.
