# PROPUESTA — Extraer el contrato de almacenamiento a `tinywasm/storage` (puerto) y dejar `tinywasm/orm` como capa ergonómica

> Estado: 🟠 **propuesta / RFC — requiere tu OK antes de implementar.** Fecha: 2026-07-16.
> Refina (no contradice) el [`DDL_DML_SPLIT_MASTER_PLAN.md`](DDL_DML_SPLIT_MASTER_PLAN.md).
> Doctrina obligatoria: [`CONSTRUCTION_HARNESS.md`](CONSTRUCTION_HARNESS.md).

---

## 0. TL;DR

Hoy `tinywasm/orm` hace **dos** cosas que siguen mezcladas incluso después del split DDL/DML:

1. **Define el contrato de almacenamiento** (las interfaces `Executor`/`Compiler`/`Scanner`/`Rows`, los
   tipos de valor `Query`/`Condition`/`Order`/`Plan`, el conformance y el mock en memoria). Esto es lo
   que un backend (`postgres`, `sqlt`, `indexdb`) necesita.
2. **Ofrece la API ergonómica** (`orm.DB`, el query builder `Where/OrderBy/ReadAll`, `Create/Update/Delete`).
   Esto es lo que un desarrollador de una app/módulo hoja necesita.

Por eso `postgres`/`sqlt`/`indexdb` dependen de `tinywasm/orm` — y tú lo notas mal, con razón: **un
driver de base de datos no debería depender de un ORM.** Debería depender solo del contrato.

**Propuesta:** extraer el contrato a un paquete nuevo **`tinywasm/storage`** (el "puerto"). Los backends
dependen solo de `storage`. `orm` pasa a ser la capa ergonómica encima de `storage`. `ddl` también deja de
depender de `orm` y pasa a depender de `storage`.

Esto **no es una idea nueva ni arriesgada**: es exactamente el split que la stdlib de Go ya hace entre
`database/sql/driver` (el puerto que implementan los drivers) y `database/sql` (la capa ergonómica,
`sql.DB`). Estás redescubriendo un patrón probado. Ver §3.

---

## 1. El problema, en concreto

Después del split DDL/DML, `orm` quedó "DML-puro", pero **sigue empaquetando dos responsabilidades
distintas con dos audiencias distintas**:

| Símbolo en `orm` hoy | ¿Quién lo usa? | ¿Qué es realmente? |
|---|---|---|
| `Executor`, `Compiler`, `Scanner`, `Rows`, `TxExecutor`, `TxBoundExecutor` | **backends** (postgres/sqlt/indexdb) | el **contrato** que un backend implementa |
| `Query`, `Action`, `Condition`, `Order`, `Plan` | orm ↑ los produce, backends ↓ los consumen | los **tipos que cruzan la frontera** orm↔backend |
| `Eq/Gt/In/IsNotNull/...`, `ErrNoRows`, `ScanAny` | backends y ddl | helpers del **contrato** |
| `orm/conformance` | backends (para probarse) | el **contrato ejecutable** |
| `orm/mock` (recorders + `NewDB` en memoria) | backends + hojas en tests | doble de test del **contrato** |
| `orm.DB`, `QB` (`Where/OrderBy/ReadOne/ReadAll`), `Create/Update/Delete` | **apps/módulos hoja** | la **API ergonómica** (el ORM propiamente dicho) |
| `Open`/`Register` (registro DSN) | apps + backends (init) | conveniencia de arranque |

Las primeras **cinco** filas son **el puerto**. La **sexta** es **el ORM** (la API ergonómica). La
**séptima** (`Open`/`Register`) es conveniencia de arranque que, además, hoy obliga a los backends a
importar `orm` solo para auto-registrarse (ver §6.6). Todo vive en el mismo repo, así que un backend
que solo quiere las interfaces arrastra (conceptualmente, y en la superficie de API que ve su autor)
todo el query builder. Eso viola directamente varios principios del harness:

- **Principio 5 (superficie mínima):** el autor de un backend ve `Where/OrderBy/ReadAll` que no le
  incumben.
- **Principio 9 (piezas lego, una responsabilidad por librería):** `orm` tiene dos.
- **La regla de las costuras:** el tipo que cruza entre `orm` y un backend (`Query`, `Plan`) está
  declarado en `orm`, no en una pieza compartida neutral. Un backend "consume `orm`" en vez de
  "cumplir un contrato".

