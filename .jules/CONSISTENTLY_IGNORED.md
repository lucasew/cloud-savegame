# Consistently Ignored Changes

This file lists patterns of changes that have been consistently rejected by human reviewers. All agents MUST consult this file before proposing a new change. If a planned change matches any pattern described below, it MUST be abandoned.

---

## IGNORE: Unpinned GitHub Actions

**- Pattern:** Using version tags (e.g., `@v4`, `@v3`) instead of commit SHAs for GitHub Actions in workflow files.
**- Justification:** Security best practice requires pinning actions to specific immutable commits to prevent supply chain attacks.
**- Files Affected:** `.github/workflows/*.yml`

## IGNORE: Premature Function Extraction

**- Pattern:** Extracting helper functions (e.g., `parse_rules`, `search_for_homes`, config getters) or wrapping logic in classes (e.g., `GitManager`) when used in a single context or module.
**- Justification:** Violates "Rule of Three" and "Locality of Behavior". Premature extraction increases complexity and file count without benefit.
**- Files Affected:** `cloud_savegame/__init__.py`, `cloud_savegame/config_utils.py`, `internal/**/*.go`

## IGNORE: Moving Configuration Files

**- Pattern:** Moving `demo.cfg` or similar root config files to subdirectories (e.g., `cloud_savegame/demo.cfg`).
**- Justification:** The application expects configuration files in the root or alongside the executable.
**- Files Affected:** `demo.cfg`, `*.ini`

## IGNORE: Adding Non-Standard Tools

**- Pattern:** Adding tools (like `uv`, `ruff`) or tasks to `mise.toml` that deviate from the project's primary language/toolchain (Go) without strong justification.
**- Justification:** Keeps the toolchain focused and avoids unnecessary dependencies and configuration complexity.
**- Files Affected:** `mise.toml`
