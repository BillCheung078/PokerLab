# Timesheet

This document is used to track the time spent on project-related tasks and provide a clear record of work activities.  
It serves as a reference for project management, progress monitoring, and effort estimation.

---

## Entries

### 2026-03-27
- **Time Spent:** 1.3 hours  
- **Activity:** Reviewed and organized assessment materials; drafted initial notes

### 2026-03-27
- **Time Spent:** 2.0 hours
- **Activity:** Expanded the design documentation, including the detailed design direction, communication model, trade-off analysis, and implementation roadmap

### 2026-03-28
- **Time Spent:** 2.6 hours
- **Activity:** Phase 1 - Initialized the Go project and created the baseline application skeleton for the poker dashboard assignment. Set up the Go module, server entrypoint, HTTP routing layer, and shared template renderer. Added the initial page layout and dashboard shell, including base templates, static CSS/JS assets, and CDN-based inclusion of HTMX and Alpine.js. Created the core project structure for future phases, including placeholder internal packages for session, table, and simulation logic. Implemented static file serving and a basic GET / dashboard handler. Added initial HTTP tests to verify homepage rendering and static asset delivery so the project could run locally with a clean, testable foundation.


### 2026-03-28
- **Time Spent:** 1.3 hours
- **Activity:** Phase 2 - Implemented the dashboard’s table lifecycle and session management features. Added server-side session handling so each browser session could own its own set of poker tables and restore them after a page refresh. Built backend support for creating and removing tables, enforcing the assignment limit of up to 8 active tables per session, and validating ownership before removal. Updated the dashboard UI to support dynamic add/remove actions through HTMX-based partial updates, including table count updates, empty-state handling, and user-facing status messages. Added tests to verify table creation, removal, session restoration, and max-table limit behavior.

### 2026-03-28
- **Time Spent:** 1.4 hours
- **Activity:** Phase 3 - Implemented the runtime foundation for independent table execution. Added core simulation domain models, including poker event, table state, and per-table runtime structures with lifecycle management through context cancellation. Extended the table manager so each created table now owns a dedicated runtime instance and runtime cleanup is triggered automatically when a table is removed. Added bounded runtime history, snapshot support, and subscriber registration primitives to prepare for the later streaming layer. Updated the dashboard status rendering to surface runtime state without introducing live simulation yet. Added unit tests covering runtime initialization, event/history updates, subscriber cleanup, and runtime cancellation on table deletion.

### 2026-03-28
- **Time Spent:** 0.4 hours
- **Activity:** Phase 3 -  Implemented the simulation loop for each table runtime so active tables now execute independently in their own goroutines. Added the repeated hand progression logic to emit a sequence of simulated poker events, including game start, player join, card dealing, betting actions, community cards, and hand results. Wired the simulation engine so runtime execution starts automatically when a table is created and stops cleanly when a table is removed. Updated the dashboard view to surface runtime progress such as current hand number, blind level, event activity, and simulation status. Added tests to verify independent runtime progression and clean shutdown after cancellation.

### 2026-03-28
- **Time Spent:** 0.3 hours
- **Activity:** Phase 4 - Implemented the backend streaming layer using Server-Sent Events (SSE) for independent table event delivery. Added a per-table stream endpoint with session ownership validation, SSE response handling, event flushing, and recent history replay so reconnecting clients can receive the latest buffered events. Integrated table runtime subscriber registration and cleanup so stream connections are removed automatically on disconnect or runtime shutdown. Ensured event fan-out remains non-blocking so slow or disconnected clients do not stall table execution. Added tests covering stream replay, live event delivery, unauthorized stream access, and subscriber cleanup on disconnect.

### 2026-03-28
- **Time Spent:** 1 hours
- **Activity:** Phase 5 - Integrated the frontend dashboard with the backend SSE streaming layer using Alpine.js and refined the stream topology to use a single shared session-level SSE connection. Implemented a shared browser stream manager that multiplexes incoming events by table_id and routes them to the appropriate table card, allowing each card to maintain its own live feed, runtime metadata, and connection state without opening a separate EventSource per table. This change resolved browser HTTP/1.1 per-origin connection limit issues that appeared with one stream per table and made the dashboard reliable up to the assignment’s 8-table session limit. Updated the UI to keep HTMX-driven add/remove actions while preserving live event updates, and adjusted the design documentation to explain the rationale for application-level multiplexing over reliance on HTTP/2.

### 2026-03-29
- **Time Spent:** 0.8 hours
- **Activity:** Bonus UI Enhancement - Implemented an optional visual table presentation to complement the text-based event feed. Added a richer poker-table interface with seated players, hole cards, community board rendering, and lightweight animated feedback for betting activity, inspired by modern online poker clients while remaining within the assignment’s lightweight frontend stack. Refined the live event panel to use a fixed-height, scrollable layout to reduce page movement during updates, and adjusted the simulation pacing to make event progression easier to follow during demos. This bonus work was intentionally scoped as a visual enhancement on top of the core lifecycle and streaming architecture rather than a change to the underlying backend model.

### 2026-03-29
- **Time Spent:** 0.2 hours
- **Activity:** Phase 6 - Finalized refresh and reconnection continuity behavior for the dashboard. Verified that active tables are restored from the server-side session after page reload, that the shared session SSE stream reconnects correctly, and that recent event history replay bridges the gap between disconnect and live updates. Added a continuity-focused handler test to validate the end-to-end reload and reconnect flow, and updated the roadmap and design documentation to clarify that Phase 6 is achieved through the combined behavior of session restoration, stream replay, and frontend reconnection rather than a separate subsystem.

