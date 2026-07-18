# MASTER PLAN — Módulos reutilizables: acoplados solo a contratos tipados

Hub de coordinación multi-repo. Cierra el acoplamiento que impide reutilizar y **testear** un
módulo de dominio: hoy un módulo (`veltylabs/modules/*`) depende de **implementaciones** de
transporte (`tinywasm/mcp`), de codificación (`tinywasm/json`) y de generación de id
(`tinywasm/unixid`), y **no declara su propia vista** (vive aguas abajo, en el app, acoplada a
`tinywasm/layout`). Un módulo así no es una pieza de lego: no se puede montar en otro transporte,
ni testear en su propio repo.

> Dispatch: 2026-07-15 · **Estado (rectificado 2026-07-17, auditoría [`AUDITORIA_ARQUITECTURA_MJOSEFA_CMS.md`](AUDITORIA_ARQUITECTURA_MJOSEFA_CMS.md)): 🟢 Fase A COMPLETA — Fase B 4.5/5 (mcp, server, layout, unixid ✅; sse: `Publisher` ✅ publicado, resta lado wasm) — Fase C (piloto) mergeada pero rota por drift de API (`orm.DB.CreateTable`, ver §2b) — 🔴 B6 nueva: `tinywasm/user` roto por el MISMO drift, parcheado con forks locales en `mjosefa-cms` (anti-patrón §3/§6) — Fase E: docs en rectificación desde 2026-07-17, código gated**
> Doctrina: [`CONSTRUCTION_HARNESS.md`](CONSTRUCTION_HARNESS.md).
> Antecedentes: [`ROUTER_CONFORMANCE_MASTER_PLAN.md`](ROUTER_CONFORMANCE_MASTER_PLAN.md)
> (el patrón `conformance` que aquí replicamos para la UI),
> [`CRUD_HARNESS_MASTER_PLAN.md`](CRUD_HARNESS_MASTER_PLAN.md) (la vista CRUD via `layout`).

---

## 1. Estado y plan por pieza

Las piezas de la **compuerta** (Fase A) definen contrato y se implementaron/publicaron
**directamente** (sin `docs/PLAN.md` + CodeJob) — cambios pequeños que desbloquean todo lo demás.
Los adaptadores (Fase B) y los módulos (Fases C–E) **sí** se despachan vía CodeJob, uno por repo
(§6). Cross-repo → los enlaces de plan son URLs de GitHub (el agente ejecutor solo tiene su repo);
cada plan es autocontenido, con el contrato completo inline.

