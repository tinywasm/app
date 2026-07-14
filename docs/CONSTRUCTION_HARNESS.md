# The construction harness — typed, explicit, hard to get wrong

> The ecosystem's guiding principle. It applies to **every** TinyWasm Framework.
> Pass it as context to an agent/LLM when designing or refactoring an API, so the result is
> aligned with how this ecosystem is built.

---

## The idea in one sentence

**The typed, explicit code is itself the harness.** In an LLM-assisted workflow the code is
largely written by an agent that does **not** know the library. That agent must be able to
produce correct code guided only by the method signatures, and the compiler must **reject**
what is wrong. The result has to be readable and hard to get wrong.

A direct consequence: **long "skill" documents full of context are unnecessary.** If the API is
a harness, a minimal "how do I…" cheat-sheet is enough — the types carry the rest.

---

## Why a harness and not a manual

A manual pushes correctness onto the reader: they must read it, remember it, and apply it
without slipping. A harness pushes correctness onto the **compiler and the signatures**: the
right path is the only one that exists, and the wrong one does not compile. For someone with no
context on the library, the second is orders of magnitude more reliable.

```go
// ❌ Generic hole: accepts anything; intent is lost and misuse fails at runtime.
func (b *Builder) Add(items ...any) *Builder

// ✅ Methods typed by intent: each states what it accepts; misuse does not compile.
func (b *Builder) Text(s string) *Builder
func (b *Builder) Child(c ...*Node) *Builder
func (b *Builder) Set(kv ...fmt.KeyValue) *Builder   // reuses types already declared in fmt
```

This is the house pattern. `tinywasm/json` already writes this way — one method per primitive
(`String`, `Int`, `Bool`, `Object`, `Array`), with `any` **only** at the I/O edge, never in the
data. And it **reuses types that already exist** (`fmt.KeyValue`, `*Element`) instead of
inventing new ones.

---

## Lego pieces: one concern, one typed contract

The harness is not only about method signatures — it is about **how the pieces fit together**.
Every project tends to rewrite the same wiring: how a module publishes its API, how a handler
receives request context, how middleware passes data. That is reusable logic trapped inside
each project.

So each concern is extracted into a **lego piece**: a single-responsibility library that owns
that concern and exposes a **typed contract**. Applications and server implementations do not
re-implement anything; they **assemble pieces**.

```go
// The seam: a module declares its identity; each boundary asserts the capability it needs.
type APIModule interface {
	model.ModuleNaming    // identity: ModelName()
	MountAPI(r Router)    // the module registers its own routes
}

// The composition root keeps modules by their minimal identity and asserts per boundary.
for _, m := range modules.All(db) {
	if api, ok := m.(router.APIModule); ok {
		api.MountAPI(r) // r is any concrete Router (native, edge, test double)
	}
}
```

**Capability bag + type assertion at the seam** is the assembly pattern: a module returns the
contracts it satisfies, and each boundary picks up only what it needs. Swapping an
implementation (a different `Router`, a fake `Caller` in tests) never touches the modules.

### The rules that keep the pieces lego

- **A consumer never re-creates a missing symbol locally.** If a library does not expose what
  you need, **stop and report it**. Recreating it downstream forks that library's
  responsibility, and the copy can never be reused.
- **A missing contract at a boundary is a defect in the library, not in the consumer.** If two
  libraries meet and there is no type to name the thing that crosses between them, the type is
  missing upstream. Do not declare a local intersection to paper over it.
- **Never wrap a library to fix its behaviour.** A wrapper that patches a defect is a fork with
  a friendlier name. Fix it where it lives and publish.
- **No `internal/` folders.** They are the signature of a forked or duplicated dependency
  instead of a contribution upstream.
- **The glue is written once, in the library that owns it.** If every application would write
  the same wiring, that wiring belongs to a piece — not to the applications.

> **Why this matters more than it looks.** An API gap always surfaces at the **leaf** (the
> application), where the agent has no authority to publish upstream — so it patches locally.
> Technical debt is then not an accident: the workflow guarantees it. These rules are what
> break that loop.

---

## The principles

Every API in the ecosystem must satisfy them.

1. **Typed over `any`.** No generic holes (`func(...any)`, `interface{}`) in the API — methods
   typed by intent, like the `tinywasm/json` writer. `any` is allowed only at the I/O edge,
   never in the data. **Reuse the types that already exist** (`fmt.KeyValue`, …) instead of
   duplicating them.
2. **Explicit over implicit.** The name declares the intent. Reading the call must be enough to
   know what it does, without opening the implementation.
