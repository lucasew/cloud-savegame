# Consistently Ignored Changes

This file lists patterns of changes that have been consistently rejected by human reviewers. All agents MUST consult this file before proposing a new change. If a planned change matches any pattern described below, it MUST be abandoned.

---

## IGNORE: Unpinned GitHub Actions

**- Pattern:** Using version tags (e.g., `@v4`, `@v3`) instead of immutable commit SHAs for GitHub Actions in workflow files. Removing or downgrading `jdx/mise-action`.
**- Justification:** Security best practice requires pinning actions to specific immutable commits. The project standardized on using `jdx/mise-action@v3` (pinned by SHA) instead of manual curl installation of mise in workflows. Replacing the action with a manual script is a consistent rejection pattern.
**- Files Affected:** `.github/workflows/*.yml`

## IGNORE: Premature Abstraction and Function Extraction

**- Pattern:** Extracting helper functions (e.g., `parse_rules`, `search_for_homes`, config helpers) to module scope or wrapping logic in classes (e.g., `GitManager`) before duplication occurs at least three times.
**- Justification:** Violates the "Rule of Three" and "Locality of Behavior". Premature extraction increases complexity and file count without real benefit, which is a known rejection pattern.
**- Files Affected:** `internal/**/*.go`, `cloud_savegame/**/*.py`

## IGNORE: Moving Root Configuration Files

**- Pattern:** Moving `demo.cfg` or similar root configuration files to subdirectories.
**- Justification:** The application expects configuration files in the root or alongside the executable. Moving them breaks this expectation and is a rejection pattern.
**- Files Affected:** `demo.cfg`, `*.ini`

## IGNORE: Adding Non-Standard Tooling

**- Pattern:** Adding non-standard tooling (e.g., `uv`, `ruff`) or tasks to `mise.toml` that deviate from the project's established Go-centric toolchain.
**- Justification:** The project strictly adheres to a Go-centric toolchain. Introducing Python-specific tools without explicit project consensus introduces unnecessary complexity and is a rejection pattern.
**- Files Affected:** `mise.toml`

## IGNORE: Bundling Unrelated Changes

**- Pattern:** Modifying runtime code or workflows in the same PR that is specifically scoped for something else (e.g., a documentation agent modifying code, or a Denoiser PR updating workflow files).
**- Justification:** Violates execution guardrails and scope discipline. Agents must execute only the explicitly requested outcome and not bundle "nice to have" changes into the implementation.
**- Files Affected:** All files
