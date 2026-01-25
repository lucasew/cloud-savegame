## 2024-07-25 - Extract `copy_item` function to module scope

**Issue:** The `copy_item` function was nested inside the `main` function in `cloud_savegame/__init__.py`. This nesting reduced readability and made the `main` function unnecessarily complex, as it was responsible for both high-level application logic and low-level file operations.
**Root Cause:** The function was likely written as a small, localized helper and was never refactored out as the script grew. This is a common pattern in single-file scripts that evolve over time.
**Solution:** I extracted the `copy_item` function and moved it to the module scope. To decouple it from the `main` function's context, I updated its signature to explicitly accept its dependencies (`output_dir` and `verbose`) as arguments.
**Pattern:** The `main` function in `cloud_savegame/__init__.py` contains several nested helper functions. Extracting these helpers to the module scope is a good simplification pattern. Future refactorings should continue to move functions like `is_path_ignored` and `ingest_path` out of `main` to improve modularity and testability.

## 2024-07-26 - Extract `is_path_ignored` function to module scope

**Issue:** The `is_path_ignored` function was nested inside the `main` function in `cloud_savegame/__init__.py`. This made the `main` function longer than necessary and mixed high-level orchestration with low-level path-checking logic.
**Root Cause:** Similar to other helpers in this file, it was likely created as a small, local utility and never refactored out as the `main` function grew in complexity.
**Solution:** I moved the `is_path_ignored` function to the module scope. I updated its signature to accept `ignored_paths` as a parameter, decoupling it from the `main` function's local state. All internal call sites were updated to use the new module-level function.
**Pattern:** Continuing the established pattern of simplifying the `main` function, this change reinforces that nested helper functions should be extracted to the module scope to improve modularity, readability, and testability.

## 2025-05-24 - Configure linters and formatters

**Issue:** The project lacked a unified task runner and consistent configuration for linters and formatters, making it difficult to maintain code quality across different file types (Python, Markdown, TOML, etc.).
**Root Cause:** Tooling was configured ad-hoc or missing.
**Solution:** I installed `go-task` and `dprint`, configured `Taskfile.yml` with `lint` and `fmt` tasks (including wildcard subtask execution), and set up `dprint.json`. I also updated `mise.toml` to include these tools.
**Pattern:** Centralizing development tasks in a `Taskfile.yml` and using `mise` for tool management ensures a consistent developer experience and simpler CI integration.

## 2026-01-25 - Extract `parse_rules` function to module scope

**Issue:** The `parse_rules` function was nested inside the `main` function in `cloud_savegame/__init__.py`. This added unnecessary complexity to the `main` function and made `parse_rules` harder to test in isolation.
**Root Cause:** It was implemented as a closure to access `rulefiles` and `config` variables from the local scope.
**Solution:** I extracted `parse_rules` to the module scope. I updated its signature to explicitly accept `app`, `rulefiles`, and `config` as arguments. I also added a unit test to verify its behavior.
**Pattern:** Extracting nested helper functions from the `main` god-function improves modularity and allows for unit testing. Explicitly passing dependencies makes the data flow clearer.
