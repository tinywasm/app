# MASTER PLAN — Puerto de almacenamiento (`tinywasm/storage`) + conformance de backends

Hub de coordinación multi-repo. **Sucesor de `DDL_DML_SPLIT_MASTER_PLAN.md` (borrado — obsoleto).**
Ya no separamos DML/DDL dentro de `orm`; extraemos **el contrato completo** (interfaces + tipos de
valor + conformance + mock) a un puerto nuevo, `tinywasm/storage`, y tanto `orm` (ergonomía DML) como `ddl`
(runtime DDL) pasan a ser consumidores de ese puerto, no dueños de él.

> Estado: 🟡 **`storage` planificado, sin implementar. Resto de piezas: planes corregidos, sin despachar.**
> Doctrina: [`CONSTRUCTION_HARNESS.md`](CONSTRUCTION_HARNESS.md).
> Razonamiento completo (por qué este diseño, alternativas descartadas, precedente
> `database/sql`/`database/sql/driver`): [`DB_PORT_PROPOSAL.md`](DB_PORT_PROPOSAL.md) — léelo antes de
> tocar cualquier repo de esta lista si algo aquí no cuadra; este documento es el rastreador, aquel es
> el porqué.
> Antecedente del patrón `conformance`: [`ROUTER_CONFORMANCE_MASTER_PLAN.md`](ROUTER_CONFORMANCE_MASTER_PLAN.md).

---

## 0. Por qué existe este documento (y qué reemplaza)

`DDL_DML_SPLIT_MASTER_PLAN.md` coordinaba separar DML/DDL **dentro de `orm`**, dejando `orm` como
dueño del contrato de almacenamiento (`Executor`/`Compiler`/`Query`/`Condition`/`Plan`) y creando
`tinywasm/ddl` como hermano para el esquema. Esa fase **sí se ejecutó**: `orm` quedó DML-puro (breaking
directo, sin ventana de gracia).

Pero durante esa ejecución surgió una pregunta más de fondo (ver DB_PORT_PROPOSAL.md §1): si `orm`
sigue siendo dueño del contrato, todo backend (`postgres`/`sqlt`/`indexdb`) sigue dependiendo del
ORM completo solo para cumplir unas interfaces — el mismo problema que el split DDL/DML resolvió para
el esquema, sin resolver para el contrato en sí. La respuesta, justificada a fondo en el proposal, es
extraer **el contrato mismo** (no solo su mitad DDL) a un puerto neutral: `tinywasm/storage`, exactamente
como `database/sql/driver` en la stdlib de Go. `orm` pasa a ser una capa ergonómica **opcional**
encima de `storage` (igual que `sql.DB` es opcional sobre `driver`); `ddl` pasa a depender de `storage`, no de
`orm`.

Este documento reemplaza al plan anterior como hub de coordinación. **No revivas el DDL_DML_SPLIT —
esas piezas (`ddl` en concreto) ya fueron re-corregidas para depender de `storage`, ver §3.**

## 1. Arquitectura objetivo

```
tinywasm/model  (Field, Definition, FieldWriter/Reader, ReadValues)                         [fmt]
      │
      ▼
tinywasm/storage     — EL PUERTO: Executor/Compiler/Conn/Scanner/Rows/TxExecutor/TxBoundExecutor,
      │           Query/Action/Order/Condition/Plan, ErrNoRows, ScanAny,
      │           storage/conformance (contrato DML ejecutable sobre Query cruda, sin builder),
      │           storage/mem (backend en memoria de referencia), storage/mock (recorders)             [model, fmt]
      │
      ├──────────────────┬──────────────────┬───────────────────────────────┐
      ▼                  ▼                  ▼                               ▼
tinywasm/orm        tinywasm/ddl      backends SQL: sqlt, postgres     indexdb (WASM)
(ergonomía DML,      (runtime DDL:     implementan storage.Conn (DML) +     implementan
 OPCIONAL sobre      CreateTable/      ddl.Compiler (DDL);             SOLO storage.Conn
 storage — no forma       DropTable/Sync,   prueban storage/conformance          (sin DDL SQL:
 parte del           ddl.Compiler,     Y ddl/conformance                stores por
 contrato)           ddl/conformance)                                   structTables)
 orm.DB, QB,         ddl.New(conn      DEP: [storage, ddl]                   DEP: [storage]
 Where/ReadAll/       storage.Conn,
 Create/Update/       ddlCompiler)
 Delete
 DEP: [storage]           DEP: [storage, ddlc]
```

Reglas de dependencia (una sola dirección, sin ciclos):

- `storage` depende **solo** de `model` + `fmt`. No conoce a `orm`, `ddl`, ni a ningún backend.
- `orm` → `storage`. `ddl` → `storage` + `ddlc`. **`orm` y `ddl` no se conocen entre sí.**
- Backends → `storage` (+ `ddl` si hacen DDL SQL). **Nunca → `orm`.** Este es el objetivo central de todo
  el rediseño (ver DB_PORT_PROPOSAL.md §2).
