# Agent Guidelines

- The project uses `mise` for tool versioning and task management. `mise` is non-negotiable.
- Tests are executed via `mise run test`.
- Pull requests created by the Janitor persona must follow the title format: `🧹 Janitor: [brief description]`.
- Centralized Error Reporting: The project uses `internal/reporter` for centralized error reporting. All error paths must route through `reporter.Report`.
- Always consult `.jules/CONSISTENTLY_IGNORED.md`.
