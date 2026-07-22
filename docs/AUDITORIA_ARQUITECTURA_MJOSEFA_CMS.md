# Auditoría de arquitectura — `veltylabs/mjosefa-cms` (Fase D)

> Fecha: 2026-07-17 · Auditoría paralela al PR #3 de `mjosefa-cms` (en corrección por agente cloud).
> Contexto: [`REUSABLE_MODULES_MASTER_PLAN.md`](REUSABLE_MODULES_MASTER_PLAN.md). Objetivos
> evaluados: **mantenibilidad**, **seguridad por defecto**, comunicación futura vía `tinywasm/sse`,
> y capacidad de **enchufar y configurar** los módulos de `veltylabs/modules/*` sin fricción.

---

## 1. Veredicto general

La arquitectura de composición de la app es **correcta y no necesita rediseño**: bag de capacidades
(`modules.Init → []any`), cosecha neutral (`router.OpModule → mcp.HarvestOps`), costuras inyectables
con cero-valor = producción (`ServerDeps`), deny-all por defecto y validación RBAC fail-fast en
`BuildServer` (D5). El patrón escala a N módulos sin tocar `config/server.go` (solo una línea por
módulo en `modules/init.go`).

Lo que sí encontré es **un hallazgo crítico aguas arriba** (§2) que el PR #3 tapó con forks locales,
más ajustes menores app-local (§3) y huecos concretos para la ola SSE y el fan-out de módulos (§4–§5).

---

## 2. Hallazgo crítico (aguas arriba): `tinywasm/user` compila contra el orm viejo

**Síntoma en la app:** `go.mod` de `mjosefa-cms` contiene dos `replace` hacia carpetas locales
(`./unixid`, `./local_orm`) — las carpetas que motivaron las críticas #2 y #3 del PR.

**Causa raíz (verificada compilando la app sin los `replace`):** `tinywasm/user@v0.0.37-…` usa la
API de `orm v0.9` que `orm v0.11` eliminó:

```
authority/migrate.go:19: db.CreateTable undefined (type *orm.DB has no field or method CreateTable)
authority/rbac.go:193:   undefined: orm.ErrNoRows
authority/rbac.go:237:   undefined: orm.ErrNoRows
```

Es exactamente el mismo drift que el master plan ya registró para `item_catalog` (fila C, hoy §2
del master consolidado: `orm.DB.CreateTable` se movió a `tinywasm/ddl`) — pero al momento de esta
auditoría **la fila de `tinywasm/user` no existía en el master plan** (rectificado: filas B6a/B6b).
El agente del PR, sin poder tocar repos aguas arriba, hizo lo que el master plan prohíbe (§3
"Decisiones cerradas": aguas arriba, nunca fork local — ni wrapper, ni `replace` a carpeta local):

- `local_orm/`: copia del orm que añade `func (d *DB) CreateTable(schema any) error { return nil }`.
  **Ese no-op desactiva en silencio la creación del esquema completo de autenticación**
  (`user/authority/migrate.go` crea las tablas `user`, `role`, `permission`, `session`, …). En una
  base fresca de producción el login/bootstrap de admin falla o —peor— arranca contra un esquema a
  medias. Es la antítesis de "seguridad por defecto": una ruta de seguridad que falla **en
  silencio**. Regla que propongo dejar escrita: *un shim de compatibilidad jamás puede ser no-op en
  una ruta de seguridad; si no puede implementarse, debe devolver error*.
- `unixid/`: copia que añade `GetNewID()` como alias de `NewID()`. **Vestigial**: verifiqué que la
  app compila sin este `replace` (nadie llama `GetNewID` ya; `unixid v0.2.24` publicado incluye el
  renombre a `NewID()`). Se puede borrar hoy mismo.

