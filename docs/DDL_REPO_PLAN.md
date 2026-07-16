---
PLAN: "feat: tinywasm/ddl — runtime DDL (repo nuevo) + ddl/conformance"
TAG: v0.0.1
---

# PLAN — `tinywasm/ddl`: runtime de DDL (repo NUEVO, vía gonew)

Orquestado por [`DDL_DML_SPLIT_MASTER_PLAN.md`](DDL_DML_SPLIT_MASTER_PLAN.md) — **pieza #2, Ola A**.
Autocontenido, en español. Tras `gonew`, este archivo se mueve a `tinywasm/ddl/docs/PLAN.md` y el agente
ejecutor **solo tiene ese repo**.

> **Prerequisito (entorno del agente):**
> ```bash
> go install github.com/tinywasm/devflow/cmd/gotest@latest
> ```
> Tests SIEMPRE con `gotest`. Publica SIEMPRE con `gopush 'mensaje'`.

## 0. Crear el repo (una vez, quien orquesta)

```bash
gonew ddl "Runtime DDL for tinywasm/orm: CreateTable/DropTable/Sync + ddl/conformance" -owner=tinywasm
cd ddl && mv <ruta>/DDL_REPO_PLAN.md docs/PLAN.md
```

## 1. Qué es y por qué

`tinywasm/orm` mezcla hoy DML (operar datos) y DDL (crear/migrar esquema). Este repo **es el runtime de
DDL**, hermano de `orm` (que queda como runtime de DML). Absorbe de `orm` toda la superficie de esquema
y añade su propio contrato ejecutable `ddl/conformance`. `tinywasm/ddlc` **no** cambia: sigue siendo la
CLI/codegen leaf que **genera** el SQL DDL (`Exporter.ExportDDL`) — `ddl` la **ejecuta** en runtime.

Ver arquitectura y secuencia expand/contract en el master. **Esta fase es aditiva**: `orm` conserva sus
métodos DDL (deprecados) hasta la Ola C; nada rompe.

## 2. Contratos que se mueven desde `orm` (verificados en `orm` hoy)

- `orm.DB.CreateTable(m model.Model) error` (`db.go:105`).
- `orm.DB.DropTable(m model.Model) error` (`db.go:121`).
- `orm.DB.CreateDatabase(name string) error` (`db.go:137`).
- `orm.DB.Sync(models ...model.Model) error` y `SyncSchema(table string, fields []model.Field) error`
  (`sync.go`) — algoritmo: emite CreateTable idempotente + `AddColumn` + rename/safe-drop
  introspectivo. Usa `RenameProvider.OldNames()` y `TableIntrospector.TableColumns(table)`.
- `orm.Action` DDL: `ActionCreateTable/DropTable/CreateDatabase/AddColumn/RenameColumn/DropColumn`
  (`query.go:15-20`) — hoy compiladas por el mismo `orm.Compiler.Compile` del dialecto (sqlt/postgres
  `translate.go`).
- Interfaces opcionales del executor: `TableIntrospector` (`sync.go`), `SchemaInspector`+`ColumnInfo`
  (`schema.go`).
- Se **reusa** de `orm` (no se mueve): `orm.Executor` (Exec/QueryRow/Query/Close). `ddl` importa `orm`
  solo para `Executor`.
- Se **reusa** de `ddlc`: `ddlc.Exporter` (`ExportDDL(models) (string, error)`), `ddlc.FieldExt`,
  `ddlc.TopologicalSort`.

## 3. Diseño del paquete `ddl`

`module github.com/tinywasm/ddl`, `go 1.25.2`. Depende de `tinywasm/orm`, `tinywasm/ddlc`,
`tinywasm/model`, `tinywasm/fmt`.

### 3.1 `ddl.Compiler` — la mitad DDL del actual `orm.Compiler`

Hoy el compilador del dialecto (sqlt/postgres) compila **todo** (DML + DDL) en un `Compile`. Se separa
la mitad DDL a una interfaz propia que `ddl` consume:

```go
package ddl

import (
	"github.com/tinywasm/model"
	"github.com/tinywasm/orm"
)

// Op is a DDL operation (schema), distinct from orm's DML actions.
type Op int

const (
	OpCreateTable Op = iota
	OpDropTable
	OpCreateDatabase
	OpAddColumn
	OpRenameColumn
	OpDropColumn
)

// Stmt is one DDL statement to run through an orm.Executor.
type Stmt struct {
	Op       Op
	Table    string
	Database string
	Column   *model.Field // for AddColumn/DropColumn
	OldName  string       // for RenameColumn
}

// Compiler is implemented by dialect adapters (sqlt, postgres). It renders a DDL Stmt to
// engine SQL. It is the DDL counterpart of orm.Compiler (which stays DML-only).
type Compiler interface {
	CompileDDL(s Stmt, m model.Model) (query string, args []any, err error)
}
```