---

## 2. Objetivo

Que la respuesta a **"¿de qué depende un driver de base de datos?"** sea:

> De `tinywasm/storage` (el contrato) y nada más. No del ORM, no del query builder.

Y que un autor de backend, sin contexto, abra `tinywasm/storage`, vea **solo** lo que debe implementar más
"corre el conformance para probarlo", y termine guiado por autocompletado. Ese es el **acid test** del
harness (§199 de CONSTRUCTION_HARNESS).

---

## 3. El precedente que valida el instinto: `database/sql` vs `database/sql/driver`

La stdlib de Go resolvió este mismo problema hace más de una década. Tiene **dos** paquetes:

| stdlib | Rol | Quién lo importa |
|---|---|---|
| `database/sql/driver` | El **puerto**: `driver.Conn`, `driver.Stmt`, `driver.Value`, `driver.Rows`… las interfaces y los tipos de valor que cruzan la frontera. | Los **drivers** (`lib/pq`, `mattn/go-sqlite3`). |
| `database/sql` | La capa **ergonómica**: `sql.DB`, `sql.Query`, `sql.Rows`, el pool, el registro… | Las **apps**. |

Un driver de Postgres implementa `driver.Conn`; **no** implementa ni importa `sql.DB`. La app usa
`sql.DB`; nunca ve `driver.Conn`. Exactamente la separación que buscas.

La correspondencia con nuestro ecosistema es 1:1:

```
database/sql/driver   ≙   tinywasm/storage     (el puerto: interfaces + tipos de valor + conformance + mem)
database/sql          ≙   tinywasm/orm    (la ergonomía: orm.DB, Where/ReadAll, Create/Update/Delete)
```