**Remedio (ya preparado):** escribí
[`tinywasm/user/docs/PLAN.md`](https://github.com/tinywasm/user/blob/main/docs/PLAN.md) —
autocontenido, listo para dispatch — que migra `authority` a `ddl.Compiler` (type-assert) +
`ddl.TopologicalSort` + `ddl.Sync`, renombra `orm.ErrNoRows → orm.ErrNotFound` y cierra la
migración `ddlc.FieldExt → model.FieldExt` (que quedó a medias, sin commitear, en el working tree
local del repo). Secuencia:

1. Despachar `tinywasm/user` PLAN → publicar `v0.0.37`.
2. En `mjosefa-cms`: eliminar `unixid/` (**ya**, no depende de nada) y, tras el paso 1, eliminar
   `local_orm/` + ambos `replace` y pinear `user v0.0.37`.
3. Añadir la fila `tinywasm/user` al master plan (hoy es un hueco de seguimiento).

---

## 3. Críticas del PR #3 — clasificación app-local vs aguas arriba

| # | Crítica | Ámbito | Veredicto |
|---|---|---|---|
| 1 | `BuildClient(cfg, deps)` / `RunServer(cfg, deps)` → una sola estructura | **App-local** | Viable sin costo: fusionar las costuras dentro de `ServerConfig`/`ClientConfig` conservando la semántica *cero-valor = producción* (los campos `OpenDB/Providers/Authn/…` pasan a la config; los tests los llenan igual). Actualizar `docs/ARCHITECTURE.md` y diagramas, que hoy documentan el par datos/costuras. |
| 2 | Carpeta `unixid/` | **Mixto** | El `replace` es innecesario **hoy** (verificado: compila sin él) — borrar carpeta + `replace` de inmediato. La causa que lo originó era upstream y ya está publicada (`unixid v0.2.24` con `NewID()`). |
| 3 | Carpeta `local_orm/` | **Aguas arriba** | NO puede borrarse todavía sin romper el build: depende de publicar `tinywasm/user v0.0.37` (PLAN ya escrito, ver §2). Borrarla antes de eso obligaría al agente a inventar otro parche peor. |
| 4 | Todos los tests en `tests/` raíz + regla en AGENTS.md | **App-local** | Viable: mover `config/auth_wire_test.go` y `modules/item_catalog/tests/*` a `tests/`. Los tests wasm siguen activándose por `//go:build wasm`, no por ubicación, así que `gotest` no cambia. **Ojo consistencia:** `docs/ARCHITECTURE.md` §2 hoy prescribe lo contrario (`modules/<mod>/tests/`) — hay que reescribir esa sección y el árbol de directorios del mismo doc, no solo AGENTS.md. |
| 5 | `client_stdlib.go`/`client_wasm.go` con build tags | **App-local** | El split existe para un `client.go` **neutral** (testeable en stdlib), que es lo que documenta ARCHITECTURE.md — pero el `client.go` actual lleva `//go:build wasm`, lo que vuelve el split redundante (el shim stdlib es código muerto). Dos salidas coherentes: (a) **recomendada** — quitar el tag de `client.go` (volverlo neutral como manda el doc; el carril de vista vuelve a correr en tests stdlib) y el par de archivos `reloadWindow` queda justificado; o (b) mantener `client.go` como wasm y borrar `client_stdlib.go`. Lo incoherente es el estado actual (tag wasm + shim stdlib a la vez). Nota menor: `syscall/js` directo en `config/` viola la regla de abstracción — `dom` no expone hoy un `Reload()`; si (a), valdría un helper aditivo en `tinywasm/dom`. |

---

## 4. SSE (`tinywasm/sse`) — estado real y lo que falta para la ola de push

Buena noticia que el master plan aún no refleja: **B5 está casi cerrada y B3 ya cerró**:

- `sse v0.1.1` (publicado) ya incluye `sse.Publisher`, el adaptador a `events.Publisher`
  (commit `a280fef`, contenido en los tags `v0.1.0`/`v0.1.1`). La fila B5 dice "pendiente de
  investigación" — desactualizada.
- `unixid v0.2.24` (publicado) ya tiene `NewID()` — la fila B3 dice "sin despachar" — desactualizada.

Lo que **sí** falta para que un módulo publique un evento y el navegador lo reciba (plan futuro
"push", hoy explícitamente fuera de alcance de Fase D — correcto):

1. **Montaje del stream en la app**: `httpd` ya soporta `router.Stream(path, StreamFunc)` y `sse`
   expone `StreamHandler() router.StreamFunc`. Falta el cableado en `config/server.go` (una ruta,
   p. ej. `/events`).
2. **Seguridad del stream (crítico para "seguro por defecto")**: la ruta SSE debe pasar por el
   `Authn` global y por `Authorize` como cualquier recurso (cookie de sesión), y el
   `ChannelProvider` de `sse` debe resolver el canal desde la **identidad autenticada**
   (`ctx.UserID()` / tenant), nunca desde un parámetro del cliente. Si el canal lo elige el
   navegador, un usuario se suscribe a eventos de otro. Esto debe quedar como criterio de
   aceptación del plan SSE, no como nota.
3. **Fan-out de brokers en el composition root**: hoy `ServerDeps.Events` es *un* `Publisher`
   (`events/mock.Broker` in-proc). Con SSE serán **dos destinos** (in-proc módulo-a-módulo + push
   al navegador). Falta un combinador — propuesta: `events.Fanout(...Publisher) Publisher` aditivo
   en `tinywasm/events` (trivial, lo usarán todas las apps), o en su defecto 5 líneas en `config/`.
4. **Lado navegador**: `sse.SSEClient` expone `OnMessage(func(*SSEMessage))` pero no hay adaptador
   a `events.Subscriber`, así que las vistas wasm no pueden suscribirse con el mismo contrato
   tipado que el servidor. Es el simétrico del `sse.Publisher` ya hecho — candidato a cerrar B5.

Recomendación: rectificar la fila B5 del master plan con estos 4 puntos como su alcance restante
(los puntos 1–2 viven en el plan de la app; los 3–4 en `events`/`sse`).

---

## 5. Harness de módulos — qué falta para "enchufar y configurar" N módulos

Con 7 módulos en cola (Fase E: `agent_switch`, `appointment_booking`, `business_hours`,
`clinical_encounter`, `provider_payouts`, `work_schedule`, + `item_catalog`), los puntos de fricción
que hoy son triviales se multiplican por N:

- **Drift módulo ↔ recurso RBAC.** La convención "ID de módulo == recurso RBAC" vive en tres sitios
  que nada valida entre sí: `const ID` del módulo app, `Resource` de las tools del módulo de
  dominio, y `config.Modules()`. Con 1 módulo es revisable a ojo; con 8 es un bug latente de
  permisos. Propuesta barata (app-local, sin librería nueva): un test en `tests/` que recorra los
  `OpModule` cosechados y verifique que cada `Resource` declarado existe en `config.Modules()` y
  viceversa. Cero infraestructura, detecta el drift en CI.
- **Detección temprana de drift de API entre libs (la lección de §2).** El incidente
  `user`/`CreateTable` ocurrió porque cada repo compila contra sus pins viejos y nadie compila el
  conjunto hasta que la app integra. Propuesta: un job de integración en `app-releases` (o en el CI
  de `mjosefa-cms`) que haga `go get -u` de todo el ecosistema `tinywasm/*` + `veltylabs/*` en una
  rama descartable y compile — no publica nada, solo avisa. Habría detectado tanto la rotura de
  `item_catalog` como la de `user` el día que se publicó `orm v0.11`.
- **Seed de permisos por módulo.** `authority.Register(RBACObject)` ya existe (permisos por
  recurso/acción/rol), pero el bag de capacidades no lo transporta: hoy los grants se siembran a
  mano en `ProductionDeps` (admin con wildcard). Cuando entren módulos con roles diferenciados
  (recepción vs clínico), conviene que el módulo de dominio exponga su declaración RBAC como una
  capacidad más del bag y `ProductionDeps` la registre — mismo patrón dispatch-por-capacidad, sin
  contrato nuevo aguas arriba (basta implementar `RBACObject` de `user`).
- **`work_schedule` viola la regla anti-fork** (`replace … => ../../../tinywasm/mcp`, ya anotado en
  master plan §3 "Decisiones cerradas"). Con el precedente de §2, cerrar eso en su rectificación de
  Fase E no es opcional: es el mismo patrón que acaba de morder en producción.

Nada de esto requiere rediseñar el harness: son verificaciones y cableado sobre el patrón existente.

---

## 6. Desalineaciones de documentación detectadas

| Documento | Desalineación | Acción |
|---|---|---|
| `REUSABLE_MODULES_MASTER_PLAN.md` §1 | B3 (`unixid`) figura "sin despachar" — publicado `v0.2.24` con `NewID()`. B5 (`sse`) figura "pendiente de investigación" — `sse.Publisher` publicado en `v0.1.1`. No existe fila para `tinywasm/user` (rota por el mismo drift que la fila C) | Rectificar filas B3/B5; añadir fila `user` apuntando a su nuevo `docs/PLAN.md` |
| `mjosefa-cms/docs/ARCHITECTURE.md` | §2 y árbol de directorios prescriben `modules/<mod>/tests/`, contradiciendo la crítica #4 del PR (todos los tests en `tests/` raíz); documenta el par `cfg, deps` que la crítica #1 elimina; documenta `client.go` neutral mientras el código lleva `//go:build wasm` | Reescribir junto con las correcciones del PR — código y doc en el mismo commit |
| `mjosefa-cms/AGENTS.md` | Falta la regla explícita de tests (petición del PR #4) | Añadirla al aplicar la crítica #4 |

---

## 7. Resumen de acciones

| Prioridad | Acción | Dónde | Estado |
|---|---|---|---|
| 🔴 1 | Despachar PLAN de `tinywasm/user` (ddl.Sync + ErrNotFound + FieldExt) y publicar `v0.0.37` | `tinywasm/user/docs/PLAN.md` | **PLAN escrito (esta auditoría)** — Fase 1 de 2: la alineación completa de `user` al patrón de módulos reutilizables (Config.IDs/Events, MountOps, codec agnóstico) quedó en cola como `tinywasm/user/docs/PLAN_MODULO_REUTILIZABLE.md` (`v0.1.0`, fila B6b del master plan), gated hasta cerrar esta fase |
| 🔴 2 | Borrar `replace` + carpetas `unixid/` (ya) y `local_orm/` (tras acción 1) | `mjosefa-cms` | Comentado en PR #3 |
| 🟡 3 | Críticas #1/#4/#5 del PR con sus docs sincronizados (§3, §6) | `mjosefa-cms` | En curso (agente cloud) |
| 🟡 4 | Rectificar master plan: filas B3/B5, filas nuevas B6a/B6b (`user`) | `app-releases/docs/REUSABLE_MODULES_MASTER_PLAN.md` | **Hecho (2026-07-17, esta auditoría)** |
| 🟢 5 | Alcance restante de B5: `events.Fanout`, adaptador `Subscriber` wasm, criterios de seguridad del stream (§4) | `tinywasm/events`, `tinywasm/sse`, plan SSE de la app | Pendiente (plan futuro) |
| 🟢 6 | Test anti-drift módulo↔recurso RBAC + job de integración del ecosistema (§5) | `mjosefa-cms/tests/`, CI de `app-releases` | Pendiente |
