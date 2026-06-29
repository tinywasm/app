# Why TinyWasm — advantages, disadvantages, and who it's for

TinyWasm is a **TypeScript alternative for full-stack development — but with Go everywhere.** Build
production web apps using **pure Go** for backend and frontend, without the modern JavaScript
toolchain, while keeping type safety across the whole stack.

---

## The problem it solves

Modern web development suffers from:

- **Configuration hell** — `package.json`, `webpack.config.js`, `tsconfig.json`, and more.
- **Tooling complexity** — multiple bundlers, transpilers, and package managers.
- **Frontend/backend split** — different languages, patterns, and ecosystems.
- **LLM context waste** — assistants spend tokens explaining infrastructure instead of building features.

## The approach

- **Convention over configuration** — your project structure *is* the configuration.
- **Single language** — share code, types, and validation between client and server.
- **Minimal dependencies** — Go standard library + minimal vanilla JS only when strictly necessary.
- **LLM-optimized** — the TUI handles infrastructure so AI assistants focus on application code.

---

## Advantages ✅

- One language (Go) for backend, frontend, and shared logic — no context switching.
- Zero frontend build config; no npm, bundlers, or transpilers to maintain.
- Tiny bundles: ~200 KB–2 MB depending on compilation mode.
- Type safety end to end; share validation and types across client/server.
- First-class LLM workflow via the MCP server (see live UI, logs, control compilation).
- Typed, explicit APIs designed so code is easy to read and hard to get wrong — see
  [CONSTRUCTION_HARNESS.md](./CONSTRUCTION_HARNESS.md).

## Disadvantages ❌ (be honest before adopting)

- **Active development** — the TUI and MCP integration are evolving; expect rough edges.
- **Young ecosystem** — Go libraries for browser APIs are still growing; you may hit a missing wrapper.
- **Go-for-frontend learning curve** — you write UI in Go, not HTML/JS frameworks.
- **`syscall/js` overhead** — a few performance-critical browser APIs may still need vanilla JS.
- **Not a fit for JS-centric stacks** — no React/Vue/Angular, no npm ecosystem, no SASS/PostCSS/module
  federation pipelines.

---

## Who should use TinyWasm

**Good fit:**

- Go developers who want web frontends without learning React/Vue/TypeScript.
- Teams seeking type safety without the Node.js toolchain.
- LLM-assisted development where the assistant should focus on code, not tooling.
- Projects that benefit from sharing validation/business logic across client and server.
- Solo developers who value zero configuration and fast time-to-market.

**Not a fit:**

- Projects that require React, Vue, or Angular.
- Teams deeply invested in the npm ecosystem.
- Apps whose frontend is primarily JavaScript/TypeScript rather than Go.
- Complex frontend build pipelines (SASS, PostCSS, module federation, etc.).