3. **Illegal states unrepresentable.** If something must not happen, it must not be writable.
   One intent = exactly one path, typed to demand what it needs.
4. **One way to do each thing.** A single construction pattern, with no alternatives that force
   a choice or a trip to the docs.
5. **Minimal surface.** Export exactly what the author uses. Internal machinery stays
   unexported: what you cannot see, you cannot misuse.
6. **Fail at compile time, not at runtime.** Order of preference for catching an error:
   compile error → loud development diagnostic → (never) silent failure.
7. **Self-describing signatures.** Autocomplete must be enough to build. If using the API
   requires reading a long document, the API is incomplete.
8. **Closed by default.** When an API governs access (permissions, visibility, exposure), the
   default state is **deny**. Granting access — or making something public — is an explicit,
   typed act, never the zero-value nor an implicit rule. A resource left reachable because
   *nobody said otherwise* is a silent failure (it violates principle 6). The safe default must
   be the one you get by writing nothing; opening is what costs an explicit, greppable line.
9. **Lego pieces, never forks.** One concern per library, exposed as a typed contract.
   Consumers assemble; they do not re-implement, wrap, or copy. A gap is fixed upstream.

---

## The rule that keeps the harness honest

> **An API is not published until a consumer-shaped test, inside the library itself, proves
> it.**

A library tested only in isolation — with opaque doubles standing in for its real
collaborators — hides its gaps until a consumer hits them. And a consumer can only *patch* a
gap, never fix it (see the lego rules above).

So the proof that an API is a harness is a test **in the owning library** that goes through the
real stack a consumer will use. Concretely: if a CRUD layout is meant to take a model, generate
a form from it, and ship it through a caller, then the library must contain a test that does
exactly that — with a real model, the real form package, and a fake caller. If that test is
awkward to write, the API is awkward to use, and you have found the defect before shipping it.

---

## Aligning a refactor (checklist)

When reviewing or rewriting a library, hunt for and fix:

- **Untyped holes.** Every `any`/`interface{}`/`...any` in the public API → replace with methods
  typed by intent (like `tinywasm/json`), reusing types already declared in `fmt`.
- **Invariants checked at runtime.** Things validated today with an `if` plus an error or a
  panic → can they move into the type system so that wrong code does not compile?
- **Things you "have to remember".** Any mandatory step the author must remember to call (call
  order, "don't forget X") → that is a hole in the harness; close it with types or a single
  path, not with prose.
- **More than one way to do the same thing.** Redundant paths → collapse to one.
- **Exported plumbing.** Public symbols only the library itself uses → make them internal.
- **Silent failures.** Cases where misuse produces neither an error nor a visible effect → turn
  them into compile errors; if that is impossible, into a loud development diagnostic.
- **Open defaults.** Access APIs whose zero-value permits instead of denies (a missing
  `Authorizer` meaning "allow everything", an implicit "public") → invert to **deny by
  default**; public and open are declared explicitly and typed.
- **Missing contracts at the seams.** A boundary where a consumer would have to declare a local
  interface to name what crosses it → name it here instead.
- **Glue a consumer would repeat.** Wiring that every application would write identically →
  move it into the piece that owns it.

Closing rule: after the refactor, the only possible failure modes must be a **compile error** or
a **loud development diagnostic** — never a runtime mystery.

---

## What this means for documentation

Because the API is the harness, **documentation shrinks to minimal "how" instructions** — not
skills with pages of context:

- **Yes:** a "I want to do X → use Y" table, and one short example per common use case.
- **Yes:** the typed signatures; let autocomplete guide.
- **No:** long documents explaining rules the compiler already enforces.
- **No:** "remember to call…" / "don't forget…" — if it must be remembered, it is a hole in the
  harness.

---

## Why it is an advantage

- **Less to learn.** You don't memorize rules; the signatures and autocomplete guide you.
- **Fewer runtime bugs.** Whole classes of mistake become compile errors instead of mysterious
  behaviour in the browser.
- **Readable code.** Each call declares its intent, so reviews and handoffs are faster.
- **Smaller docs.** The types carry what a manual would have had to say.
- **Reusable by construction.** A gap fixed in its own piece is fixed for every application at
  once — instead of being patched again in each one.

---

## The acid test

If an agent **with no context** on the library produces correct code guided only by autocomplete
and a few-line example, the harness is closed. If it needs to read a manual to avoid mistakes,
something is still untyped.

And if that agent has to **ask a question** to proceed — "is it acceptable to declare this
interface locally?" — the harness has already failed by its own definition: the signature did
not guide, and the compiler rejected the correct intent.
