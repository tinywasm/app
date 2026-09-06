---
PLAN: "feat: an API explorer that reads /_routes and lets you call each endpoint"
EXECUTOR: jules
REVIEWER: none
---

> This plan is dispatched via the CodeJob workflow. See skill: agents-workflow.
>
> **This file is not in a repository yet.** The repo it belongs to does not
> exist. Create it first — `gonew --owner tinywasm apiexplorer` — then copy this
> file to `docs/PLAN.md` inside it and dispatch from there. Creating a GitHub
> repository is the account owner's action, not an execution agent's.
> Without `--owner tinywasm` the repo lands under the personal account.

# Plan — `tinywasm/apiexplorer`

## What this is

A page that fetches `/_routes` from a running server and renders it as a table
an operator can read and act on: every endpoint, its access, the permission it
requires, **which roles hold that permission**, and a form to call it.

## Why it is a separate package, and why it renders in the browser

The route table is already served as JSON by `router.MountIntrospection` — from
httpd and from a Cloudflare Worker alike. What is missing is the reading of it.

Three places could render that HTML. Two are wrong:

- **The Worker.** It would pull `tinywasm/html` and the UI kit into a binary
  with a hard 1 MB Cloudflare limit. `veltylabs/misitio`'s Worker sits at
  662 KB today. Not a candidate.
- **`tinywasm/server/httpd`.** It is `!wasm`: the explorer would exist on the
  dev server and be absent from production — the deployment you most need to
  interrogate.

So the HTML is built **in the browser**, from JSON any implementation already
serves. The server pays one `json.Encode` of a slice it already has in memory.

It is a repo of its own rather than a subpackage of `tinywasm/router` because
`router` is the ecosystem's contract package: it has three light dependencies
and every consumer inherits them. Adding `dom`/`html`/`fetch` there would put
the browser kit in the go.mod of everything that routes.

## Dependency

Requires **`github.com/tinywasm/router`** at the version exporting
`IntrospectionPath` and `MountIntrospection`. **Already published** when this
plan is dispatched. `go get github.com/tinywasm/router@latest`. Never add a
`replace`, never invent a version.

## Anti-footguns

- `apiexplorer.go` is `//go:build wasm` and compiles into a browser binary.
  Use `tinywasm/fmt`, `tinywasm/json`, `tinywasm/dom`, `tinywasm/html`,
  `tinywasm/fetch`. No stdlib `fmt`/`strings`/`errors`, no `encoding/json`, no
  `reflect`.
- **SSR split by extension**, per the ecosystem convention: `html.go`
  (`RenderHTML`), `css.go` (`RenderCSS`) — each `//go:build !wasm`. Never an
  `ssr.go`, never a `front.go`.
- Embed `dom.Element` **by value**, never as `*dom.Element`.
- This is a developer tool. It never stores a credential, never remembers a
  request body across reloads, and never sends anything the operator did not
  press a button to send.
- English: code, comments and docs.

---

## Stage 1 — Decoding the wire

File: **`routes.go`** (no build tag — both halves may want the shape).

`router.RouteInfo` encodes but does not decode, so declare the reading shape
here as a `model.Decodable`. Match `router.MountIntrospection`'s output
exactly:

```go
// Route is one entry of the /_routes table as the browser reads it back.
type Route struct {
	Method      string
	Path        string
	Resource    string
	Action      string // CRUD letters: "r", "ru", "crud"
	Access      string // "public" | "authenticated" | "guarded"
	PolicyKnown bool
	Roles       []string

	// Args is the schema Route.Accepts declared, absent when the route takes
	// no body. Absent and empty are different claims — keep them apart.
	Args    []ArgField
	HasArgs bool
}

// ArgField is one field of a route's declared body schema.
type ArgField struct {
	Name     string
	Kind     string
	Required bool
}

// Orphan reports a guarded route whose required permission NO role holds.
// It is the finding this tool exists for: such a route answers 403 to every
// caller, forever, while looking perfectly declared.
//
// It is false when PolicyKnown is false — "the server did not say" is not the
// same claim as "nobody has it", and rendering them the same way would cry
// wolf on every server that does not describe its policy.
func (r Route) Orphan() bool {
	return r.Access == "guarded" && r.PolicyKnown && len(r.Roles) == 0
}
```

Plus `Table []Route` with `DecodeFields` reading the `"routes"` array.

## Stage 2 — The table

File: **`apiexplorer.go`** (`//go:build wasm`).

```go
// Options configures a mounted explorer.
type Options struct {
	// RoutesURL is where the table is fetched from.
	// Empty means router.IntrospectionPath on the same origin.
	RoutesURL string
}

// Mount fetches the route table and renders the explorer into containerID.
// A failed fetch renders the error in place — never an empty page, which
// reads as "this server has no routes".
func Mount(containerID string, opts Options)
```

One row per route, columns: **method · path · access · permission · roles ·
call**.

Rendering rules, and they are the product:

- `Orphan()` rows are marked visibly and sorted **first**. A tool that buries
  the one broken thing in row 40 has not reported it.
