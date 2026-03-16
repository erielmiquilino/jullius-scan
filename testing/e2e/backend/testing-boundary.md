# Testing Boundary

- `testing/e2e/backend/` contains backend-only E2E automation.
- `testing/e2e/mobile/` is reserved for a future Flutter mobile E2E workspace.
- Backend E2E must own its own dependencies, fixtures, commands, and reports.
- Mobile E2E will be added later in parallel rather than merged into this workspace.
- Shared helpers between backend and mobile should only be extracted later if they are environment-neutral and intentionally versioned as common tooling.
