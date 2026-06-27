# Consistently Ignored Changes

This file lists patterns of changes that have been consistently rejected by human reviewers. All agents MUST consult this file before proposing a new change. If a planned change matches any pattern described below, it MUST be abandoned.

---

## IGNORE: Unpinned GitHub Actions & Manual Tool Setup

**- Pattern:** Using version tags (e.g., `@v4`, `@v3`) instead of immutable commit SHAs for GitHub Actions in workflow files. Downgrading actions (e.g., `jdx/mise-action`). Manual curl installation of tools (e.g., `mise`) via shell script in workflows.
**- Justification:** Security best practices require pinning actions to specific immutable commits to prevent supply chain attacks. The project relies on `jdx/mise-action` pinned by SHA, and replacing it with manual scripts or downgraded actions is a consistent rejection pattern.
**- Files Affected:** `.github/workflows/*.yml`

## IGNORE: Premature Abstraction and Function Extraction

**- Pattern:** Extracting helper functions (e.g., `parse_rules`, config getters) to module scope, or wrapping logic in classes (e.g., `GitManager`) before duplication occurs at least three times.
**- Justification:** Violates the "Rule of Three" and "Locality of Behavior". Premature extraction increases complexity and file count without real benefit, which is a known rejection pattern.
**- Files Affected:** `internal/**/*.go`, Python scripts, and test files

## IGNORE: Adding Non-Standard Tooling

**- Pattern:** Adding non-standard tooling (e.g., `uv`, `ruff`) or non-standard tasks to `mise.toml` that deviate from the project's established Go-centric toolchain.
**- Justification:** The project strictly adheres to a Go-centric toolchain. Introducing Python-specific tools or deviating from explicit project consensus introduces unnecessary complexity and is a rejection pattern.
**- Files Affected:** `mise.toml`

## IGNORE: Bundling Unrelated Changes

**- Pattern:** Modifying runtime code or workflows in the same PR that is specifically scoped for something else (e.g., a documentation agent modifying code or conventions, or a Denoiser PR updating workflow files).
**- Justification:** Violates execution guardrails and scope discipline. Agents must execute only the explicitly requested outcome and not bundle "nice to have" changes into the implementation.
**- Files Affected:** All files

## IGNORE: False Positive Vulnerability Fixes (Path Traversal)

**- Pattern:** Adding restrictive path checks (e.g., enforcing the Current Working Directory as a base path) to file traversal or ingestion functions under the assumption of "fixing" path traversal vulnerabilities.
**- Justification:** The application is a backup tool designed to read files from various locations across the filesystem. Restricting its scope to the current working directory breaks its core functionality and constitutes a false positive vulnerability "fix".
**- Files Affected:** `internal/backup/*.go`
