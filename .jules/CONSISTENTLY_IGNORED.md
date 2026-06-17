# Consistently Ignored Changes

This file lists patterns of changes that have been consistently rejected by human reviewers. All agents MUST consult this file before proposing a new change. If a planned change matches any pattern described below, it MUST be abandoned.

---

## IGNORE: Unpinned GitHub Actions

**- Pattern:** Using version tags (e.g., `@v4`, `@v3`) instead of immutable commit SHAs for GitHub Actions in workflow files.
**- Justification:** Security best practice requires pinning actions to specific immutable commits to prevent supply chain attacks. Usage of tags is a consistent rejection pattern.
**- Files Affected:** `.github/workflows/*.yml`

## IGNORE: Premature Function Extraction

**- Pattern:** Extracting helper functions (e.g., `parse_rules`, `search_for_homes`, config getters) or wrapping logic in classes (e.g., `GitManager`) when used in a single context or module before duplication occurs at least three times.
**- Justification:** Violates the "Rule of Three" and "Locality of Behavior". Premature extraction increases complexity and file count without benefit, and is a known rejection pattern.
**- Files Affected:** `internal/**/*.go`, `cloud_savegame/**/*.py`

## IGNORE: Moving Configuration Files

**- Pattern:** Moving `demo.cfg` or similar root config files to subdirectories (e.g., `cloud_savegame/demo.cfg`).
**- Justification:** The application expects configuration files in the root or alongside the executable. Moving it to subdirectories is a rejection pattern.
**- Files Affected:** `demo.cfg`, `*.ini`

## IGNORE: Adding Non-Standard Tools

**- Pattern:** Adding non-standard tooling (e.g., `uv`, `ruff`) or tasks to `mise.toml` that deviate from the project's primary language/toolchain (Go).
**- Justification:** The project strictly adheres to an established Go-centric toolchain defined in `mise.toml`. Introducing Python-specific tools without explicit project consensus is a rejection pattern.
**- Files Affected:** `mise.toml`

## IGNORE: Bundling Unrelated Changes

**- Pattern:** Modifying runtime code or workflows (e.g., fixing `mise-action` installation) in the same PR that is specifically scoped for something else (e.g., a Denoiser PR updating patterns).
**- Justification:** Violates execution guardrails and scope discipline. Agents must execute only the explicitly requested outcome and not bundle "nice to have" changes into the implementation.
**- Files Affected:** All
