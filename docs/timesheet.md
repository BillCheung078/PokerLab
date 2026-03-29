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