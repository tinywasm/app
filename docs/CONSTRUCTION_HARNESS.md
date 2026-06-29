# The construction harness — typed, explicit, hard to get wrong

TinyWasm's defining idea: **the typed, explicit code is itself the harness.** In an LLM-assisted
workflow the code is largely written by an agent, so the APIs are designed so that an agent that
doesn't know the library still builds correctly — guided only by the method signatures — while the
compiler rejects what's wrong.

## What this means for you

- **Typed over `any`.** No generic slots where the wrong value slips in unnoticed; each method takes
  the type that makes sense.
- **Explicit over implicit.** The method name states intent, so reading a call tells you what it does
  without opening the implementation.
- **Illegal states unrepresentable.** "I want this to change" has exactly one path, typed to require
  it — the wrong version doesn't compile.
- **Fail at compile time, not runtime.** What the compiler can't catch becomes a loud development
  warning, never a silent failure.
- **Minimal public surface.** You see only what you're meant to type; the engine's internals stay
  private, so there's nothing to misuse.

## Why it's an advantage

- **Less to learn.** You don't memorize rules; the signatures and autocomplete guide you.
- **Fewer runtime bugs.** Whole classes of mistakes become compile errors instead of mysterious
  behavior in the browser.
- **Readable code.** Each call declares its intent, so reviews and handoffs are faster.
- **Smaller docs.** Because the API is the harness, documentation shrinks to a short "how" cheat-sheet
  instead of long manuals — the types carry the rest.

This principle applies across the TinyWasm ecosystem libraries, not just one package.
