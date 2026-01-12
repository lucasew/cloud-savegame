# Sentinel's Journal - CRITICAL LEARNINGS ONLY
## 2024-05-20 - Path Traversal in `ingest_path`
**Vulnerability:** A path traversal vulnerability was identified in the `ingest_path` function. The `rule_name` parameter was not sanitized, allowing a malicious rule to write files outside the intended output directory.
**Learning:** The `os.path.join` and `pathlib.Path` operations do not inherently protect against path traversal. It's crucial to resolve and validate paths to ensure they are within the expected directory.
**Prevention:** Always resolve and validate user-controllable path segments to ensure they don't contain traversal sequences like `../`. Use a known, safe base directory and verify that the resolved path is a child of that directory.