| Fase | Repo | Qué cambia / Plan | ¿Rompe API? | Estado |
|---|---|---|---|---|
| A1 | `tinywasm/model` | Implementado directo — `IDGenerator{NewID() string}` en `interface.go` | No (aditivo) | ✅ `v0.0.15` |
| A2 | `tinywasm/router` | Implementado directo — `Route.Accepts`, `Context.Decode/Encode` + 4 cláusulas nuevas en `router/conformance` + `router/mock` al día. Su antiguo `docs/PLAN.md` (conformance) se retiró: contenido ya implementado, pasó a `README.md` | No en consumidores; **sí en implementadores** (métodos nuevos) | ✅ `v0.1.12` |
| A2b | `tinywasm/router` | Implementado directo — `Caller.Call` gana un destino tipado (`into model.Decodable`), simétrico a `Context.Decode`. No estaba en el boceto original — lo exigió construir `view` sin `json` | Rompe implementadores de `Caller` (`mcp.NewCaller`) y llamantes con la firma vieja de 3 argumentos | ✅ `v0.1.13` |
| A2c | `tinywasm/router` | Implementado directo — `Op` **sale** de la interfaz gorda `Router` y pasa a `router.OpRegistry` (interfaz propia, un método); nace `router.OpModule{ModelName(); MountOps(OpRegistry)}`. Motivo: obligaba a un transporte que solo cosecha operaciones (mcp) a fingir ser un router HTTP (`Get/Post` con `panic`). `router/mock`+`conformance`+`README` al día | **Sí en implementadores** que hubieran añadido `Op` a `Router` (solo `mock`, ningún httpd) | ✅ `v0.1.14`, consumido en verde por B1/B2/B4 |
| A3 | `tinywasm/view` (**repo nuevo**, `gonew`) | Implementado y publicado — `view.New(caller, record, listOp, newList, project, opts...)` + `Presenter` + `view/mock` + `view/conformance`. Forma final distinta del boceto original (no existe un tipo `Descriptor`) | N/A (nuevo) | ✅ `v0.1.0` |
| A4 | `tinywasm/events` (**repo nuevo**, `gonew`) | Implementado directo — `Publisher`/`Subscriber`/`Broker`/`Event`, `events/mock.Broker`, `events/conformance` (5 cláusulas), test consumer-shaped | N/A (nuevo) | ✅ `v0.0.2` |
| B1 | `tinywasm/mcp` | [`mcp/docs/PLAN.md`](https://github.com/tinywasm/mcp/blob/main/docs/PLAN.md) — bump a `router@v0.1.14`; `HarvestOps(...OpModule) ToolProvider` con un `opRegistry` que implementa `router.OpRegistry` (un método, sin panics) + `opContext` (bridge `router.Context`↔mcp); `mcpCaller.Call` migra a la firma tipada; `NewCaller` intacto. Sin `mcpd` | **Sí** — módulos dejan de implementar `Tools() []mcp.Tool` y pasan a `MountOps` | ✅ despachado y verificado, `v0.2.0` |
| B2 | `tinywasm/server` | `server/docs/PLAN.md` — bump a `router@v0.1.14`; httpd añade **solo** `Context.Decode/Encode` a su `httpContext`. Se cae la proyección `/op` (mcp es el único transporte de ops) | No en consumidores; método nuevo en context | ✅ despachado y verificado, `v0.2.32` |
| B3 | `tinywasm/unixid` | Renombre `GetNewID → NewID()` + `SetNewID(*string)` tipado (commit `e83f7b5`) — satisface `model.IDGenerator` | Sí (renombre) | ✅ **publicado `v0.2.24`** (verificado 2026-07-17: el tag contiene el renombre; la fila anterior estaba desactualizada) |
| B4 | `tinywasm/layout` | `layout/docs/PLAN.md` — `crudview` deja de ser el motor CRUD y pasa a ser **solo el renderer**: `Config` pierde `ListOp`/`SaveOp`/`DeleteOp`/`Args`/`Decode`/`Fill`/`Caller`/`Record`, absorbidos por el `view.Presenter` que el módulo ya construye vía `view.New(...)`. Los tres call-sites de `Caller.Call` en `crudview` desaparecen (no se migran, se eliminan) | **Sí** — `crudview.Config` → `Config{ParentID, Presenter}` | ✅ despachado y verificado, `v0.0.14` |
| B5 | `tinywasm/sse` | Mitad servidor: `sse.Publisher` adapta `*SSEServer` a `events.Publisher` (commit `a280fef`) — ✅ **publicado `v0.1.1`**. Resta (alcance rectificado, ver auditoría §4): adaptador `events.Subscriber` del lado wasm sobre `SSEClient.OnMessage`, `events.Fanout` (combinador in-proc + sse, aditivo en `events`), y en el plan del app: montar `StreamHandler()` vía `router.Stream` con canal resuelto desde `ctx.UserID()` — nunca elegido por el cliente | No (aditivo) | 🟡 servidor ✅ `v0.1.1`; lado wasm + Fanout sin plan escrito |
| C (piloto) | `veltylabs/item_catalog` | Implementó `router.OpModule` (`MountOps(OpRegistry)` con `r.Op(...).Requires().Accepts()`), las 11 ops existentes; vista via `view.New(...)`; inyecta `model.IDGenerator` y `events.Publisher` tipado; borra `EventPublisher`+`UIAdapter`; sin mcp/json/unixid | **Sí** | ✅ mergeado a `main` (`c6f90d0`), pero 🔴 **roto en este momento**: `orm.DB.CreateTable` fue removido aguas arriba (schema migration se movió a `tinywasm/ddl`, ver §2b) y el `New()` del módulo aún lo llama. `docs/PLAN.md` (ya despachado) fue borrado por convención; hay un nuevo `docs/PLAN.md` describiendo solo el fix pendiente (adoptar `ddl.CreateTable`) + cambios locales sin commitear (bump de deps, `interfaces.go`) |
| B6a | `tinywasm/user` | [`user/docs/PLAN.md`](https://github.com/tinywasm/user/blob/main/docs/PLAN.md) — fix del MISMO drift de la fila C: `authority/` llama `orm.DB.CreateTable` (removido) y `orm.ErrNoRows` (renombrado) → `ddl.TopologicalSort`+`ddl.Sync` con type-assert `ddl.Compiler`, `orm.ErrNotFound`, y cierre de `ddlc.FieldExt → model.FieldExt`. Urgente: `mjosefa-cms` lo parchea hoy con forks locales (`local_orm` con `CreateTable` **no-op** que desactiva en silencio el esquema de auth — hallazgo crítico de la auditoría) | No en consumidores | ☐ plan escrito 2026-07-17, sin despachar — **desbloquea** que `mjosefa-cms` borre sus `replace` |
| B6b | `tinywasm/user` | [`user/docs/PLAN_MODULO_REUTILIZABLE.md`](https://github.com/tinywasm/user/blob/main/docs/PLAN_MODULO_REUTILIZABLE.md) — alineación al patrón de este master plan (`v0.1.0`): `Config.IDs model.IDGenerator` (hoy construye `unixid` en 5 archivos), `Config.Events events.Publisher` + `TopicSecurity` (reemplaza el callback ad-hoc `OnSecurityEvent` — el caso exacto que motivó `events`), `Tools() []mcp.Tool` → `router.OpModule.MountOps` (op `me` con `.Authenticated()`), codec vía `Context.Decode/Encode`. Excepciones documentadas: `MountAPI` (flujos cookie = SU transporte), `jwt`/`x/crypto` (dominio auth), `fetch`+`json` confinados a `oauth.go` (protocolo externo OAuth). La vista sigue el principio "views belong to the consumer": el módulo exporta modelos tipados con `Kind`; presenter admin = plan futuro | **Sí** (Config y transporte) | ☐ plan escrito 2026-07-17, **gated** hasta cerrar B6a y que `mjosefa-cms` quede verde sin `replace` |
| D | `veltylabs/mjosefa-cms` | `mjosefa-cms/docs/PLAN.md` (rectificar) — el composition root declara explícito qué `OpModule`s se cosechan: `mcp.HarvestOps(items…)`; inyecta `unixid` como `IDGenerator`, `httpd` como `Router` real bajo `mcp`, `layout/crudview` como renderer, broker `events` (in-proc + `sse`) | Consumidor | 🟡 plan rectificado, sin despachar |
| E | `agent_switch`, `appointment_booking`, `business_hours`, `clinical_encounter`, `provider_payouts`, `work_schedule` | Implementación **gated** (§5): no se despacha código hasta que C+D cierren en verde. **Los `docs/PLAN.md`/`ARCHITECTURE.md`/`AGENTS.md` sí se rectifican ahora** (2026-07-17, sesión doc-only) — el gate bloquea CodeJob dispatch, no la corrección de documentos que ya están desalineados con el patrón validado por C | **Sí** c/u | 🟡 docs en rectificación, código no iniciado (intencional) |

---

## 2. La solución — contratos y puertos, cero implementaciones concretas en el módulo

Un módulo depende de `tinywasm/model` + `tinywasm/router` + `tinywasm/view` + `tinywasm/events` —
**más** `tinywasm/orm` (y, transitivamente, `tinywasm/storage`) para persistencia y
`tinywasm/ddl` para su propio schema. Todo lo demás (el generador de id concreto, el transporte
concreto, el codec concreto, el renderer concreto, el broker concreto, el *backend* de storage
concreto) se inyecta.

### 2b. `orm`/`storage`/`ddl` son puertos, no implementaciones — por qué están en la lista

`orm` (query builder) y `ddl` (schema/DDL en runtime) son capas ergonómicas **agnósticas** sobre
`storage.Conn` — el equivalente de `database/sql` (`orm`) y de una migración runtime (`ddl`) sobre
`database/sql/driver` (`storage`). Un módulo que llama `orm.New(conn)`/`m.db.Create(...)` o
`ddl.CreateTable(conn, ...)` no sabe ni le importa si `conn` es `sqlt`, `postgres`, `mem` o un futuro
`indexdb` — eso lo decide quien construye el `storage.Conn` e inyecta el `*orm.DB` resultante en
`New(db, deps)`. Eso es categóricamente distinto de importar `tinywasm/sqlite` (nombra un driver) o
`tinywasm/mcp` (nombra un transporte) directamente. La lista de imports "de dominio" prohibidos en
código no-test (§5) es sobre **implementaciones concretas** — `mcp`/`json`/`unixid`/`sqlite`/
`sqlt`/`postgres`/`layout` — no sobre `orm`/`storage`/`ddl`, que son puertos como `router`/`view`.

**`orm.DB.CreateTable` fue removido** (la migración de schema se extrajo a `tinywasm/ddl`, sibling de
`orm`, ninguno depende del otro). `ddl.New(conn, ddlCompiler)` toma **dos** argumentos —
`ddl.Compiler` es una capacidad opcional que solo implementan backends SQL (`sqlt`, `postgres`);
`storage/mem` (el backend que los propios tests del módulo usan) no la implementa — crea tablas de
forma perezosa en el primer `Exec` y no necesita DDL. El módulo hace un *type assertion* sobre
`db.RawConn()` en vez de asumir la capacidad (mismo idioma que `storage.TxExecutor` ya usa para
transacciones opcionales): si el backend implementa `ddl.Compiler`, migra; si no (`mem`, en tests),
es un no-op. Sigue siendo el módulo quien migra su esquema, solo cambia el paquete que lo hace y se
vuelve condicional a la capacidad del backend (ver `veltylabs/item_catalog`, roto por este cambio de
API justo ahora — fila C de §1).

## 3. Lo que NO hacemos (y por qué)

- ❌ **Envolver `mcp`/`unixid`/`layout` en el módulo** para "adaptarlos". Un wrapper que parcha es un
  fork con nombre amable. El contrato se arregla aguas arriba y el módulo lo consume.
- ❌ **Declarar interfaces locales** que intersecten tipos de `model`/`router`. Si falta un contrato
  en una frontera, el defecto está aguas arriba — se nombra ahí.
- ❌ **Dejar la vista en el app.** Es justo lo que impide testear el módulo. La vista se declara en el
  módulo; el app solo acopla el renderer concreto.
- ❌ **Meter la vista en `layout`** como contrato. Acoplaría los módulos a esa tech de UI.

## 4. Grafo de dependencias

```mermaid
flowchart TB
    subgraph A["Fase A — COMPUERTA (contratos)"]
        A1["model: IDGenerator"]
        A2["router: OpRegistry/OpModule + Route.Accepts + Context.Decode/Encode + Caller typed"]
        A3["view (gonew): Presenter + conformance"]
        A4["events (gonew): Publisher/Subscriber + conformance"]
    end
    subgraph B["Fase B — adaptadores (paralelo tras A)"]
        B1["mcp: HarvestOps(OpModule)→tools (OpRegistry, sin panics) + Caller typed"]
        B2["server/httpd: solo Context.Decode/Encode (sin proyección /op)"]
        B3["unixid: satisface IDGenerator"]
        B4["layout/crudview: implementa view"]
        B5["sse: implementa events.Publisher"]
    end
    C["Fase C — PILOTO: item_catalog\n(solo model+router+view+events)"]
    D["Fase D — app mjosefa-cms\nensambla e inyecta"]
    E["Fase E — fan-out: los otros 7 módulos"]

    A1 --> A2 --> A3
    A1 --> A4
    A --> B1 & B2 & B3 & B4 & B5
    B --> C --> D --> E
```

- **A es la compuerta.** `model` primero (todos lo importan); luego `router` y `events` (importan
  `model`); luego `view` (importa `model`+`router`).
- **B corre en paralelo** tras A (repos distintos).
- **C (piloto) valida el patrón end-to-end en UN módulo** antes de tocar los demás. Si un contrato
  está mal, se descubre en 1, no en 8.
- **D** integra en el app (una sola pasada; doctrina: el app siempre al final).
- **E** replica el patrón a los 7 restantes solo cuando el piloto está en verde.

## 5. Criterio de aceptación del piloto (Fase C, la validación del patrón)

Antes de disparar la Fase E, en `veltylabs/item_catalog`:

- `grep -rn "tinywasm/mcp\|tinywasm/json\|tinywasm/unixid\|tinywasm/sqlite\|tinywasm/sqlt\|tinywasm/postgres\|tinywasm/layout" .`
  **vacío en todo el repo, tests incluidos** (ver §2b: `orm`/`storage`/`ddl` sí son imports
  legítimos — no forman parte de esta lista negra; lo prohibido son implementaciones concretas). Un
  módulo no trae un driver de DB ni para sus propios tests: usa `storage/mem` (`orm.New(mem.New())`).
- El módulo implementa `router.OpModule` (`MountOps(r router.OpRegistry)` con
  `r.Op(...).Requires().Accepts()`) — **no** `MountAPI(Router)` — y construye su vista con
  `view.New(caller, record, listOp, newList, project, opts...)` (no existe un tipo `Descriptor`).
  **No** declara ningún `EventPublisher` propio: usa `events.Publisher`. Compila
  `var _ router.OpModule = (*Module)(nil)`.
- El id y el publicador entran por `Deps{ IDs model.IDGenerator; Publisher events.Publisher }` — el
  módulo nunca construye un generador ni un broker.
- El módulo migra su propio schema en `New()` vía `github.com/tinywasm/ddl` (`ddl.CreateTable`/
  `ddl.Sync` sobre `db.RawConn()`), no vía `orm.DB.CreateTable` (removido, ver §2b).
- El módulo carga un `AGENTS.md` (copia del canónico en `veltylabs/modules/AGENTS.md` + su sección
  "Domain-specific notes") en su raíz.
- Un test **en el repo del módulo** ejerce el `view.Presenter` resultante con un `router.Caller`
  falso (o el `conformance.FakeCaller` de `view/conformance`, codec-free): prueba
  lista/seleccionar/guardar/eliminar sin app, sin navegador real (target no-wasm) y con DOM real
  (target wasm). Y un test ejerce `MountOps` contra `router/mock` (que satisface `OpRegistry`): op
  enrutada + RBAC, y la publicación de eventos contra un `events.Publisher` falso.
- `gotest ./...` verde en ambos targets.

Si esos tests son incómodos de escribir, el contrato (Fase A/B) tiene un defecto: **se corrige aguas
arriba y se vuelve a publicar**, nunca se parcha en el módulo.

## 6. ⚠️ Compuertas y advertencias

- **Orden ejecutado**: A1 → A2 (incluida A2b) → A4 → A3, todo publicado en ese orden (import chain
  `model ← router`, `model ← events`, `model`+`router ← view`; A3/A4 podían ir en paralelo tras
  A1/A2).
- **`router` rompió tres veces** (`v0.1.12`/`v0.1.13`/`v0.1.14`, detalle en la tabla §1) — cada ola la
  cierra un plan B distinto: B2 (`server/httpd`) absorbe `Context.Decode/Encode`; B1 (`mcp`) absorbe
  el `Caller` tipado y `HarvestOps`. `httpd`/`sse` nunca implementaron `Caller`, sin impacto ahí.
- **B1 (`mcp`, `v0.2.0`) y B4 (`layout`, `v0.0.14`) rompen API de sus consumidores actuales**
  (`mjosefa-cms`, sus tests). Ambos ya están publicados — `mjosefa-cms` verá rojo hasta la Fase D. Es
  el hallazgo, no una regresión: prueba que el acoplamiento existía.
- **Ni `mcpd` ni partir `mcp`**: `mcp` ya depende solo de la abstracción `router` (no importa
  `net/http`/`httpd`); el consumidor ya inyecta `httpd`. Un paquete "mcp-contrato" no tendría
  consumidor (los módulos usan `router`; el composition root usa el concreto). Se descartó.
- **Publicación:** doctrina de dispatch — cada repo se publica cuando su plan cierra; el app
  (`mjosefa-cms`) siempre al final, en una sola pasada de integración. No tocar repos ya publicados
  sin pedirlo. B1/B2/B4 despachados, verificados y publicados; B3 (`unixid v0.2.24`) y la mitad
  servidor de B5 (`sse v0.1.1`, `sse.Publisher`) resultaron ya publicados (verificado 2026-07-17 —
  las filas estaban desactualizadas). Pendientes de decisión de dispatch: B6a (`user`, urgente —
  desbloquea los `replace` de `mjosefa-cms`), B6b (`user` alineación, gated tras B6a) y el resto de
  B5 (lado wasm + `events.Fanout`).
- **No hay carpetas `internal/`** en ninguna fase: son señal de fork en vez de contribución aguas
  arriba. `veltylabs/work_schedule` viola esto hoy con `replace github.com/tinywasm/mcp =>
  ../../../tinywasm/mcp` en su `go.mod` — se cierra como parte de su rectificación de Fase E, no como
  excepción.
- **`AGENTS.md` es obligatorio por módulo** (decisión 2026-07-17): un canónico vive en
  `veltylabs/modules/AGENTS.md` — whitelist/blacklist de imports, el límite exacto con la stdlib
  (`tinywasm/fmt` reemplaza `fmt`/`strings`/`strconv`/`errors`; nada más queda prohibido salvo lo que
  duplique un contrato de esta lista), no-map/no-reflect, testing contra `storage/mem` (nunca un
  driver concreto, tests incluidos), publicación vía CodeJob/PR (no `gopush`). Cada módulo copia ese
  archivo verbatim y llena solo su sección "Domain-specific notes". Los `docs/PLAN.md` de cada módulo
  referencian ese `AGENTS.md` en vez de re-inlinear el contrato — evita que 6 planes carguen 6 copias
  ligeramente distintas (y crecientemente obsoletas) del mismo contrato `model.Kind`, que es lo que
  había ocurrido antes de esta rectificación.
- **`clinical_encounterOld` no se toca.** Directorio hermano de `clinical_encounter`, código legado
  sin importadores; fuera de alcance de toda esta iniciativa (ya lo señalaba
  `clinical_encounter/docs/PLAN_MODEL_MIGRATION.md` antes de esta rectificación).
