# Initial Notes - Internal Audit Used Only

## 1. Purpose of This Document

This document captures the initial technical framing of the assignment before design and implementation begins. Its purpose is to convert the assignment prompt into a clear engineering problem statement, identify the hidden design constraints, and establish the most appropriate solution direction for the final implementation.

This is not the final architecture specification. It is the working requirements and design-framing document that informs the design document and roadmap.

---

## 2. Assignment Summary

The assignment requires a browser-based dashboard that can display multiple poker tables at once, where each table shows its own independent, real-time stream of simulated poker events. The implementation must use Go for the backend and HTMX plus Alpine.js for the frontend. The dashboard must support dynamic table creation, table removal, resource cleanup, and reconnection or last-known-state restoration when the page is refreshed or reopened. The solution should also include tests and clear documentation.

---

## 3. Restated Problem Definition

At a practical level, this assignment is not primarily a UI exercise and not primarily a poker-logic exercise. It is a streaming systems exercise in the context of a lightweight web application.

The underlying engineering problem can be stated more precisely as follows:

> Design and implement a small, maintainable system that can create up to eight independent server-side table runtimes, stream their simulated event output efficiently to the browser, reconnect those streams when the client refreshes, and guarantee proper cleanup of goroutines, connections, and table state when tables are removed or clients disconnect.

This framing is important because it reveals that the core evaluation criteria are architectural rather than cosmetic.

---

## 4. Explicit Functional Requirements

The assignment explicitly requires the following functional behavior:

### 4.1 Dashboard behavior

- The page should load with a single **Add Table** button.
- Clicking **Add Table** should create a new table on the page.
- Additional tables may be created until the dashboard reaches a maximum of **8 tables**.
- Each table must be removable individually.

### 4.2 Streaming behavior

- Each table must stream its own independent sequence of poker events.
- Events from one table must not appear in another table.
- The stream should continue indefinitely using repeated simulated hands.

### 4.3 Event model

The system must simulate the following event types:

- `game_started`
- `players_joined`
- `card_dealt`
- `bet_action`
- `community_cards`
- `hand_result`

### 4.4 Reconnection behavior

- If the browser refreshes or the user navigates away and returns, active tables should reconnect automatically or at minimum show their last known state.

### 4.5 Documentation and testing

- The repository must contain clear documentation.
- The documentation should explain architecture and client-server communication choice.
- Unit tests are required.

---

## 5. What the Assignment Is Really Evaluating

The written evaluation criteria make it clear that the assignment is meant to expose engineering judgment rather than feature count.

The work product is likely being reviewed for the following:

- whether the transport choice matches a read-heavy, server-to-client streaming problem
- whether each table is modeled as an independent concurrent unit
- whether resource cleanup is deliberate and reliable
- whether the frontend uses HTMX and Alpine.js idiomatically rather than treating them as substitutes for a large SPA framework
- whether the solution remains simple, readable, and well structured
- whether the documentation clearly explains both the chosen path and the alternatives that were rejected

This means a technically strong solution should optimize for clarity and control of lifecycle, not for unnecessary feature breadth.

---

## 6. Hidden Constraints and Implied Design Requirements

Several important constraints are implied by the wording of the brief even if they are not all written as hard functional requirements.

### 6.1 The workload is read-heavy

The assignment explicitly notes that the system is read-heavy by nature. This is a major clue for transport design. A one-way streaming mechanism is likely more appropriate than a full duplex protocol unless bidirectional behavior is actually required.

### 6.2 Table runtimes must be isolated

Each table is conceptually independent. That implies isolation not just in UI rendering but in runtime ownership and cancellation. The architecture should make it easy to prove that one table’s lifecycle cannot accidentally corrupt or block another’s.

### 6.3 Cleanup is a first-class requirement

The assignment specifically calls out goroutine and connection cleanup. That means cleanup should not be treated as an incidental implementation detail. It should be visible in the design and covered by tests.

### 6.4 Session continuity matters

The brief asks for reconnect or last-known-state restoration. This introduces a lightweight state continuity problem, even though persistent storage is not required.

### 6.5 The solution only needs to run locally

Because production deployment is explicitly out of scope, local in-memory state becomes an acceptable default. Introducing a database or distributed infrastructure would likely increase complexity without increasing evaluation value.

---

## 7. Core Engineering Questions That Must Be Resolved Early

Before implementation begins, the following design questions need explicit answers:

1. What transport should be used for live event streaming?
2. What server-side abstraction should represent a table?
3. How should active tables be associated with a browser session?
4. How should reconnect behavior work after refresh?
5. How should the system prevent one slow subscriber from blocking an entire table?
6. What should be stored as table state versus event history?
7. How should table removal guarantee backend cleanup?
8. How much complexity is justified given that this is a local-only technical exercise?

These questions drive the architecture more than the visual UI does.

---

## 8. Candidate Streaming Strategies

Three realistic transport strategies exist for this problem.

### 8.1 Polling / long polling

**Pros**
- easy to understand conceptually
- plain HTTP request/response model

