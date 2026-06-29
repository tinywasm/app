# Features

## 🤖 LLM integration via Model Context Protocol (MCP)

A fully functional MCP server lets AI assistants (Claude, GitHub Copilot, etc.):

- **See your UI** — capture screenshots and analyze the live browser state.
- **Read logs** — WASM compilation, server, and browser console.
- **Control compilation** — switch WASM modes (L/M/S) dynamically.
- **Manage the environment** — start/stop servers, reload the browser, check status.

Your LLM can debug visual issues and understand runtime behavior without you copy-pasting logs or
screenshots — saving tokens and time. Starts automatically at `http://localhost:3030/mcp`. Full
reference: [MCP.md](./MCP.md).

## 📦 Three WebAssembly compilation modes

| Mode | Compiler | Size | Use case |
|------|----------|------|----------|
| **L** (Large) | Go standard | ~2 MB | Development — full stdlib, fastest iteration |
| **M** (Medium) | TinyGo (debug) | ~500 KB | Debugging — balanced size and functionality |
| **S** (Small) | TinyGo (release) | ~200 KB | Production — minimal size, optimized |

Switch on the fly via the TUI or let your LLM choose based on context.

## 🔥 Intelligent hot reload

- **Backend** — recompiles and restarts the Go server on `.go` changes.
- **Frontend** — recompiles WASM and reloads the browser on `.go`/`.html`/`.css`/`.js` changes.
- **Asset pipeline** — minifies CSS/JS with cache busting.
- **SSR extraction** — pulls CSS/JS/HTML from `ssr.go` files across local and external modules.
- **Image optimization** — converts/optimizes images (WebP) from all project modules.
- **Smart detection** — watches only relevant files, ignores build artifacts.

## 🖥️ TUI development environment

Your development control center:

- Real-time status of server, WASM compiler, asset watcher, and browser.
- Color-coded logs from all components.
- One command: run `tinywasm` in your project directory.
- Chrome automation with live-reload injection.
- HTTPS on port 6060 with dev certificates.

The TUI manages infrastructure complexity so you (and your LLM) focus on application logic.

## 🌐 Pure Go stack

- **Backend** — Go standard library (no external frameworks required).
- **Frontend** — Go compiled to WebAssembly.
- **No transpiling** — no TypeScript, Babel, or JSX. **No bundlers** — assets handled internally.

### When to use JavaScript

TinyWasm provides the WebAssembly bootstrap automatically. Write vanilla JavaScript **only** when:

1. A browser API is performance-critical and `syscall/js` overhead is too high (rare).
2. You have an existing vanilla JS module not yet ported to Go.
3. There is no Go WASM wrapper for the API yet.

Otherwise, everything is Go.
