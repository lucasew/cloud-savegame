# Janitor Journal

**Mission**: Identify and fix ONE small quality issue (< 50 lines) per PR that reduces chaos or improves maintainability.

## Issues Addressed

- **Centralized Error Reporting (Debt)**: Replaced scattered calls to `slog.Error` (and other logging functions when used to report errors directly) with the centralized `reporter.Report` function located in `internal/reporter/reporter.go`. This adheres to the requirement that all code paths must funnel errors through a single centralized reporting function. The issue was detected by tracking log occurrences as a quality defect, which makes maintenance harder if we later want to integrate with systems like Sentry. We established a pattern, and the `reporter.Report` function maps to Sentry, etc., as required.

## Consistent Rules To Follow

- Do not leave empty catch blocks or swallow errors.
- Do not log errors directly via `console.error` / `Sentry.captureException` / `slog.Error`. Use the centralized error reporting module (e.g., `reporter.Report`).
- Respect established project conventions found in `AGENTS.md` and `.jules/CONSISTENTLY_IGNORED.md`.

## Lessons Learned

- Integrating a centralized reporting module improves code consistency and simplifies potential future integration with specialized external error tracking services like Sentry.
