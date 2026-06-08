# ADR 001: Single-Binary Deployment via Go Embed

**Date:** 2026-06-08
**Status:** Accepted

### Context
To simplify distribution and reduce installation friction, the DNS server should be deployable as a single executable without requiring external dependencies for the frontend.

### Decision
Utilize Go's `//go:embed` functionality to include the production build of the React frontend directly into the Go backend binary.

### Consequences
*   **Pros:** Simplifies deployment; frontend and backend are always in sync; no need to manage static file paths in production.
*   **Cons:** Increases binary size slightly; requires a full rebuild to update frontend assets.
