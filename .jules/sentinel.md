# Sentinel's Journal - CRITICAL LEARNINGS ONLY

## 2026-02-05 - Fix Path Traversal in Rule Ingestion

**Vulnerability:** The `IngestPath` function allowed relative paths (e.g., `../secret`) to be processed when no base path was explicitly provided (e.g., in rules without variables). This could allow reading files outside the intended directory scope if the tool is run in a sensitive directory. The check `filepath.IsAbs` was insufficient as it did not block relative path traversal.

**Learning:** When validating file paths, ensuring a path is not absolute is not enough. You must also ensure it does not traverse upwards using `..`. Enforcing a safe base directory (like CWD) and checking that resolved paths stay within it is crucial.

**Prevention:** Always resolve paths to their absolute form and verify they are strictly contained within a trusted base directory using `strings.HasPrefix` (or safer equivalent). Default to a restrictive base (like CWD) if none is provided.
