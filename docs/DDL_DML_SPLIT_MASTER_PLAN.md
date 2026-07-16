# MASTER PLAN — Split DDL/DML + conformance de backends de almacenamiento

Hub de coordinación multi-repo. Separa dos responsabilidades que hoy viven mezcladas en
`tinywasm/orm` y crea el **contrato ejecutable** (conformance) que cada backend debe cumplir, para que
todos —incluido `indexdb`— se comporten igual y sean intercambiables.

> Dispatch: 2026-07-16 · **Estado: 🟡 planes escritos, sin despachar**
> Doctrina: [`CONSTRUCTION_HARNESS.md`](CONSTRUCTION_HARNESS.md).
> Antecedente del patrón: [`ROUTER_CONFORMANCE_MASTER_PLAN.md`](ROUTER_CONFORMANCE_MASTER_PLAN.md)
> (el `conformance` de `router` que aquí replicamos para almacenamiento).

---

## 1. El problema

`tinywasm/orm` mezcla hoy **dos** contratos en un mismo `*orm.DB`:

- **DML (operación de datos):** `Create`/`Update`/`Delete`/`Query(ReadOne/ReadAll)` — mover filas cuando
  la tabla **ya existe y está ok**.
- **DDL (esquema):** `CreateTable`/`DropTable`/`CreateDatabase`/`Sync`/`SyncSchema` + las `Action`
  DDL (`AddColumn`/`RenameColumn`/`DropColumn`) — crear/migrar el esquema.

Esa mezcla impide un `conformance` limpio de backend: `indexdb` **no** crea stores dinámicamente como
SQL (`CREATE TABLE` en caliente) — los declara por adelantado en `New(...structTables)` durante
`onUpgradeNeeded`. Si el contrato de backend incluye DDL, `indexdb` nunca puede conformar igual que
`sqlite`/`postgres`. Pero su **DML** es idéntico al de todos. La solución es separar:

- **`tinywasm/orm`** = runtime **DML**. Un backend conforma `orm/conformance` (datos).
- **`tinywasm/ddl`** (**repo NUEVO**, vía `gonew`) = runtime **DDL** — hermano de `orm`. Un backend SQL
  conforma `ddl/conformance` (esquema).
- **`tinywasm/ddlc`** = se queda como es: la **CLI/codegen leaf** de DDL (build-time). Genera el SQL DDL
  (`Exporter.ExportDDL`), ordena FKs (`TopologicalSort`), tiene su `tui`. **No** es runtime, **no**
  cambia en este master (solo se aclara su rol). Depende solo de `model`+`fmt`.

Resultado — **dos conformances, cada backend entra donde puede**:

| Backend | `orm/conformance` (DML) | `ddl/conformance` (DDL) |
|---|:--:|:--:|
| `orm/mock` (memoria) | ✅ | — (memoria, sin DDL SQL) |
| `sqlite` (vía `sqlt`) | ✅ | ✅ |
| `postgres` | ✅ | ✅ |
| `indexdb` (WASM) | ✅ | — (stores por `structTables`, no DDL SQL) |

## 2. Arquitectura objetivo

```
tinywasm/ddlc  (CLI/codegen leaf: Exporter, FieldExt, TopologicalSort, tui)   [model, fmt]
      ▲ genera SQL DDL (build-time)
      │
tinywasm/ddl   (runtime DDL: CreateTable/DropTable/Sync + ddl/conformance)    [orm(Executor), ddlc]
      │  implementado por sqlt/postgres        probado por: sqlite, postgres
      │
tinywasm/orm   (runtime DML: Create/Update/Delete/Query + orm/conformance + orm/mock)   [model]
         implementado por sqlt/postgres/indexdb/mock   probado por: mock, sqlite, postgres, indexdb
```

- `orm` sigue **sin** depender de `ddl`/`ddlc` (leaf DML runtime). `ddl` depende de `orm` (usa
  `orm.Executor`) y de `ddlc` (usa `Exporter`/`FieldExt`). Una sola dirección, sin ciclos.
