# Sentinel's Journal - CRITICAL LEARNINGS ONLY

## 2026-01-18 - Path Traversal Bypass via Prefix Matching
**Vulnerability:** A `startswith` check was used to validate if a file path resided within a base directory. This is insecure because `/tmp/safe_evil` starts with `/tmp/safe`, allowing access to sibling directories with similar names.
**Learning:** String prefix matching is insufficient for path validation. Path boundaries must be respected.
**Prevention:** When validating paths, ensure the target path either equals the base path or starts with the base path followed by the OS path separator (`os.sep`). Always resolve paths to their canonical absolute forms first.
