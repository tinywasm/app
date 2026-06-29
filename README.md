# TinyWasm

<img src="https://raw.githubusercontent.com/tinywasm/app/main/docs/img/badges.svg">

**LLM-Friendly Full-Stack Go Framework** — build complete web applications using only **Go + WebAssembly**, with minimal JavaScript. TinyWasm is a TUI-based development environment that acts as an intelligent intermediary between you, your LLM assistant, and your application.

> ⚠️ **Active development.** The TUI and MCP integration are evolving; expect rough edges.
> Source code is maintained privately — this repository distributes official binaries.

---

## What is TinyWasm?

A **TypeScript alternative for full-stack development — with Go everywhere.** Write backend, frontend, and shared logic in pure Go; your project structure is the configuration. The TUI handles all infrastructure (compilation, hot reload, browser, MCP) so you and your AI assistant focus on application code.

**Is it for you?** Clear advantages, disadvantages, and who should (and shouldn't) adopt it: **[docs/WHY.md](docs/WHY.md)**.

## What makes it different — the construction harness

The typed, explicit code **is** the harness: an agent that doesn't know the library still builds correctly, guided by the signatures, while the compiler rejects what's wrong. Less to learn, fewer runtime bugs, smaller docs. → **[docs/CONSTRUCTION_HARNESS.md](docs/CONSTRUCTION_HARNESS.md)**

---

## Installation

### Linux / macOS

```bash
curl -fsSL https://raw.githubusercontent.com/tinywasm/installer/main/scripts/install.sh | bash
```

### Windows (PowerShell as Admin)

```powershell
irm https://raw.githubusercontent.com/tinywasm/installer/main/scripts/install.ps1 | iex
```

The installer downloads the latest binary for your platform, verifies its SHA256 checksum, and places it on your `PATH`. Go and TinyGo are installed automatically — no prerequisites needed.

**Update:** `tinywasm --update` · **Manual download:** [Releases](https://github.com/tinywasm/app/releases) (Linux amd64/arm64, macOS arm64/amd64, Windows amd64).

## Quick start

```bash
mkdir myapp && cd myapp
tinywasm
```

TinyWasm will scaffold a new project, start the dev server on `https://localhost:6060`, launch the TUI with live logs, open Chrome with auto-reload, and start the MCP server on `http://localhost:3030/mcp`.

---

## Documentation

| Document | What's inside |
|----------|---------------|
| [docs/WHY.md](docs/WHY.md) | The problem it solves, advantages ✅ / disadvantages ❌, and who it's for |
| [docs/FEATURES.md](docs/FEATURES.md) | MCP integration, the three WASM modes, hot reload, TUI, pure Go stack, when to use JS |
| [docs/CONSTRUCTION_HARNESS.md](docs/CONSTRUCTION_HARNESS.md) | The typed, explicit API approach and why it's an advantage |
| [docs/MCP.md](docs/MCP.md) | MCP daemon: start, endpoints, tools, IDE config, diagnostics |

---

## Support

- **Issues**: [github.com/tinywasm/app/issues](https://github.com/tinywasm/app/issues)
- **Discussions**: [github.com/tinywasm/app/discussions](https://github.com/tinywasm/app/discussions)

## License

MIT — see [LICENSE](LICENSE)