- `sqlt`/`postgres` implementan **ambos**: `orm.Compiler` (DML) y el compilador DDL que `ddl` consume.
- `indexdb` y `orm/mock` implementan **solo** DML.

## 3. Piezas y estado

Cross-repo: el agente ejecutor **solo tiene su repo**; cada `docs/PLAN.md` es autocontenido con el
contrato inline. Enlaces de plan = URLs de GitHub cuando cruzan repos.

| # | Repo | Qué cambia / Plan | ¿Rompe API? | Estado |
|---|---|---|---|---|
| 0 | `tinywasm/ddlc` | **Nada de código.** Se aclara en su `README` que es la CLI/codegen leaf (no runtime). | No | ☐ nota |
| 1 | `tinywasm/orm` | [`orm/docs/PLAN.md`](https://github.com/tinywasm/orm/blob/main/docs/PLAN.md) — nace `orm/conformance` (**solo DML**) + `orm/mock` (recorders reubicados + `NewDB()` en memoria funcional que auto-crea tablas al insertar). **No** toca todavía los métodos DDL de `orm.DB` (eso es la ola de contracción, #6). | No (aditivo) | 🟡 escrito |
| 2 | `tinywasm/ddl` | **repo NUEVO (`gonew`)** — [`DDL_REPO_PLAN.md`](DDL_REPO_PLAN.md) — runtime DDL: `ddl.DB` (absorbe `CreateTable`/`DropTable`/`CreateDatabase`/`Sync`/`SyncSchema` + algoritmo de `orm/sync.go`), `ddl.Compiler` (la mitad DDL del actual `orm.Compiler`), `ddl/conformance` (esquema; solo SQL). | N/A (nuevo) | 🟡 escrito |
| 3 | `tinywasm/sqlt` | [`sqlt/docs/PLAN.md`](https://github.com/tinywasm/sqlt/blob/main/docs/PLAN.md) — prueba `orm/conformance` (DML, con executor `database/sql` inline) **y** `ddl/conformance` (DDL); su compilador implementa `ddl.Compiler` (mueve sus ramas DDL de `translate.go`). | Aditivo hasta #6 | 🟡 escrito |
| 4 | `tinywasm/postgres` | [`postgres/docs/PLAN.md`](https://github.com/tinywasm/postgres/blob/main/docs/PLAN.md) — prueba `orm/conformance` (DML, skip sin `POSTGRES_DSN`) **y** `ddl/conformance` (DDL); su `Compiler` implementa `ddl.Compiler`. | Aditivo hasta #6 | 🟡 escrito |
| 5 | `tinywasm/indexdb` | [`indexdb/docs/PLAN.md`](https://github.com/tinywasm/indexdb/blob/main/docs/PLAN.md) — prueba **solo** `orm/conformance` (DML, WASM). Ya no hay fricción DDL. | Aditivo | 🟡 escrito |
| 6 | `tinywasm/orm` (contracción) | **Ola posterior, se detalla al despachar.** Quita DDL de `orm.DB` (`CreateTable`/`DropTable`/`CreateDatabase`/`Sync`/`SyncSchema`, `Action` DDL, `sync.go`, `validate` DDL) una vez que TODOS los consumidores migraron a `ddl` (#7). | **Sí** (breaking) | ☐ pendiente |
| 7 | consumidores | **Ola posterior.** Migran `db.CreateTable(m)`/`db.Sync(...)` → `ddl.New(exec,gen).CreateTable/Sync`. Repos: `app`, `ormc`, `user/authority`, `goflare-demo`, `veltylabs/{item_catalog,appointment_booking,business_hours,agent_switch}`. | Consumidor | ☐ pendiente |

## 4. Secuencia (expand → migrate → contract)

Regla de oro: **nada rompe hasta que todo lo que dependía ya migró.** Se añade lo nuevo (aditivo),
se migran consumidores, y **al final** se quita lo viejo de `orm`.

- **Ola A — Fundación de conformance (aditiva, sin romper nada). Despachable YA.**
  - Gate A1: **#1 `orm`** publica `orm@vX` con `orm/conformance` + `orm/mock`. (Mantiene DDL intacto.)
  - Gate A2: **#2 `ddl`** creado con `gonew`, publica `ddl@v0.0.1` con runtime DDL + `ddl/conformance`.
    Depende de `orm@vX` (Executor) y `ddlc`.
  - Gate A3 (paralelo, tras A1+A2): **#3 sqlt**, **#4 postgres** prueban ambos conformances; **#5
    indexdb** prueba el de DML. Cada uno publica.
  - **Salida de la Ola A:** dos contratos ejecutables verdes en todos los backends; `orm.DB` aún tiene
    DDL (deprecado, no removido). Los consumidores siguen compilando sin cambios.
- **Ola B — Migración de consumidores (#7).** Cada repo consumidor cambia sus llamadas DDL a `ddl`.
  Aditivo respecto a `orm` (que aún ofrece ambos). Se despacha uno por repo.
- **Ola C — Contracción de `orm` (#6).** Solo cuando la Ola B está 100%: `orm` borra su superficie DDL
  y publica el major/minor breaking. `app` al final (integra a todos, memoria
  [[tinywasm-dispatch-doctrine]]).

> **Por qué `orm` no borra DDL en la Ola A.** `CreateTable`/`Sync` los llaman ~10 repos (app, ormc,
> user, goflare-demo, 4 módulos veltylabs). Borrarlos antes de migrarlos rompe el ecosistema entero de
> golpe. Expand/contract lo evita.

## 5. El contrato de conformance (idéntico patrón `router/conformance`)

Cada suite es un paquete **no-`_test`** que importa `testing` a propósito (un `_test.go` no se importa
desde otro repo), expone `Run(t, Factory)`, y cada cláusula de comportamiento es un `t.Run`. Un backend
prueba conformidad desde su propio paquete de test con un `Factory` que construye una instancia fresca.

- **`orm/conformance`** (DML): round-trip Create→ReadOne/ReadAll/Update/Delete + Conditions/OrderBy/
  Limit/Offset + operadores (`= != > >= < <= IN LIKE`) + lógica AND/OR + `ReadOne` sin match ⇒
  `ErrNotFound`. **Cero DDL**: el `Factory` entrega un `*orm.DB` con la tabla **ya lista** (mock la
  auto-crea al insertar; sqlite/postgres corren `ddlc.ExportDDL` en el factory; indexdb la declara vía
  `structTables`).
- **`ddl/conformance`** (esquema, solo SQL): `CreateTable` crea columnas correctas (verifica por
  introspección); `CreateTable` idempotente; `Sync` añade columna nueva; `DropTable` elimina. `Factory`
  provee el compilador DDL del dialecto + un executor + introspección.

## 6. Doctrina de despacho

- `gotest` para tests; `gopush 'msg'` para publicar (tag automático). Prereq del agente:
  `go install github.com/tinywasm/devflow/cmd/gotest@latest`.
- Planes cross-repo autocontenidos, contrato inline; sin enlaces markdown cross-repo dentro del código.
- No tocar repos ya publicados sin pedirlo; `app` siempre al final ([[tinywasm-dispatch-doctrine]]).
- Cada `docs/PLAN.md` terminado se **borra** tras publicar; su diseño duradero pasa a
  `ARCHITECTURE.md`/`README.md` del repo ([[codejob-plan-lifecycle]]).

## 7. Orden de creación de `tinywasm/ddl` (gonew)

```bash
gonew ddl "Runtime DDL for tinywasm/orm: CreateTable/DropTable/Sync + ddl/conformance" -owner=tinywasm
```
Luego se mueve [`DDL_REPO_PLAN.md`](DDL_REPO_PLAN.md) a `tinywasm/ddl/docs/PLAN.md` y se despacha.
