# Roadmap

## Overview

This roadmap breaks the implementation into small, logical phases so the project can be delivered incrementally while keeping the architecture clear, testable, and easy to review.

The implementation priority is:

1. establish a clean project foundation
2. get table lifecycle management correct
3. implement independent event streaming
4. support reconnection and session continuity
5. add testing and documentation
6. polish only if time remains

---

## Phase 0 — Planning and Framing

### Goal
Clarify the assignment requirements and lock the technical direction before implementation begins.

### Tasks
- restate the functional and non-functional requirements
- choose the communication model
- define session continuity strategy
- define package structure
- identify non-goals and scope boundaries
- document the initial architecture direction

### Deliverables
- initial notes
- design direction
- implementation task breakdown

---

## Phase 1 — Project Skeleton

### Goal
Create a clean, runnable baseline with clear separation of concerns.

### Tasks
- initialize the Go module
- create the server entrypoint
- set up routing
- add base HTML templates
- include HTMX and Alpine.js
- create folders for internal packages, templates, and static assets
- add minimal styling for readability

### Deliverables
- application starts locally
- dashboard page renders
- project structure is in place

---

## Phase 2 — Session and Table Lifecycle

### Goal
Implement add/remove behavior and session ownership rules before introducing streaming.

### Tasks
- implement session cookie handling
- implement session management
- implement table management
- enforce the maximum of 8 tables per session
- add table creation endpoint
- add table removal endpoint
- render table partials from the server
- restore active tables on initial page load

### Deliverables
- tables can be added and removed
- session state tracks active tables
- maximum table limit is enforced

---

## Phase 3 — Table Simulation Engine

### Goal
Build the independent event generator for each table.

### Tasks
- define the poker event model
- define table state and recent event history
- implement the repeating simulation loop
- emit the required event sequence
- update current table state as events are produced
- make each table runtime cancellable

### Deliverables
- each table produces its own event stream
- simulation runs independently per table
- simulation can be stopped cleanly

---

## Phase 4 — Streaming Layer

### Goal
Deliver live table events from the backend to the browser.

### Tasks
- implement the SSE stream endpoint
- register and unregister subscribers
- stream event payloads to the frontend
- flush events correctly
- handle client disconnects
- ensure slow or disconnected clients do not block table execution

### Deliverables
- frontend receives live events in near real time
- each table has its own stream
- stream cleanup works correctly on disconnect

---

## Phase 5 — Frontend Streaming Integration

### Goal
Connect the browser UI to the streaming backend using HTMX and Alpine.js appropriately.

### Tasks
- build an Alpine component for each table
- initialize `EventSource` per table
- append incoming events to the table feed
- display connection state
- wire remove actions to the backend
- handle basic reconnect states in the UI

### Deliverables
- event feeds update live in the browser
- each table behaves as an isolated component
- add/remove flow works end to end

---

## Phase 6 — Reconnection and Continuity

### Goal
Support refresh and reconnection behavior in a clear and predictable way.

### Tasks
- restore active tables from the server-side session
- reconnect table streams after page reload
- preserve current state or recent event history
- optionally cache lightweight client-side state for smoother recovery
- handle expired or removed tables gracefully

### Deliverables
- refreshing the page restores active tables
- table streams reconnect successfully
- users can recover last known state when needed

---

## Phase 7 — Testing and Hardening

### Goal
Validate the important lifecycle, concurrency, and cleanup behaviors.

### Tasks
- add unit tests for session management
- add unit tests for table lifecycle management
- add tests for simulator behavior
- add tests for stream subscription and cleanup
- add handler tests using `httptest`
- verify edge cases such as max table limit and removed tables

### Deliverables
- core backend behavior is covered by tests
- cleanup and reconnection behavior are validated
- key lifecycle paths are stable

---

## Phase 8 — Documentation and Submission

### Goal
Make the project easy to run, review, and evaluate.

### Tasks
- finalize the README
- document architecture and communication choices
- document trade-offs and alternatives considered
- document setup and test instructions
- review code organization and formatting
- prepare the final submission package

### Deliverables
- polished repository structure
- clear setup and usage instructions
- complete supporting documentation

---

## Optional Phase — Visual Enhancement

### Goal
Add a richer table presentation only after the core architecture is complete.

### Possible additions
- player seating summary
- compact board display
- richer table status panel
- more visual hand progression

### Constraint
This phase should only be attempted after streaming, cleanup, reconnection, testing, and documentation are already complete.

---

## Final Implementation Priorities

If scope needs to be reduced, the order of priority should remain:

1. correct table lifecycle management
2. independent event streaming
3. cleanup of goroutines and connections
4. session continuity and reconnection
5. testing
6. documentation polish
7. optional visual enhancements