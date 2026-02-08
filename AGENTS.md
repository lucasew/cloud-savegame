# AGENTS.md

This file defines the project conventions and guidelines for all agents working on this codebase.

## Mise First

- **Mise First:** Before doing anything, you MUST install `mise` and use it for all task execution.
- **Real Execution:** Never simulate commands. Run them.
- **Mise Necessity:** `mise` is a non-negotiable basic necessity.

## Error Handling

- **Centralized Error Reporting:** The project MUST have a single, centralized error-reporting function. All code paths that handle unexpected errors MUST funnel through this function.
- **Sentry-aware:** If the project uses Sentry, report to it. If not, log with enough context.
- **No silent failures:** Every `catch` block or error callback that is not an expected/recoverable condition MUST call the centralized error-reporting function.

## Negative Memory

- **Pivot:** Read `.jules/CONSISTENTLY_IGNORED.md` before planning your work. If your intended change matches a pattern listed there, do NOT proceed with it.

## Retroactive Violations

- **Fix First:** If you find existing code that violates a known rule, fixing that violation is your first priority before implementing new features.

## Testing

- **Test Beyond Automation:** Passing lint and unit tests is the baseline, not the finish line. Manually verify changes where possible.
- **Tests Must Earn Their Place:** Tests must verify implemented logic or edge cases, not just exercise external libraries.

## Dependencies

- **Don't downgrade dependencies**: Don't downgrade dependencies unless explicitly asked.

## Documentation

- **Essentialism:** Only write necessary comments.
- **Value-Driven:** Focus on non-obvious details.
- **Source of Truth:** Ensure docs match current code.
- **PR Title:** Docs PRs must be titled `📝 Docs: [Description]`.