- `PolicyKnown == false` renders the roles column as `—` with a title
  explaining the server did not describe its policy. Never as an empty list.
- `access: public` is marked too. A public route is a deliberate decision and
  worth seeing at a glance.
- Permission renders as `resource:action` (`site_content:u`), empty for
  non-guarded routes.

## Stage 3 — Calling an endpoint

Same file. Each row expands into a form:

- **Path** — pre-filled with the pattern. Every `{name}` becomes its own
  labelled input, and the request path is rebuilt from them. Do not hand the
  operator a raw string with braces in it to edit by hand.
- **Body** — built from the route's `args` (what `Route.Accepts` declared and
  `MountIntrospection` serializes): one labelled input per field, assembled into the request
  body. A route with no `args` key gets a plain textarea instead — that is the honest
  fallback, not the default. **Never ask the operator to hand-write JSON for a route that
  already told you its schema.**
- **Send** — issues the request through `tinywasm/fetch` with
  `credentials: include` so the session cookie travels; renders status, then
  headers, then body.
- A `403` is annotated with the permission the route declared, so the operator
  reads *"403 — requires `site:r`, held by: nobody"* instead of a bare number.
  That single line is what turns this from a listing into a diagnosis.

Never send automatically on expand. A `DELETE` fired by opening a row is a
tool that destroys data by being read.

## Stage 4 — The standalone page

- **`html.go`** (`//go:build !wasm`) — `RenderHTML() string`: the shell with the
  container element the wasm binary mounts into. `sitec` picks it up as a
  producer.
- **`css.go`** (`//go:build !wasm`) — `RenderCSS() string`, built from
  `tinywasm/css` tokens. No literal colours: an orphan row is a semantic state
  (`danger`), not a hex value.
- **`cmd/apiexplorer/main.go`** — thin: parse a `-url` flag, print help and exit
  `0` with no args, stderr for logs and stdout for data. All logic in the
  library.

## Stage 5 — Tests

Under **`tests/`**.

| Test | Asserts |
|---|---|
| `TestDecodeTable` | a fixture JSON matching `MountIntrospection`'s output decodes into the right routes |
| `TestOrphanDetectsUnheldPermission` | guarded + `policy_known` + empty roles → true |
| `TestOrphanIsFalseWhenPolicyUnknown` | guarded + `!policy_known` + empty roles → **false** |
| `TestOrphanIsFalseForPublic` | public route → false |
| `TestOrphansSortFirst` | ordering puts every orphan above every healthy route |
| `TestPathBuilderFillsParams` | `/api/sites/{site}/assets/{key}` + `{site:a, key:b}` → `/api/sites/a/assets/b` |
| `TestPathBuilderRejectsEmptyParam` | a blank value produces an error, not `/api/sites//assets` |
| `TestArgsAbsentIsNotArgsEmpty` | a route with no `args` key decodes to `HasArgs == false`, not an empty slice |
| `TestRenderHTMLHasContainer` | the shell contains the id `Mount` expects |

Decoding and the pure helpers must be testable **without a browser**: keep them
out of the `//go:build wasm` file. That split is the reason Stage 1 has no build
tag.

## Stage 6 — Documentation

- **`README.md`** — what it is, the two ways to use it (mounted into an existing
  panel, or as its own page), a screenshot-shaped description of the table, and
  a loud paragraph on **not** serving it publicly in production: the full
  permission map of a service is a map of what to attack.
- **`docs/ARCHITECTURE.md`** — the three-layer split (router owns the data,
  every transport serves the JSON, this package renders it in the browser) and
  why the Worker must never render it.
- **`AGENTS.md`** — the repo's constraints, from this plan's anti-footguns.
- Do **not** link `docs/PLAN.md` from any permanent document.

## Acceptance criteria

- [ ] `go build ./...`, `go vet ./...` clean; `GOOS=js GOARCH=wasm go vet ./...`
      clean; `gotest ./...` green.
- [ ] `grep -rn "\"strings\"\|\"errors\"\|encoding/json\|\"reflect\"" .` → empty
      outside `tests/`.
- [ ] `grep -rn "\*dom.Element" .` → empty (value embedding only).
- [ ] `ls ssr.go front.go` → fails.
- [ ] No hex colour literal outside `css.go`.
- [ ] `Orphan()` distinguishes "nobody holds it" from "policy unknown", with a
      test for each.
- [ ] Nothing is sent without an explicit click; no test drives a request as a
      side effect of rendering.
- [ ] `go.mod` has no `replace` directive.

## Out of scope

Editing a policy, persisting request history, authentication of its own
(it rides the browser's session cookie), and OpenAPI export.

## Stages

| # | Stage | Files |
|---|---|---|
| 1 | Wire decoding | `routes.go` |
| 2 | The table | `apiexplorer.go` |
| 3 | Calling an endpoint | `apiexplorer.go` |
| 4 | Standalone page | `html.go`, `css.go`, `cmd/apiexplorer/main.go` |
| 5 | Tests | `tests/` |
| 6 | Documentation | `README.md`, `docs/ARCHITECTURE.md`, `AGENTS.md` |
