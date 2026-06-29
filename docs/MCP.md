# MCP reference

The MCP daemon starts automatically and integrates with your IDE so AI assistants can see the UI,
read logs, and control compilation.

## Start

```bash
tinywasm -mcp     # global daemon (MCP + SSE on :3030)
tinywasm          # TUI client (connects to the daemon)
```

Configurable port: `TINYWASM_MCP_PORT=3030`.

## HTTP endpoints

| Method | Path | Description |
|--------|------|-------------|
| POST | `/mcp` | JSON-RPC 2.0 — standard MCP tools |
| GET | `/logs` | SSE — log stream of the active project |
| GET | `/tinywasm/state` | JSON state of the active project |
| POST | `/tinywasm/action` | Action dispatch: `{key, value}` |
| GET | `/version` | Daemon version |

## Available tools

| Tool | When | Description |
|------|------|-------------|
| `start_development` | Always | Start/switch the active project (headless) |
| `app_rebuild` | With an active project | Recompile WASM and reload the environment |
| WasmClient/Browser tools | With an active project | Depend on the project's modules |

## IDE configuration (auto-managed when the daemon starts)

| IDE | Config file |
|-----|-------------|
| VS Code | `~/.config/Code/User/mcp.json` |
| Claude Code | `~/.claude.json` → `mcpServers.tinywasm.url` |
| Antigravity | `~/.gemini/antigravity/mcp_config.json` |

## Diagnostics

```bash
# Verify the daemon is up
curl http://localhost:3030/mcp -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"initialize","id":"1","params":{"protocolVersion":"2024-11-05","clientInfo":{"name":"test","version":"0"}}}'

# Check the Claude Code config
cat ~/.claude.json | grep -A5 mcpServers
```

## Why it matters — the LLM workflow

Without TinyWasm, an assistant burns tokens explaining setup (webpack, npm, TypeScript, babel…).
With the MCP integration it reads the project, sees the live UI via screenshot, writes Go in
`web/client.go`, updates shared validation in `web/shared.go`, and TinyWasm auto-compiles and
reloads — the LLM then verifies the result via a new screenshot. The TUI is the intermediary that
handles all infrastructure so the assistant focuses on application logic.