> `sqlt`/`postgres` ya tienen estas ramas dentro de su `translate.go` (`case orm.ActionCreateTable:`
> etc.). Sus fases (#3/#4) las mueven detrás de `CompileDDL`. **No** reimplementes la generación SQL
> aquí: `ddl` orquesta; el dialecto renderiza.

### 3.2 `ddl.DB` — runtime que ejecuta el esquema

```go
// DB applies schema changes through an orm.Executor using a dialect Compiler.
type DB struct {
	exec     orm.Executor
	compiler Compiler
	log      func(...any)
}

func New(exec orm.Executor, compiler Compiler) *DB { return &DB{exec: exec, compiler: compiler} }

func (db *DB) CreateTable(m model.Model) error { /* CompileDDL(OpCreateTable) → exec.Exec */ }
func (db *DB) DropTable(m model.Model) error   { /* OpDropTable */ }
func (db *DB) CreateDatabase(name string) error
func (db *DB) SyncSchema(table string, fields []model.Field) error
func (db *DB) Sync(models ...model.Model) error // mover el algoritmo de orm/sync.go verbatim
```

- **Mueve** `orm/sync.go` (algoritmo `Sync`/`SyncSchema`, `schemaModel`, `RenameProvider`,
  `TableIntrospector`) a este repo, adaptando: donde emitía `orm.Query{Action: ActionAddColumn,...}` +
  `orm.Compiler.Compile`, ahora emite `ddl.Stmt{Op: OpAddColumn,...}` + `ddl.Compiler.CompileDDL`.
- `TableIntrospector`/`SchemaInspector`+`ColumnInfo` se **mueven** aquí (son introspección de esquema,
  DDL). El executor del dialecto los implementa igual que hoy.

### 3.3 `ddl/conformance` — contrato ejecutable de DDL (solo backends SQL)

`package conformance`, importa `testing`+`ddl`+`orm`+`model`. Mismo patrón que `router/conformance`.

```go
type Factory struct {
	Name string
	// New returns a fresh ddl.DB plus the orm.Executor it writes through (for introspection/DML
	// verification) and an introspector to read back the resulting schema. Called once per clause.
	New func(t *testing.T) (schema *ddl.DB, exec orm.Executor, cols func(table string) []string)
}

func Run(t *testing.T, f Factory) {
	t.Run("create_table_makes_expected_columns", ...) // CreateTable(&Widget{}) → cols == [id name qty active]
	t.Run("create_table_is_idempotent", ...)          // segundo CreateTable no falla
	t.Run("sync_adds_new_column", ...)                // Sync con un field extra ⇒ columna nueva presente
	t.Run("drop_table_removes_schema", ...)           // DropTable ⇒ tabla ausente
}
```

Usa el mismo modelo `Widget` que `orm/conformance` (id TEXT PK, name TEXT, qty INT, active BOOL);
**re-decláralo aquí** (o impórtalo de `orm/conformance` si ya es exportado — preferido: importar
`conformance.Widget` de orm para no duplicar el fixture).

> Backends que entran: **`sqlt`(sqlite)** y **`postgres`**. `indexdb`/`mock` **no** — no hacen DDL SQL.

## 4. Tests del propio repo

- `ddl/conformance` no se auto-prueba (necesita un dialecto real); su cobertura la dan sqlt/postgres.
- `ddl` (runtime) SÍ necesita test propio: un `ddl.Compiler` mock que registre los `Stmt` emitidos y un
  `orm.Executor` mock (usa `orm/mock.Executor` de la pieza #1) para verificar que `CreateTable`/`Sync`
  emiten los `Op` correctos en el orden correcto (incl. el algoritmo de rename/safe-drop de `Sync`).
  Cobertura alta del runtime.

## 5. Criterios de aceptación

- `github.com/tinywasm/ddl` existe (gonew), `go 1.25.2`, deps `orm`+`ddlc`+`model`+`fmt`.
- `ddl.DB` con `CreateTable/DropTable/CreateDatabase/Sync/SyncSchema`; `ddl.Compiler`/`ddl.Stmt`/`ddl.Op`;
  algoritmo `Sync` migrado verbatim (con `TableIntrospector`/`RenameProvider`).
- `ddl/conformance` con `Run(t, Factory)` + ≥4 cláusulas.
- Test runtime verde contra mocks (`orm/mock` + un `ddl.Compiler` de prueba).
- `orm` **no** se modifica en esta fase (sigue con su DDL deprecado). `ddl` no importa ningún driver.
- `gotest` verde; publicado `ddl@v0.0.1` con `gopush`.

## 6. Etapas

| # | Etapa | Archivo(s) | Criterio |
|---|---|---|---|
| 1 | gonew + go.mod | — | repo creado, deps `orm`/`ddlc`/`model`/`fmt` |
| 2 | `Compiler`/`Stmt`/`Op` | `compiler.go` | interfaz DDL del dialecto |
| 3 | `ddl.DB` + Sync migrado | `db.go`, `sync.go` | CreateTable/DropTable/Sync/SyncSchema |
| 4 | introspección | `schema.go` | `TableIntrospector`/`SchemaInspector`/`ColumnInfo` |
| 5 | `ddl/conformance` | `conformance/conformance.go` | `Run`+`Factory`+≥4 cláusulas |
| 6 | test runtime | `ddl_test.go` | Stmt emitidos correctos vía mocks |
| 7 | publicar | — | `gotest` verde; `gopush 'feat: runtime DDL + conformance'` |

## 7. Cierre

Tras `gopush`, **borra** `docs/PLAN.md`; el diseño duradero pasa a `README.md`/`docs/ARCHITECTURE.md`.