- Consumidores finales (apps/módulos hoja) → `orm` (ergonomía) **o** `storage` directamente si escriben
  infraestructura (el propio `ddl`, o un módulo hoja que arma una `Query` a mano es legítimo, no una
  fuga de abstracción — ver DB_PORT_PROPOSAL.md §6.8).

## 2. El contrato de conformance — ahora sobre `Query` cruda, no sobre el builder

Diferencia importante respecto al plan anterior: `storage/conformance` (antes `orm/conformance`) ya **no**
pasa por ningún query builder — construye valores `storage.Query{...}` directamente y llama
`Compile`+`Exec`/`Query`+`Scan`. Esto prueba exactamente la obligación de un backend (compilar +
ejecutar), independiente de cualquier capa ergonómica. El builder de `orm` se prueba en `orm`, con los
recorders de `storage/mock`, no aquí. Ver `storage/docs/PLAN.md` §4 (razonamiento completo) y DB_PORT_PROPOSAL.md
§6.4.

`ddl/conformance` (esquema, solo backends SQL) no cambia de forma — sigue viviendo en `ddl`, sigue
probando `CreateTable`/`Sync`/`DropTable` por introspección.

## 3. Piezas y estado

Cross-repo: el agente ejecutor **solo tiene su repo**; cada `docs/PLAN.md` es autocontenido con el
contrato inline. Enlaces de plan = URLs de GitHub cuando cruzan repos.

