# MASTER PLAN — SVG sprite runtime assembly (non-deterministic icon serving)

Multi-repo coordination hub. Make the served inline SVG sprite **deterministic**:
every module's icons must appear on every run, not "sometimes". Today only the
icons under the served root subtree are guaranteed; all others depend on a racy,
single-shot, silently-failing background scan.

> Dispatch: 2026-07-22 · **Status: 🚧 PROPOSED**
> Coordinates: `assetmin/docs/PLAN.md` (primary — owns the scan + sprite
> accumulation + serving) and `app/docs/PLAN_SPRITE_RUNTIME.md` (composition-root
> wiring). Extraction (`tinywasm/ssr`) is **correct and needs no change** —
> proven below. Related: `SVG_ICON_HARNESS_MASTER_PLAN.md` (the WASM-leak split,
> already shipped), `CONSTRUCTION_HARNESS.md` (principle 6: fail loud, never
> silent).

---

## 1. Symptom

The demo (`layout`, served from `layout/platformd/web`) renders with only
platformd's nav icons (`home`, `products`, `info`). The crudview toggle (`+`/`↺`),
the search magnifier, and the targetlist `⋮` dots render blank — their `<use>`
references resolve to nothing because those `<symbol>`s are absent from the
served sprite. **Sometimes** they are present (a previous daemon instance served
the full set, with duplicates); after a restart they vanish. "A veces se
configura bien y otras no."

## 2. Root cause — evidence (not a guess)

Extraction is correct and stable. Replicating the daemon's exact extractor
(`ssr.New("…/layout/platformd")`, default finder, ssr v0.0.16):

```
SYNC  ReloadSSRModule(rootDir=…/layout/platformd) => module …/layout/platformd  icons=3
ASYNC ExtractAll()                                => …/layout        icons=6   (platformd 3 + crudview 3)
                                                    …/components     icons=2   (targetlist + selectsearch)
                                                    TOTAL icons=8
```

Two independent extraction passes returned the same 8 — no drift (the earlier
`ssr` cache-mutation bug is fixed, v0.0.16). So the icons are extracted
correctly, every time. The loss is entirely in **runtime assembly**:

1. **The synchronous inject covers only the root subtree.** `app/section-build.go`
   calls `ReloadSSRModule(h.RootDir)` once, synchronously, at startup. With
   `RootDir = layout/platformd`, that merges only `layout/platformd`'s 3 icons.
   crudview (`layout/crudview`, a *sibling* of platformd) and the `components`
   module are **not** under that subtree.

2. **Everything else depends on one async, best-effort scan.** crudview's and
   components' icons reach the sprite only through `LoadSSRModules()` →
   `ScheduleSSRLoad()` → `ExtractAll()`, launched in the step-9 background
   goroutine of `section-build.go`. That goroutine races the first HTTP serve
   and the concurrent WASM build.

3. **Failures are silent and never retried.** `ScheduleSSRLoad` does
   `if all, err := ssrExtractor.ExtractAll(); err == nil { … } else { c.Logger("SSR ExtractAll error:", err) }`
   — and `assetmin`'s `log` is **nil** (the app never calls
   `AssetsHandler.SetLog`), so the error is dropped into the void. The scan is
   single-shot: one transient `go run` failure (the extractor compiles+runs a
   generated `main.go` while the WASM build is hammering the same build/module
   cache) permanently loses every non-root module's icons **and CSS** until an
   unrelated file save happens to re-extract that one module. This is the
   non-determinism.

4. **Accumulation is not idempotent.** `assetmin.mergeSprite` does
   `masterSprite.Merge(s)` (append), and `sprite.String()` emits every entry
   with no dedup. Re-extraction on hot reload therefore appends duplicate
   `<symbol>`s (observed: the full icon set served twice), and because a browser
   uses the *first* symbol of a given id, a hot-reloaded icon edit is shadowed
   by its stale earlier copy. CSS and JS already avoid this — they are stored
   **per module name** in slots (`UpdateContentInSlot(name, …)`); only icons
   were left as a blind append.

The served sprite therefore equals whatever `masterSprite` happens to hold at
request time: the 3 sync-guaranteed icons always, plus the async set only if the
scan won its race and didn't error — with duplicates when it ran more than once.

## 3. Decision

Keep the fast synchronous first-paint inject, but make the authoritative mass
scan **reliable and idempotent**, and stop serving a half-built sprite as if it
were final. Concretely:

- **Icons become module-keyed, exactly like CSS/JS.** Re-extracting a module
  *replaces* that module's icons instead of appending — kills duplicates and
  stale-first-symbol in one move, and removes the special-case (principle 4,
  "one way to do each thing").
- **The mass scan retries until it succeeds once, and never fails silently.**
  A transient extractor failure must not permanently drop assets.
- **Diagnostics are wired** so a real failure is loud (principle 6).

This is owned primarily by `assetmin` (it owns the scan scheduling, the sprite,
and the serving). `app` only wires the logger and re-triggers a browser reload
once the first full scan lands.

## 4. Phases

| Phase | Repo / plan | Scope | Execution | Status |
|---|---|---|---|---|
| **A (primary)** | `assetmin/docs/PLAN.md` | (1) module-keyed sprite storage so re-extraction replaces, not appends — dedup by id; (2) `ScheduleSSRLoad` retries with bounded backoff until one success, error surfaced via `Logger` not swallowed; (3) refresh `.svg` after the background scan merges. + regression tests. | codejob | ☐ |
| **B (wiring)** | `app/docs/PLAN_SPRITE_RUNTIME.md` | (1) `AssetsHandler.SetLog(h.Watcher.Logger)` so assetmin diagnostics are visible; (2) after the first successful `LoadSSRModules` scan, call `h.Watcher.RequestReload()` so the browser re-fetches the now-complete sprite. | codejob | ☐ |

Phase A is self-sufficient for correctness (the dynamic sprite handler + retry
means the sprite eventually completes and the existing post-reload re-serve picks
it up). Phase B makes it observable and removes the manual-reload dependency.
`ssr` is unchanged.

## 5. Verification (definition of done)

Deterministic across repeated cold starts (no manual reload, no lucky race):

```bash
# after daemon start, poll the served sprite until the scan lands, then assert:
curl -s localhost:8080/ | grep -o '<symbol id="[^"]*"' | sort | uniq -c
#  → each of icon-crud-new, icon-crud-cancel, icon-crud-search, home, products,
#    info, tl-dots, ss-arrow-down appears EXACTLY once (count 1, no duplicates)
```

- Kill + relaunch the daemon 5× → the full set every time (was: intermittent).
- Edit a component's `svg.go` (change a path) → the served symbol updates in
  place, not shadowed by a stale duplicate.

## 6. Rules

- An agent in one phase does NOT touch another repo's contracts; on conflict,
  STOP and report (per `CONSTRUCTION_HARNESS.md`).
- On closing a plan: mark it here, record the permanent decision in the repo's
  ARCHITECTURE/AGENTS.md, delete the dispatched `docs/PLAN.md`.
- The fix lives where the concern lives: icon accumulation + scan reliability in
  `assetmin`; logger wiring + reload trigger in `app`. Do not paper over it in
  the leaf (`layout`) by hand-registering icons — that would fork the concern.
