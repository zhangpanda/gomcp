# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- `examples/grpc-adapter` — self-contained gRPC adapter example using grpc/health + reflection. No protoc required.
- Benchmark tests for hot paths: `HandleRaw` dispatch, schema validation, middleware chain, tools/list.
- Cookbook docs: `docs/cookbook/` with 3 guides (search tool, import existing API, auth + RBAC).
- `release-please` workflow for automated changelog and versioning.
- SSE endurance test (10s sustained connection, heartbeat + notification delivery verification).
- CI matrix now includes `windows-latest` for cross-platform regression coverage.
- `regression_tool_name_test.go` — locks in OTel span name and Prometheus tool label.
- `regression_close_eviction_test.go` — stress test for Close() vs concurrent session eviction.

### Fixed

- **[BREAKING-FIX] `_tool_name` invisible to outer middleware chain.** `OpenTelemetry()` emitted spans named `mcp.tool.unknown` and `PrometheusMetrics()` bucketed all calls under an empty tool label. Root cause: `_tool_name` was only set inside `handleToolsCall` (on a forked child context) after the middleware chain had already fired. Now peeked from `tools/call` params before `executeChain` runs. **If you relied on the old (broken) behaviour of `_tool_name` being absent in middleware, this is a breaking change — but the old behaviour was a bug, not a feature.**
- **Session leak in stdio mode.** `GetOrCreate("")` created a new session (with a random UUID) on every tool call when no `Mcp-Session-Id` header was present. After 30s of sustained load, heap grew 115x. Fix: empty-id calls now reuse a single `_default` session.
- **Redundant JSON unmarshal in handleToolsCall.** `msg.Params` was parsed twice per tool call (once in `mergedArgsForMiddleware`, once in `handleToolsCall`). Eliminated the second parse by reading `_tool_name` from context. Saves ~8 allocs and ~300B per call.
- Subprocess cleanup in `integration_stdio_test.go` uses process groups (Setpgid + killGroup) to prevent grandchild leaks from `go run`.

### Changed

- Docs clarify auth middleware error shape: failures return `isError=true` in the tools/call result, not a JSON-RPC error object or HTTP 401/403.
- Docs clarify Provider `version` field renames tool to `name@version`.

---

### Previously in [Unreleased]

- Global `Use` middleware runs for JSON-RPC methods uniformly (except notification `notifications/initialized`); Streamable HTTP request context propagates into resource and prompt handlers.
- `WithSSEAuth` plus `SSEBearerAuth`, `SSEAPIKeyAuth`, and `SSEBasicAuth` for SSE (GET) gates.
- `SkipAuthForMCPMethods`, `HandshakeAuthSkipMethods`, and `*SkipHandshake` auth wrappers for MCP handshake-friendly HTTP clients.
- `transport.WrapCORS` for explicit browser `Origin` allowlists (no wildcard with credentials).
- `mergedArgsForMiddleware` merges `api_key` from `prompts/get` arguments and `resources/read` params JSON when headers are absent.

### Changed

- Docs clarify Bearer validation is caller-defined (JWT decoding not built-in).

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
