## Archived: 2026-11-02

* The `gt-jules` command source code is located in `cmd/gt-jules` and utilizes a client library in `internal/jules`.
* The project includes built-in agent presets for `claude`, `gemini`, `codex`, `cursor`, `auggie`, `amp`, `opencode`, and `jules`.
* Project tests are run using standard Go testing commands (e.g., `go test ./internal/config/...`).
* Gas Town is a multi-agent orchestration system written in Go designed to coordinate coding agents.
* The `jules` agent preset uses the `gt-jules` command (a custom wrapper for the Jules API) and requires the `JULES_API_KEY` environment variable.
* Agent presets and configurations are defined in `internal/config/agents.go`.
