# Consistently Ignored Patterns

This file serves as the central registry for patterns that have been consistently rejected by human reviewers. All agents must consult this file before creating a plan or writing code.

- Empty catch blocks and swallowed errors are strictly forbidden.
- False positive vulnerability fixes (e.g., security implementations that break intended core functionality) are a consistent rejection pattern.
- Do not abstract code prematurely. Apply the "Rule of Three" and "Locality of Behavior".
- Bundling unrelated code changes is a consistent rejection pattern.
- Do not introduce non-standard tooling without explicit project consensus. Adhere to the established toolchain defined in `mise.toml`.
- Do not use version tags (e.g., `@v4`) in GitHub Actions workflows. Use immutable commit SHAs.
- In Go projects, avoid naming custom error reporting packages `errors` to prevent shadowing the standard library `errors` package. Use alternatives like `reporter`.
- The project uses `internal/reporter` for centralized error reporting. Direct calls to underlying loggers (e.g., `slog.Error`) are forbidden and must funnel through `reporter.Report`.