**Cons**
- inefficient for continuous updates
- increased request overhead
- introduces either lag or waste
- poor fit for many small sequential events

**Assessment**
- Not suitable for this assignment unless simplicity were the only concern.

### 8.2 WebSockets

**Pros**
- full duplex communication
- widely used for real-time apps
- can support future client → server messaging

**Cons**
- more connection and protocol complexity
- introduces bidirectional capability that is not needed by the prompt
- additional lifecycle and testing complexity
- can be harder to justify when the stream is fundamentally one-way

**Assessment**
- Technically viable, but likely overbuilt for the problem.

### 8.3 Server-Sent Events (SSE)

**Pros**
- designed for server → client streaming
- browser-native `EventSource`
- automatic reconnection support
- simpler than WebSockets for append-style feeds
- fits well with a read-heavy workload

**Cons**
- one-way only
- less flexible if future bidirectional interaction is added
- requires a separate request channel for client actions

**Assessment**
- Best match for the stated problem.

---

## 9. Initial Technical Direction

Based on the assignment and the trade-offs above, the strongest initial direction is:

- Go HTTP server
- Server-Sent Events for live streaming
- HTMX for add/remove table and server-rendered partials
- Alpine.js for table-local stream state and EventSource lifecycle
- in-memory server-owned table runtimes
- session cookie to associate active tables with a browser session

This direction is attractive because it keeps the system aligned with the brief while remaining straightforward to explain and test.

---

## 10. Proposed Runtime Model

The simplest maintainable server model is to represent each active table as an independent runtime object.

That table runtime should own:

- a unique table identifier
- the browser session identifier that created it
- a cancellation context
- a simulation loop
- the current table state
- a bounded history of recent events
- a set of active subscribers

This model makes the major lifecycle actions obvious:

- create table → create runtime
- remove table → cancel runtime
- browser reconnect → attach a new subscriber to the existing runtime
- session restore → re-render active table cards for the existing runtime set

This is likely the cleanest model for demonstrating concurrency and cleanup.

---

## 11. Reconnection Strategy Options

The assignment does not require durable persistence, only reconnect or last-known-state restoration. That opens several reasonable approaches.

### Option A: Pure client-side restoration

The browser stores table IDs and last rendered events in `sessionStorage` or `localStorage`.

**Pros**
- simple
- no server session management needed

**Cons**
- browser becomes source of truth
- difficult to reconnect to active server runtimes correctly
- stale or invalid table state becomes harder to manage

### Option B: Pure server-side session restoration

The server issues a session cookie and stores active table IDs in memory.

**Pros**
- server remains source of truth
- table ownership and reconnect are straightforward
- cleaner lifecycle story

**Cons**
- requires session manager logic
- state disappears on server restart

### Option C: Hybrid restoration

Use server-side session state as source of truth and client-side session storage as a cosmetic fallback for last-known-state while reconnect occurs.

**Pros**
- best UX of the three
- maintains clean server ownership
- improves refresh experience

**Cons**
- slightly more frontend logic

**Recommended initial direction**
- Option C if time permits, otherwise Option B.

---

## 12. Risks Identified Up Front

### 12.1 Goroutine leaks

If table cancellation is not properly wired through the simulation loop and subscriber registry, removed tables may continue consuming resources.

### 12.2 Slow subscriber backpressure

A slow or disconnected client must not stall an active table runtime.

### 12.3 Unclear ownership boundaries

If session ownership is not modeled explicitly, invalid table access and stale reconnection flows become harder to manage.

### 12.4 Overcomplicated frontend state

Trying to use HTMX alone for long-lived streaming state would likely produce awkward code. Client-local stream state should remain minimal and well bounded.

### 12.5 Time overrun

Because this is a technical assignment rather than a production feature, excessive investment in optional enhancements could reduce time available for documentation and tests, which are explicitly evaluated.

---

## 13. Scope Boundaries

### In scope

- add/remove table functionality
- up to 8 active tables
- independent stream per table
- proper runtime cleanup
- refresh/session continuity
- documentation
- unit tests
- clean package structure

### Out of scope

- real poker logic
- persistent database storage
- production deployment
- authentication
- responsive design complexity
- distributed scale-out
- multi-user collaboration across sessions

These boundaries should remain explicit throughout the implementation to protect the schedule.

---

## 14. Initial Success Criteria

The implementation should be considered successful if it demonstrates the following:

- active tables can be added and removed correctly
- each table streams independently
- table removal reliably stops backend work
- refresh restores active tables for the current session
- the chosen transport is easy to justify against alternatives
- code is organized by responsibility and is maintainable
- unit tests verify the most important lifecycle guarantees
- documentation explains both design choices and trade-offs clearly

---

## 15. Conclusion

The strongest solution direction is not the most visually advanced one. It is the one that presents the clearest control over streaming, concurrency, and lifecycle.

The most promising architecture direction at this stage is:

- server-owned table runtimes
- SSE for event delivery
- HTMX for structural interactions
- Alpine.js for table-local browser state
- in-memory session and runtime state
- explicit cancellation and cleanup paths

That direction is carried forward into the design document.
