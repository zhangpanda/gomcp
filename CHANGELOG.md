# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [v1.3.0] - 2026-04-30

### Fixed
- **[Critical]** gRPC adapter used input descriptor for response deserialization — all gRPC calls returned wrong/empty results
- **[High]** `handlePromptsGet` called user handler under RLock — deadlock if handler registers tools
- **[High]** OpenAPI adapter query parameters not URL-encoded — `&`/`=` injection risk
- **[High]** OpenAPI adapter body fields always sent as strings — APIs expecting int/bool would fail
- **[High]** Self-referential structs caused infinite recursion stack overflow in schema generator
- **[High]** `[]Struct` slices generated `"string"` item schema instead of nested object schema
- **[High]** `SetMaxConcurrentTasks` replaced entire task manager — orphaned in-flight tasks
- **[High]** `AsyncToolFunc` wrapped all tools with matching base name — corrupted existing sync tools
- `notifyFn` read/write data race between `notify()` and `Handler()`/`HTTP()`
- Completed async tasks never evicted — memory leak proportional to total async calls
- HTTP transport silently truncated oversized bodies — confusing JSON parse errors instead of 413
- `transport/stdio` `append(resp, '\n')` could mutate handler's returned slice
- `mcptest.Client` methods panicked on nil `call()` return or missing map keys
- `ToolFunc` only checked parameter count, not types — invalid signatures caused runtime panics
- Handler returning `(nil, nil)` produced `"result": null` violating MCP protocol
- `watchDir` goroutine leaked on server shutdown (no context cancellation)
- Deleted YAML tool files left zombie tools registered forever
- `provider.go` HTTP client had no timeout — could hang indefinitely
- Multiple `Handler()` calls overwrote `notifyFn` — earlier SSE clients lost notifications

## [v1.2.0] - 2026-04-29

### Added
- SECURITY.md with vulnerability reporting policy
- Dependabot for automated dependency updates (Go modules + GitHub Actions)
- CodeQL static analysis workflow (weekly + on push)
- CHANGELOG.md following Keep a Changelog format

### Fixed
- Removed useless `modTimes` initialization in `watchDir` (CodeQL go/useless-assignment-to-local)

### Changed
- Upgraded `actions/checkout` from v5 to v6 (via Dependabot)
- Upgraded `github/codeql-action` from v3 to v4 (via Dependabot)
- Enhanced `glama.json` with license, repository, author, tools, resources, and prompts metadata

## [v1.1.0] - 2026-04-27

### Added
- Streamable HTTP transport (`Server.HTTP` / `Handler`)
- Filesystem MCP server example
- Real environment tests — coverage 74.4% → 87.0%
- Regression tests for all bugs found during development
- Stdio and HTTP integration tests
- GitLab CI configuration

### Fixed
- Async task handler panic crashes entire process
- Version resolution uses semantic comparison instead of lexicographic
- `ContentBlock.Text` omitempty dropped empty strings

### Changed
- Upgraded MCP protocol to 2025-11-25
- Upgraded GitHub Actions to v5/v6 (Node.js 20 deprecation)
- Unified tool naming to snake_case for better coherence score
- Improved adapter test coverage from 45.6% to 55.0%

## [v1.0.0] - 2026-04-17

### Added
- Initial stable release
- Struct-tag based JSON Schema generation
- Middleware system (Recovery, RequestID, Logger, Timeout, RateLimit)
- Gin adapter for importing existing routes as MCP tools
- OpenAPI adapter for importing specs as MCP tools
- gRPC adapter for importing services as MCP tools
- Stdio transport for Claude Desktop, Cursor, and other MCP clients
- Async tool support with task management
- Resource and ResourceTemplate support
- Prompt support with argument completion
- OpenTelemetry middleware

[v1.3.0]: https://github.com/zhangpanda/gomcp/compare/v1.2.0...v1.3.0
[v1.2.0]: https://github.com/zhangpanda/gomcp/compare/v1.1.0...v1.2.0
[v1.1.0]: https://github.com/zhangpanda/gomcp/compare/v1.0.0...v1.1.0
[v1.0.0]: https://github.com/zhangpanda/gomcp/releases/tag/v1.0.0
