# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

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

[v1.2.0]: https://github.com/zhangpanda/gomcp/compare/v1.1.0...v1.2.0
[v1.1.0]: https://github.com/zhangpanda/gomcp/compare/v1.0.0...v1.1.0
[v1.0.0]: https://github.com/zhangpanda/gomcp/releases/tag/v1.0.0