**Mejora sobre la stdlib:** `database/sql/driver` no trae un conformance ejecutable ni una
implementación en memoria de referencia (los drivers se prueban ad-hoc). Nosotros sí los ponemos en
`storage`, porque el harness exige que el contrato sea ejecutable ("An API is not published until a
consumer-shaped test, inside the library itself, proves it"). Es decir: hacemos lo que Go hace, y
además cerramos el agujero que Go dejó abierto.

---

## 4. Arquitectura propuesta

```
tinywasm/model   — definiciones de datos (Field, Definition, FieldWriter/Reader).            [fmt]
      │
      ▼
tinywasm/storage      — PUERTO DE ALMACENAMIENTO:
      │             · interfaces: Executor, Compiler, Scanner, Rows, Tx*, Conn
      │             · tipos de valor: Query, Action, Condition (+Eq/Gt/In/IsNotNull…), Order, Plan
      │             · sentinela: ErrNoRows · helper: ScanAny
      │             · storage/conformance  (contrato DML ejecutable, driven por el contrato crudo)
      │             · storage/mem          (backend en memoria de referencia — funcional)
      │             · storage/mock         (recorders: dobles que capturan la Query)                [model]
      │
      ├───────────────────────────┬───────────────────────────┐
      ▼                           ▼                            ▼
tinywasm/orm                tinywasm/ddl               backends: postgres, sqlt, indexdb
  (ERGONOMÍA DML)             (runtime DDL)              implementan storage.Conn (Executor+Compiler),
  orm.DB, QB, Create/          ddl.DB, Sync…             corren storage/conformance;
  Update/Delete/Query,         ddl/conformance           sqlt/postgres además ddl/conformance.
  orm.New(conn)                (esquema, solo SQL)        DEP: [db]   (+ [ddl] si son SQL)
  DEP: [db]                    DEP: [db, ddlc]            NUNCA [orm]
```

Reglas de dependencia (una sola dirección, sin ciclos):

- `storage` depende solo de `model` (y `fmt`). Es el contrato; no conoce a nadie hacia arriba.
- **backends → `storage`** (y → `ddl` si hacen DDL SQL). **Nunca → `orm`.** ← *este es el objetivo central.*
- `orm` → `storage`. `ddl` → `storage` (+ `ddlc`).
- `orm` y `ddl` **no se conocen entre sí**.

---

## 5. Qué va en cada paquete (y por qué exactamente ahí)

| Símbolo | Paquete destino | Justificación (harness) |
|---|---|---|
| `Executor`, `Compiler`, `Scanner`, `Rows`, `TxExecutor`, `TxBoundExecutor` | `storage` | Son el contrato que cruza la costura orm/ddl ↔ backend. Deben vivir en la pieza compartida, no en un consumidor (regla de las costuras). |
| `Conn` (= `Executor` + `Compiler`) **[nuevo]** | `storage` | Une las dos mitades que todo backend implementa en un solo tipo. Ver §6.5. |
| `Query`, `Action`, `Condition`, `Order`, `Plan` | `storage` | Tipos de valor que orm produce y el backend consume. Contrato. |
| `Eq/Gt/Lt/In/Like/IsNotNull/…` | `storage` | Construyen `storage.Condition` (tipo del contrato). Usados por el builder de orm y directamente por `ddl` (safe-drop) y por `orm.Update(m, Eq(...))`. |
| `ErrNoRows`, `ScanAny` | `storage` | `ErrNoRows` es el sentinela que el backend **debe** mapear; `ScanAny` es el helper de scan que usa el backend en memoria. Contrato. |
| `storage/conformance` | `storage` | El contrato ejecutable. Los backends lo corren. Prueba el contrato **crudo**, no el builder (§6.4). |
| `storage/mem` (`mem.New() storage.Conn`) | `storage` | Backend en memoria de referencia. `storage/conformance` se prueba contra él; las hojas lo usan en tests. Solo depende de `storage`. |
| `storage/mock` (recorders) | `storage` | Dobles que capturan la `Query` para verificar "¿mi código construyó la Query correcta?". Los usa `orm` en sus tests y cualquier consumidor. |
| `orm.DB`, `QB`, `Where/OrderBy/Limit/Offset/ReadOne/ReadAll`, `Create/Update/Delete`, `Tx` | `orm` | La API ergonómica. Audiencia: apps/hojas. No le incumbe a un backend. |
| `orm.New(conn storage.Conn)` | `orm` | El punto de ensamblaje: envuelve un backend (o `mem`) en el handle ergonómico. |
| `ErrNotFound` | `orm` | Es el error de `ReadOne` (concepto del builder: "no hubo fila para tu ReadOne"). El backend solo conoce `storage.ErrNoRows`; `ReadOne` lo traduce a `ErrNotFound`. |
| `CreateTable/DropTable/Sync/SyncSchema`, `ddl.Compiler`, `Op`, `Stmt`, `TableIntrospector`, `ddl/conformance` | `ddl` | Ya decidido en el split DDL/DML. Cambia solo su dependencia: **`storage` en vez de `orm`** (§6.7). |

---

## 6. Decisiones justificadas, una por una

### 6.1. Por qué extraer el contrato a `storage` (y no dejarlo en `orm`)

Porque la dependencia `backend → orm` es la deuda. Un backend implementa un contrato; el contrato debe
vivir en una pieza cuya única razón de existir sea **ser ese contrato**. Mientras el contrato viva
dentro del ORM, "cumplir el contrato" y "usar el ORM" son indistinguibles en el grafo de
dependencias, y el autor del backend ve una superficie que no le corresponde. Esto es literalmente el
ejemplo de la stdlib: `driver.Conn` no vive en `database/sql`, vive en `database/sql/driver`.

### 6.2. Por qué el nombre `storage`

- Es el sustantivo que **todos los backends son**: `postgres`, `sqlt`, `indexdb` "son un db". El
  contrato se llama como la cosa que se implementa. `storage.Executor` se lee "el executor que un db debe
  dar"; `storage.Conn` "la conexión que un db expone"; `storage/conformance` "prueba que eres un db".
- Es corto y sigue el estilo del ecosistema (`model`, `form`, `router`, `json`, `dom`).
- Alternativas consideradas y descartadas:
  - `driver` (como stdlib): correcto pero menos intuitivo en nuestro vocabulario; "driver" es jerga.
  - `store`/`storage`: válidos, pero `storage` es más directo y ya es la palabra que usamos al hablar.
  - `data`: demasiado genérico.
- **Ojo — colisión de lectura:** el paquete `storage` coexistiría con el tipo `orm.DB`. No es un problema
  real (es exactamente `database/sql/driver` + `sql.DB`), pero si te incomoda, la alternativa es
  renombrar el handle a `orm.Conn` o `orm.Client`. **Recomiendo mantener `orm.DB`** por el precedente
  `sql.DB` (un handle llamado DB es familiar). → *decisión abierta, §10.*

### 6.3. Por qué `orm` sigue siendo un repo separado (y no fusionar todo en `storage`)

Porque son dos responsabilidades con dos audiencias. Si el query builder viviera en `storage`, un backend
que importa `storage` para las interfaces compilaría también `Where/OrderBy/ReadAll` — volveríamos al mismo
enredo con otro nombre. La separación por audiencia es la que mantiene la superficie mínima:

- Autor de **backend** → abre `storage`, ve solo el contrato + conformance + mem. 
- Autor de **app** → abre `orm`, ve solo `Create/Query/Where/ReadAll`.

Cada uno ve exactamente lo suyo. Eso es el harness cerrado.

### 6.4. Por qué el conformance vive en `storage` y prueba el **contrato crudo**, no el query builder

Hoy `orm/conformance` maneja `storage.Query(&got).Where("id").Eq("w1").ReadOne()` — es decir, prueba
**builder + backend juntos**. Eso está bien para probar el stack completo, pero para un **conformance
de backend** es impreciso: las obligaciones de un backend son *compilar una `Query` y ejecutar un
`Plan` correctamente*; el query builder **no** es obligación del backend.

Propuesta: `storage/conformance` construye valores `storage.Query{...}` directamente y llama
`Compile`+`Exec`/`Query`+`Scan`, verificando filas. Esto:

- Prueba **exactamente** el contrato del backend, aislado del builder.
- Permite que `storage/conformance` **no dependa de `orm`** (si dependiera, tendríamos ciclo o una
  dependencia invertida fea).
- Es *consumer-shaped* desde la perspectiva del backend: `orm` solo llama a `compiler.Compile` +
  `exec.Exec/Query`; el conformance ejercita justo eso.

¿Y quién prueba que `.Where("id").Eq("w1").ReadOne()` produce la `Query` correcta? **`orm`, en sus
propios tests**, con los recorders de `storage/mock` que capturan la `Query`. Y el end-to-end
"builder + backend real" se prueba en `orm` corriendo la API fluida contra `storage/mem`. Cada librería
tiene su test *consumer-shaped* (la regla que mantiene el harness honesto, §127 de CONSTRUCTION_HARNESS).

### 6.5. Por qué introducir `storage.Conn` y que `orm.New` tome **un** argumento

Hoy `orm.New(exec Executor, compiler Compiler)` toma dos. Pero todo backend implementa **ambos** en un
mismo tipo (el engine es una sola struct). Dos argumentos permiten un estado ilegal: pasar un
`exec` de un backend y un `compiler` de otro. El harness dice "illegal states unrepresentable"
(principio 3) y "fail at compile time" (principio 6).

Solución: `storage.Conn interface { Executor; Compiler }`. Los backends devuelven un `storage.Conn`; `orm.New(conn
storage.Conn)` toma uno solo. Imposible desparejar. Bonus ergonómico: el ensamblaje pasa de dos líneas a
una, e igual para el mem (`orm.New(mem.New())`).

### 6.6. Por qué eliminar el registro `Open`/`Register` (o, si se conserva, moverlo a `storage`)

El registro DSN (`orm.Register("postgres", factory)` + `orm.Open("postgres://…")`) es un **lookup por
string en runtime**: si el scheme no está registrado, falla en runtime con "unknown scheme". Eso es un
*silent-ish failure* que el harness quiere evitar (principio 6: fallar en compilación, no en runtime).
Además, para que un backend se auto-registre en su `init()`, tendría que importar el paquete donde vive
`Register` — si eso es `orm`, **reintroducimos la dependencia backend → orm que estamos eliminando**
(es justo el compromiso que hace `database/sql`: los drivers importan `database/sql` solo para
`sql.Register`).

Podemos hacerlo mejor que la stdlib. Recomendación: **eliminar el registro** y usar constructores
tipados explícitos:

```go
// El backend expone un constructor que devuelve el PUERTO (no el ORM):
func sqlt.Open(dsn string) (storage.Conn, error)

// El consumidor ensambla la capa ergonómica explícitamente:
conn, err := sqlt.Open("sqlite::memory:")
d := orm.New(conn)
```

Ventajas: sin lookup por string, sin error de runtime "unknown scheme", `sqlt` depende solo de `storage`,
y el cambio de backend es un cambio de import+constructor (explícito y greppable) en vez de un string
mágico. → *decisión abierta, §10: eliminar vs. mover el registro a `storage`.*

### 6.7. Efecto colateral bueno: `ddl` se simplifica

En el plan actual, `ddl.New(exec orm.Executor, ddlCompiler ddl.Compiler, dmlCompiler orm.Compiler)`
necesitaba un `orm.Compiler` para el safe-drop (una lectura DML). Con el contrato en `storage`, ese
argumento pasa a ser `storage.Compiler`, y `ddl` **deja de depender de `orm`**:

```go
func ddl.New(conn storage.Conn, ddlCompiler ddl.Compiler) *ddl.DB
// el safe-drop usa conn.Compile (storage.Compiler) + conn.Query (storage.Executor). Cero orm.
```

`ddl` pasa a depender de `storage` + `ddlc`. Más limpio que el plan vigente.

### 6.8. Por qué el ORM es una capa **opcional** y **no** parte del contrato de `storage`

Esta es la decisión bisagra. La respuesta: **el ORM (el builder `Create`/`Query`/`Where`/`ReadAll`) es
una capa opcional encima del puerto, nunca una obligación del contrato `storage.Conn`.**

**La línea que divide contrato de ergonomía** no es "¿es útil?" (todo lo es), sino **¿esto varía por
backend?**

- **Lo que VARÍA por backend** = `Compile` (traducir `Query` → SQL del dialecto) + `Exec`/`QueryRow`/
  `Query` (correr el `Plan`). Postgres, sqlite e indexdb lo hacen distinto. **Eso, y solo eso, es el
  contrato** (`storage.Conn`).
- **Lo que es INVARIANTE entre backends** = `Create(m)`, `Query(m).Where(...).ReadAll(...)`. Los cuatro
  backends construyen **exactamente la misma** `storage.Query{Action: ActionCreate, Columns, Values}` a
  partir del mismo `model`. No cambia ni una línea según el dialecto.

El harness (§88 de CONSTRUCTION_HARNESS) es tajante: *"The glue is written once, in the library that
owns it. If every application would write the same wiring, that wiring belongs to a piece."* El builder
es glue **invariante** → se escribe **una vez, encima del contrato**, jamás como obligación del
contrato.

**Por qué NO puede ser parte del contrato:**

1. **Forzaría duplicación.** Si `Create`/`Query`/`Where` fueran métodos que un `storage.Conn` debe
   implementar, cada backend reimplementaría código idéntico. Ese es justo el fork que el harness
   prohíbe (principio 9). Un backend implementa lo que varía, no lo que es igual para todos.
2. **El conformance dejaría de ser preciso.** Si el builder fuera del contrato, `storage/conformance`
   probaría el builder contra cada backend N veces — desperdicio, porque el builder es el mismo
   siempre. Sacándolo del contrato, el conformance prueba solo lo que varía (§6.4) y el builder se
   prueba **una vez** en `orm`.
3. **Rompería la pureza de audiencia** (principio 5). El autor de un backend vería `Where`/`ReadAll`,
   que no le incumben. (Este es el mismo argumento de §6.3, visto desde el contrato en vez del
   packaging.)
4. **El builder ni siquiera es la última palabra en ergonomía.** `.Where("id")` toma un **string** —
   eso es un hueco genérico (principio 1): `.Where("idd")` compila y falla en runtime. La API
   *verdaderamente* harness-compliant para un modelo concreto es la que **genera `ormc`** con símbolos
   tipados (`User_.Id`, o accessors `ReadOneUser(...)`), donde una columna mal escrita **no compila**.
   Esa capa generada también se apoya en el mismo `storage.Conn`. Si el builder estuviera soldado dentro de
   `storage`, `storage` quedaría casado con **un** estilo ergonómico para siempre; con el puerto limpio y estable,
   **coexisten y evolucionan varias capas** sobre él (el fluent de `orm`, los accessors tipados de
   `ormc`) sin tocar lo que los backends implementan. Es Open/Closed a nivel de arquitectura — y es
   exactamente por eso que `database/sql/driver` **no** contiene `sql.DB`.

**"Opcional" no significa "no guiado"** (para que no suene contradictorio con el harness):

- **Opcional a nivel de arquitectura:** el puerto `storage` se sostiene solo y tiene **consumidores directos
  reales** que NO usan el builder fluido: `ddl` (arma `storage.Query` a mano para el safe-drop, §6.7),
  `storage/conformance` (§6.4), los tests internos de cada backend, y el código generado por `ormc`. El
  camino crudo es de primera clase para infraestructura.
- **Pero sigue siendo el camino guiado para código de aplicación:** un dev (o un LLM) escribiendo
  lógica de negocio usa `orm.New(conn)` + `Create`/`Query`/`Where`/`ReadAll` — o los accessors tipados
  de `ormc`. Nunca arma `storage.Query{}` a mano. Para esa audiencia el builder es la superficie obvia y
  recomendada; solo que se **entrega como capa, no horneada en el contrato**.

En términos de la stdlib: `sql.DB` es opcional sobre `driver` — puedes hablarle a `driver`
directamente (las herramientas de migración lo hacen), pero una app usa `sql.DB`. Nadie considera
`sql.DB` "parte del contrato del driver". Idéntico aquí.

> **En una frase:** el contrato de `storage` es *lo que un backend debe implementar* (`Compile` + `Exec`); el
> ORM es *cómo un humano/LLM prefiere hablarle a cualquier backend* — invariante, escrito una vez,
> opcional a nivel de arquitectura pero el camino guiado para apps. Meterlo en el contrato obligaría a
> cada backend a cargar glue idéntico y congelaría un solo estilo ergonómico sobre un puerto que debe
> quedar abierto a capas mejores (las tipadas de `ormc`).

---

## 7. Impacto por repo

| Repo | Cambio | ¿Rompe? |
|---|---|---|
| **`tinywasm/storage`** (nuevo, `gonew`) | Recibe interfaces + tipos de valor + `Eq/…` + `ErrNoRows` + `ScanAny` + `storage/conformance` + `storage/mem` + `storage/mock`, movidos desde `orm`. | N/A (nuevo) |
| **`tinywasm/orm`** | Se queda solo con `orm.DB`/`QB`/`Create/Update/Delete`/`Tx`/`ErrNotFound`. Importa `storage`. `orm.New(conn storage.Conn)`. Sus tests de builder usan `storage/mock`; su e2e usa `storage/mem`. | Sí (API import cambia) |
| **`tinywasm/ddl`** | `ddl.New` toma `storage.Conn` + `ddl.Compiler`. Dep `orm` → `storage`. Su plan (`ddl/docs/PLAN.md`) se re-corrige. | N/A (aún sin implementar) |
| **`postgres`, `sqlt`, `indexdb`** | Implementan `storage.Conn` (antes `orm.Executor`+`orm.Compiler`). `Open(dsn) (storage.Conn, error)`. Corren `storage/conformance`. Dep `orm` → `storage`. | Ya rotos por el break de orm; se reescriben **una sola vez** contra `storage`. |
| **hojas** (`app`, `user`, `veltylabs/*`, `goflare-demo`, `ormc`) | En prod: `conn, _ := backend.Open(dsn); d := orm.New(conn)`. En tests: `orm.New(mem.New())`. | Ya rotos por el break; migran una vez. |

**Timing — por qué AHORA es el momento correcto:** los backends `sqlt`/`postgres`/`indexdb` ya están
rotos por el break de `orm` (master plan §3, filas 3-5) y **deben reescribirse igual**. Si hacemos la
extracción a `storage` *antes* de reescribirlos, se reescriben **una vez** contra `storage`. Si no, se
reescriben contra `orm` ahora y **otra vez** contra `storage` después. Hacerlo ahora ahorra una migración
completa del ecosistema. Es la ventana de oro.

---

## 8. Cómo se ve para el usuario final (el acid test)

**Autor de un backend** (p. ej. `sqlt`) — abre `tinywasm/storage`, y todo lo que ve es lo que debe cumplir:

```go
package sqlt

import (
	"github.com/tinywasm/storage"
	"github.com/tinywasm/model"
)

type conn struct{ /* *sql.DB interno */ }

func Open(dsn string) (storage.Conn, error)                                  { /* … */ }
func (c *conn) Compile(q storage.Query, m model.Model) (storage.Plan, error)      { /* SQL del dialecto */ }
func (c *conn) Exec(query string, args ...any) error                   { /* … */ }
func (c *conn) QueryRow(query string, args ...any) storage.Scanner          { /* mapea sql.ErrNoRows → storage.ErrNoRows */ }
func (c *conn) Query(query string, args ...any) (storage.Rows, error)       { /* … */ }
func (c *conn) Close() error                                           { /* … */ }

// Y lo prueba con el contrato ejecutable, sin escribir asserts a mano:
func TestConformance(t *testing.T) {
	conformance.Run(t, conformance.Factory{
		Name: "sqlite",
		New:  func(t *testing.T) storage.Conn { return newTestConn(t) },
	})
}
```

Ni una mención a `orm`, `Where`, `ReadAll`. Guiado por autocompletado. **Acid test cerrado.**

**Desarrollador de una app hoja:**

```go
import (
	"github.com/tinywasm/orm"
	"github.com/tinywasm/sqlt"
)

conn, _ := sqlt.Open("sqlite::memory:")
d := orm.New(conn)                       // handle ergonómico
d.Create(&User{Id: "u1", Name: "Ada"})
var u User
d.Query(&u).Where("id").Eq("u1").ReadOne()
```

Y en su test, sin driver real:

```go
import "github.com/tinywasm/storage/mem"

d := orm.New(mem.New())                  // misma API, backend en memoria
```

> El fluent `.Where("id")` es el camino cómodo, pero recuerda (§6.8) que la variante **más**
> harness-compliant es la generada por `ormc` con símbolos tipados por columna — sobre el mismo puerto,
> sin string mágico. El ORM fluido y los accessors generados son dos capas opcionales sobre `storage`, no el
> contrato.

---

## 9. Verificación harness (checklist de §144 CONSTRUCTION_HARNESS)

- **Huecos sin tipar:** `orm.New(exec, compiler)` (dos args desparejables) → `orm.New(conn storage.Conn)`
  (uno). ✅
- **Invariantes en runtime:** `Open("scheme://")` con "unknown scheme" en runtime → constructores
  tipados `backend.Open(dsn) (storage.Conn, error)` sin lookup por string. ✅ *(bajo la opción recomendada de
  §6.6; si se conserva el registro moviéndolo a `storage`, este punto no aplica.)*
- **Cosas "que hay que recordar":** desaparece "recuerda registrar tu driver en init". ✅ *(ídem — solo
  si se elimina el registro.)*
- **Contratos ausentes en las costuras:** `Query`/`Plan` que cruzan orm↔backend hoy viven en un
  consumidor (`orm`) → se nombran en la pieza compartida (`storage`). ✅
- **Piezas lego, no forks:** un backend deja de "importar el ORM"; cumple un contrato. ✅
- **Test consumer-shaped en la librería dueña:** `storage/conformance` corre contra `storage/mem` dentro de
  `storage`; `orm` corre su API fluida contra `storage/mem` dentro de `orm`. ✅

---

## 10. Decisiones abiertas (necesito tu OK antes de escribir planes)

1. **Nombre del puerto:** ¿`tinywasm/storage` (recomendado) o prefieres `store`/`storage`/`driver`?
2. **Nombre del handle ergonómico:** ¿mantener `orm.DB` (recomendado, por precedente `sql.DB`) o
   renombrar a `orm.Conn`/`orm.Client`? ¿Y mantener el repo llamado `orm`, o te sigue pareciendo poco
   intuitivo (alternativas: dejarlo; no veo una mejor)?
3. **Registro DSN:** ¿eliminarlo a favor de `orm.New(backend.Open(dsn))` (recomendado) o conservarlo
   moviéndolo a `storage` para que los backends no dependan de `orm`?
4. **`storage/mem` vs `storage/mock`:** ¿separo el engine funcional (`storage/mem`) de los recorders (`storage/mock`)
   por claridad semántica (recomendado), o los dejo juntos en un solo `storage/mock` como está hoy en
   `orm/mock`?

---

## 11. Qué NO cambia

- `model`, `ddlc`, `fmt` no se tocan.
- El split DDL/DML sigue en pie; esto lo **refina** (mueve el contrato un nivel más abajo). El
  `DDL_DML_SPLIT_MASTER_PLAN.md` se actualizaría para que las filas de dependencia digan `storage` donde hoy
  dicen `orm`.
- La regla "sin `map` en el ecosistema" (ya en los AGENTS.md de `orm` y `ddl`) se hereda en `storage`.

---

## 12. Siguiente paso

Si apruebas §10, escribo: (a) el `PLAN.md` autocontenido de `tinywasm/storage` (con el código a mover
inline), (b) la corrección de `orm` para depender de `storage`, (c) la re-corrección del `PLAN.md` de `ddl`,
y (d) la actualización del master plan. Nada de eso lo implemento hasta tu OK sobre los nombres.