| # | Repo | Qué cambia / Plan | ¿Rompe API? | Estado |
|---|---|---|---|---|
| 0 | `tinywasm/ddlc` | Sin cambios (CLI/codegen leaf, no runtime). | No | ☐ nota, sin tocar |
| 1 | `tinywasm/storage` | **repo NUEVO (`gonew`, ya creado)** — [`storage/docs/PLAN.md`](https://github.com/tinywasm/storage/blob/main/docs/PLAN.md). El puerto completo: contrato + `storage/conformance` (sobre `Query` cruda) + `storage/mem` + `storage/mock`. Deps: solo `model`+`fmt`. Isomórfico (wasm + TinyGo), cero `map`. | N/A (nuevo) | 🟡 plan escrito, **sin implementar** |
| 2 | `tinywasm/orm` | [`orm/docs/PLAN.md`](https://github.com/tinywasm/orm/blob/main/docs/PLAN.md) — pasa a ser **capa ergonómica opcional** sobre `storage`: `orm.DB`/`QB`/`Create`/`Update`/`Delete`/`Tx`, `orm.New(conn storage.Conn)` (1 arg). Pierde `Executor`/`Compiler`/`Query`/`Condition`/`Order`/`Plan`/`ErrNoRows`/`ScanAny` (se importan/re-exportan de `storage`). Pierde el registro `Open`/`Register` (eliminado, no trasladado — ver DB_PORT_PROPOSAL.md §6.6). Conserva `ErrNotFound` (traduce `storage.ErrNoRows`). | **Sí** (breaking) | 🟡 plan escrito, sin despachar |
| 3 | `tinywasm/ddl` | [`ddl/docs/PLAN.md`](https://github.com/tinywasm/ddl/blob/main/docs/PLAN.md) — **re-corregido**: `ddl.New(conn storage.Conn, ddlCompiler ddl.Compiler) *ddl.DB` (2 args — `conn` ya trae exec+compiler DML para el safe-drop de `Sync`, no hacen falta 3). `ddl.Stmt` gana `ColumnName string` para `OpDropColumn` (gap de diseño corregido en esta pasada). Deps: `storage`+`ddlc`, **cero `orm`**. | N/A (aún sin implementar) | 🟡 plan corregido, sin despachar |
| 4 | `tinywasm/sqlt` | [`sqlt/docs/PLAN.md`](https://github.com/tinywasm/sqlt/blob/main/docs/PLAN.md) — **re-corregido**: es el **compilador puro** (no abre conexiones) — implementa `storage.Compiler` (antes `orm.Compiler`) + `ddl.Compiler`; prueba `storage/conformance` (antes `orm/conformance`) + `ddl/conformance` con un `storage.Conn` de test local. Deps: `storage`+`ddl`+`ddlc`, **cero `orm`**. | Rompe ya (orm cambió) | 🟡 plan corregido, sin despachar |
| 4b | `tinywasm/sqlite` | **repo descubierto durante este rediseño — no estaba en la tabla original.** [`sqlite/docs/PLAN.md`](https://github.com/tinywasm/sqlite/blob/main/docs/PLAN.md) — el **adapter real** (`*sql.DB` de verdad, vía `modernc.org/sqlite`), consume el compilador de `sqlt`. `sqlite.Open(dsn) (storage.Conn, error)` — sin `init()`/`orm.Register`. `sqliteConn` implementa `storage.Conn`+`storage.TxExecutor`+`ddl.TableIntrospector`+`ddl.SchemaInspector`. Deps: `storage`+`ddl`+`sqlt`, **cero `orm`**. Depende de que `sqlt` (#4) esté publicado primero. | Rompe ya (orm cambió) | 🟡 plan escrito, sin despachar |
| 5 | `tinywasm/postgres` | [`postgres/docs/PLAN.md`](https://github.com/tinywasm/postgres/blob/main/docs/PLAN.md) — backend completo en un repo (compilador+adapter juntos, a diferencia de sqlt/sqlite): `storage.Conn`+`ddl.Compiler`; `postgres.Open(dsn) (storage.Conn, error)` sin registro; prueba `storage/conformance`+`ddl/conformance`. | Rompe ya | 🟡 plan corregido, sin despachar |
| 6 | `tinywasm/indexdb` | [`indexdb/docs/PLAN.md`](https://github.com/tinywasm/indexdb/blob/main/docs/PLAN.md) — **re-corregido**: `indexdb.New(...) storage.Conn` (antes `*orm.DB`); prueba **solo** `storage/conformance` (nunca `ddl/conformance` — sin DDL SQL). Deps: `storage`, **cero `orm`**. | Rompe ya | 🟡 plan corregido, sin despachar |
| 7 | consumidores | **Ola posterior.** El registro DSN no existe — construyen explícito: `conn, _ := sqlite.Open(dsn); d := orm.New(conn)` (o `postgres.Open`/`indexdb.New` según el backend). Repos: `app`, `ormc`, `user/authority`, `goflare-demo`, `veltylabs/{item_catalog,appointment_booking,business_hours,agent_switch}`. Ya rotos por el break anterior de `orm`; migran directo al patrón nuevo (no hay una migración intermedia que valga la pena). | Consumidor | ☐ pendiente, uno por repo |

## 4. Secuencia de despacho

Regla: **`storage` primero — es la base de todo lo demás y no depende de nada del ecosistema salvo
`model`+`fmt`.** Una vez publicado, `orm` y `ddl` se despachan **en paralelo** (no se conocen entre
sí, ambos solo dependen de `storage`). Los backends SQL (`sqlt`/`postgres`) esperan a `ddl` publicado
(porque implementan `ddl.Compiler`); `indexdb` no espera a nadie más que `storage` (no hace DDL).

1. **`tinywasm/storage` (#1).** Bloquea todo lo demás. Plan ya escrito y autocontenido — despachable ya.
2. **En paralelo, tras `storage@v0.0.1`:**
   - **`tinywasm/orm` (#2)** — reescribe su capa ergonómica contra `storage`.
   - **`tinywasm/ddl` (#3)** — implementa su runtime contra `storage` (no espera a `orm`).
   - **`tinywasm/indexdb` (#6)** — implementa `storage.Conn`, prueba `storage/conformance` (no espera a `ddl`,
     no hace DDL).
3. **Tras `ddl@v0.0.1`:**
   - **`tinywasm/sqlt` (#4)** — el compilador, implementa `storage.Compiler`+`ddl.Compiler`, prueba ambos
     conformances con un `storage.Conn` de test local.
   - **`tinywasm/postgres` (#5)** — backend completo en un repo, implementa `storage.Conn`+`ddl.Compiler`
     directamente, prueba ambos conformances.
4. **Tras `sqlt@v0.1.0`:** **`tinywasm/sqlite` (#4b)** — el adapter real, consume el compilador de
   `sqlt` + abre conexiones de verdad. No puede despacharse antes que `sqlt` (lo importa).
5. **Último — consumidores (#7), uno por repo.** `app` al final (memoria
   [[tinywasm-dispatch-doctrine]]).

> **Por qué no expand/contract.** Mismo razonamiento que el split DDL/DML anterior: los backends ya
> están rotos por ese break previo y deben reescribirse de todos modos. Extraer `storage` ahora significa
> que se reescriben **una vez** contra el puerto final, no dos (una vez contra `orm`, otra después
> contra `storage`). Ver DB_PORT_PROPOSAL.md §7 ("la ventana de oro").

## 5. Doctrina de despacho

- `gotest` para tests (`gotest -tinygo` es **obligatorio** en `storage`, isomórfico puro); `gopush 'msg'`
  para publicar (tag automático). Prereq del agente:
  `go install github.com/tinywasm/devflow/cmd/gotest@latest`.
- Planes cross-repo autocontenidos, contrato inline; sin enlaces markdown cross-repo dentro del
  código.
- **Sin `map[K]V`** en `storage`, `orm`, `ddl`, ni en los backends — ver `AGENTS.md` de cada repo.
- No tocar repos ya publicados sin pedirlo; `app` siempre al final ([[tinywasm-dispatch-doctrine]]).
- Cada `docs/PLAN.md` terminado se **borra** tras publicar; su diseño duradero pasa a
  `ARCHITECTURE.md`/`README.md` del repo ([[codejob-plan-lifecycle]]).

## 6. Qué NO cambia

- `tinywasm/ddlc` (CLI/codegen leaf, build-time) no se toca.
- El modelo `Widget` de conformance (id TEXT PK, name TEXT, qty INT, active BOOL) es el mismo en
  `storage/conformance` que el que existía en `orm/conformance` — no lo rediseñes al portarlo.
- Las 12 cláusulas DML y las 4 cláusulas DDL no cambian de intención, solo de mecanismo (Query cruda
  en vez de builder para las DML).
