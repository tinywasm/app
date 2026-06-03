# TinyWasm

<img src="https://raw.githubusercontent.com/tinywasm/app/main/docs/img/badges.svg">

**LLM-Friendly Full-Stack Go Framework** — Build complete web applications using only Go + WebAssembly, with minimal JavaScript. TinyWasm is a TUI-based development environment that acts as an intelligent intermediary between you, your LLM assistant, and your application.

> Source code is maintained privately. This repository distributes official binaries.

---

## Installation

### Prerequisites

- **Go 1.25.2+** — [go.dev/dl](https://go.dev/dl/)
- **TinyGo** (for M/S WASM modes) — [tinygo.org/getting-started/install](https://tinygo.org/getting-started/install/)
- **Chrome/Chromium** — for browser automation

### Install via Go

```bash
go install github.com/tinywasm/app/cmd/tinywasm@latest
```

### Download binary

Pre-compiled binaries for Linux, macOS, and Windows are available on the [Releases](https://github.com/tinywasm/app/releases) page.

---

## Quick start

```bash
mkdir myapp && cd myapp
tinywasm
```

TinyWasm will:

1. Scaffold the project structure if it's a new directory
2. Start the development server on `https://localhost:6060`
3. Launch the TUI with live logs and component status
4. Open Chrome with auto-reload enabled
5. Start the MCP server on `http://localhost:3030/mcp` for LLM integration

---

## Key features

| Feature | Description |
|---------|-------------|
| **MCP server** | AI assistants can see your UI, read logs, and control compilation |
| **Three WASM modes** | L (Go std ~2MB), M (TinyGo debug ~500KB), S (TinyGo release ~200KB) |
| **Hot reload** | Backend, WASM, assets, and browser — all automatic |
| **TUI control center** | Real-time status and logs for all components |
| **Zero config** | Project structure is the configuration |

---

## MCP server

The MCP daemon starts automatically and integrates with your IDE:

```bash
tinywasm -mcp     # start global daemon (MCP + SSE on :3030)
tinywasm          # start TUI client (connects to daemon)
```

| IDE | Config file |
|-----|-------------|
| VS Code | `~/.config/Code/User/mcp.json` |
| Claude Code | `~/.claude.json` → `mcpServers.tinywasm.url` |

---

## WebAssembly compilation modes

| Mode | Compiler | Size | Use case |
|------|----------|------|----------|
| **L** | Go standard | ~2 MB | Development — full stdlib, fastest iteration |
| **M** | TinyGo debug | ~500 KB | Debugging — balanced size and functionality |
| **S** | TinyGo release | ~200 KB | Production — minimal size, optimized |

Switch modes on-the-fly via the TUI or via MCP tool call.

---

## Support

- **Issues**: [github.com/tinywasm/app/issues](https://github.com/tinywasm/app/issues)
- **Discussions**: [github.com/tinywasm/app/discussions](https://github.com/tinywasm/app/discussions)

---

## License

MIT — see [LICENSE](LICENSE)
