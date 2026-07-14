# MASTER PLAN — La política de seguridad es del consumidor; el vocabulario, del contrato

Hub de coordinación multi-repo. Cierra un hueco del arnés que **ya está produciendo
síntomas**: un agente ejecutando la fase D de `mjosefa-cms` se quedó bloqueado, y en cada
punto el "arreglo evidente" empeoraba el problema.

> Dispatch: 2026-07-13 · **Estado: 🚧 EN CURSO — A, B y C publicadas (`model` v0.0.12, `router` v0.1.9, `mcp` v0.1.21)**
> Doctrina: `app/docs/CONSTRUCTION_HARNESS.md`.

---

## 1. Los síntomas

Un agente en la nube preguntó dos cosas al llegar a la autenticación:

1. *"No encuentro `CreateUser` para sembrar el admin. ¿Hago un insert directo a la DB?"*
2. *"`userserver.Module` no satisface `router.APIModule`. He escrito un wrapper. ¿Bien?"*

Las dos respuestas obvias son trampas: un **insert directo** salta hashing, invariantes y
eventos de seguridad (un backdoor auto-infligido); un **wrapper local** es un fork de una
responsabilidad ajena, que se duplicará en cada consumidor.

## 2. El problema de fondo (dos capas)

### 2.1 `tinywasm/user` mezcla mecanismo con política

| | Qué es | De quién debe ser |
|---|---|---|
| **Mecanismo** | hashear, sesiones/JWT, cookies, `/login`, `/logout`, guardar y comprobar permisos, eventos de seguridad | **la librería** |
| **Política** | qué roles existen y cómo se llaman, qué recursos hay, quién recibe qué, si hay comodín, cómo se llama el primer usuario | **el consumidor** |

Hoy la librería decide la política. La evidencia:

- `user.go:112-114` exporta `RoleCodeAdmin = "admin"`, `ResourceAll = "*"`, `ActionAll = "*"`.
  Una app cuyos roles sean `dueño`/`recepción` hereda un vocabulario ajeno.
- **`Bootstrap` crea solo un rol con permiso comodín `*:*`** (`server/bootstrap.go`), con IDs
  (`role_admin`, `perm_all`) elegidos por la librería. Una app bajo mínimo privilegio **no
  puede decir que no**. Eso no es "cerrado por defecto": el default **concede todo**.
- El tool `me` hardcodea `Resource: "profile"` — un recurso que la app nunca declaró.

### 2.2 El vocabulario RBAC eran `string` pelados en tres contratos

`router.Route.Requires(resource, action string)`, `router.RouteInfo.Resource string`,
`mcp.Tool.Resource string`. Un recurso, una acción y un rol eran intercambiables; un typo
compilaba y fallaba como **denegación silenciosa**, en el único sitio donde el silencio es
inaceptable.

### 2.3 El error que yo mismo cometí (queda como aviso)

Al ver el síntoma (2), añadí `ModelName()` devolviendo el literal `"user"` y lo publiqué:
**`user` v0.0.32**. Eso mete un tercer string mágico y lo hace **divergir** del `"profile"`
de sus propios tools. `Platform.CanView` filtra la UI por identidad de módulo y el servidor
exige por recurso: si divergen, **nadie falla ruidosamente** — al usuario se le muestra una
sección y luego se le niegan los datos.

Es la tesis de este plan en una línea: **cuando la política vive en la librería, cada
"arreglo obvio" añade otro literal y otra forma de divergir en silencio.** v0.0.32 queda
**obsoleta**; la fase E la reemplaza.

## 3. La decisión

### 3.1 El vocabulario vive en `tinywasm/model` — no en una librería nueva

Se evaluó crear `tinywasm/rbac`. **Descartado.** El tipo no puede vivir en `user` (ciclo:
`user` importa `mcp`, y `mcp` tendría que importar `user` para tiparse), pero de ahí no se
sigue que necesite un módulo propio: se sigue que necesita **una librería sin dependencias
que todos ya importen**. Y esa existe.

`model` ya es el contrato del ecosistema: lo importan `router` (por `APIModule`), `mcp` (por
`Tool.Args`), `orm`, `form`, `user` y **cada módulo de dominio** (su código de `ormc` está
lleno de `model.Field`). Un módulo aparte les añadiría **una dependencia nueva a todos** para
darles cinco tipos que ya podían tener gratis. El arnés lo dice: *"reutiliza los tipos ya
declarados"*, *"superficie mínima"*.

**Y la ganancia decisiva no es ahorrar un módulo:** `model` ya declara `ModuleNaming`
(la identidad). Poner el vocabulario **al lado** permite atar identidad y recurso **con
tipos**, en vez de con una convención escrita en un documento:

```go
// tinywasm/model — publicado en v0.0.10
type Resource string   // ABIERTO: el lenguaje del dominio lo declara la app
type RoleCode string   // ABIERTO: los roles los nombra la app

// CERRADO: los cuatro verbos de la persistencia, como máscara de bits. Los 107 tools del
// ecosistema ya usaban 'c','r','u','d' y ninguno necesitó un quinto verbo. Un string libre
// no compraba nada y costaba toda la clase de typos ("raed" compilaba y denegaba en
// silencio). Un verbo de dominio ("approve") no es una acción: es otro recurso.
type Action uint8
const ( Create Action = 1 << iota; Read; Update; Delete )
const AllActions = Create | Read | Update | Delete

// Numérico porque SOLO un tipo numérico cierra el conjunto: con `type Action string`,
// Requires("orders","write") sigue compilando (Go convierte la constante sin tipo).
// Pero se GUARDA y se LOGUEA como letras — un 6 en una columna es ilegible:
func (a Action) String() string                 // Read|Update → "ru"
func ParseAction(s string) (Action, error)      // "ru" → Read|Update; letra rara → error

const Wildcard Resource = "*"  // solo para recursos: un conjunto cerrado no necesita comodín

// La convención "el ID de un módulo ES su recurso RBAC" deja de ser una nota y pasa a ser
// una función: identidad y recurso ya no pueden divergir.
func ResourceOf(m ModuleNaming) Resource

type Grant struct { Resource Resource; Actions Action }
func (g Grant) Matches(r Resource, a Action) bool   // el ÚNICO sitio que interpreta "*"
func AnyGrant(grants []Grant, r Resource, a Action) bool

type Authorizer func(userID string, r Resource, a Action) bool
func Allowed(auth Authorizer, userID string, r Resource, a Action) bool // nil => deniega
```

Zero value = deniega. Comodín = **mecanismo** (se sabe interpretar), **nunca política** (nadie
lo concede solo).

### 3.2 `Bootstrap` recibe la política, no la inventa

```go
type Seed struct {
	Email, Password, Name string
	Role   model.RoleCode
	Grants []model.Grant
}
func (m *Module) Bootstrap(s Seed) error // no-op si ya hay usuarios (idempotente)
```

La app que quiera un admin total lo escribe **en su código**, visible y `grep`-eable. La que
opere bajo mínimo privilegio **simplemente no escribe el comodín**. Hoy no puede evitarlo.

### 3.3 `Access`: tres estados, UNA declaración, zero value cerrado

`router` ya tenía los tres estados, pero **codificados por ausencia** (`Public bool` + un
`Resource` vacío-o-no). Eso hacía escribible un estado ilegal: `.Public().Requires(...)`
compilaba y la verja **descartaba el permiso en silencio**. Ahora `model.Access` los declara:

| Estado | Exige | Para |
|---|---|---|
| `AccessGuarded` (**zero value**) | identidad **y** permiso | todo lo que toca datos. Lo que no declara nada cae aquí y queda **inalcanzable** hasta declarar recurso. |
| `AccessAuthenticated` | identidad, sin recurso | operaciones sobre el propio llamador (`me`). |
| `AccessPublic` | nada | assets, login. Se escribe a propósito. |

Y la contradicción **falla al arrancar**, no en runtime: un tool *guarded* sin recurso
autorizaba contra `""` y **denegaba todas las llamadas** pareciendo protegido.

### 3.4 `me` deja de inventar un recurso

`me` devuelve **el perfil de quien llama**: la autenticación ya *es* la comprobación. No
necesita recurso — y así la librería deja de ensuciar el espacio de nombres del consumidor.
Requiere que `mcp` distinga un tercer estado (§4, fase C).

## 4. Fases

| Fase | Repo | Plan | Qué | Estado |
|---|---|---|---|---|
| **A (compuerta)** | `tinywasm/model` | — | Vocabulario tipado + `ResourceOf` + `Grant`/`Authorizer`, junto a `ModuleNaming`. Recursos y roles ABIERTOS (los declara la app); acciones CERRADAS (CRUD, máscara de bits). | ☑ **v0.0.10** |
| **B** | `tinywasm/router` | — | Frontera tipada + `Access` (v0.1.9): el `Public bool` junto a un `Resource` vacío-o-no hacía **escribible un estado ilegal** (`.Public().Requires(...)` descartaba el permiso en silencio). Ahora es UNA declaración. Además: `Requires(model.Resource, model.Action)`, `RouteInfo` tipado. Un test del propio repo usaba `Requires("orders", "write")` — **un verbo que no existe en CRUD**: prueba viva de que el string libre dejaba inventárselos. Ahora no compila. | ☑ **v0.1.8** |
| **C** | `tinywasm/mcp` | — | `Tool` tipado; `Config.Authorize model.Authorizer`; `Access` consumido de `model` (no duplicado). `AddTool` **falla al arrancar** si la declaración se contradice: guarded sin recurso (denegaría siempre en silencio) o público con recurso (parece protegido y no lo está). | ☑ **v0.1.21** |
| **D** | `tinywasm/server` | `docs/PLAN.md` | `httpd` tipa su `Authorize` y su verja con `model.Authorizer`. | ☐ |
| **E** | `tinywasm/user` | `docs/PLAN_POLICY.md` | Borrar `RoleCodeAdmin`/`ResourceAll`/`ActionAll` y la creación implícita de rol+comodín; `Bootstrap(Seed)`; `me` sin recurso; `ModelName` + aserción `router.APIModule`. **Break: v0.1.0. Deja obsoleta v0.0.32.** | ☐ |
| **F** | `veltylabs/mjosefa-cms` | su `docs/PLAN.md` (fase D) | `config/auth.go` declara **la política de la app** (roles, grants, semilla) y llama `Bootstrap(Seed)`. **Borrar el wrapper `AuthModule`.** | ☐ |

**Orden:** A ✅ → B ✅ → C ✅ → D → E → F. **`server/httpd` y `goflare/edge` están ahora en rojo**: es la señal correcta — el compilador rechaza a todo implementador que aún hable con strings. `goflare` implementa `router.Router`: la fase B lo rompe
y debe coordinarse con su cola (hoy tiene sesión de codejob).

## 5. Alternativas descartadas

1. **Dejarlo y que cada app haga un wrapper.** Fork de una responsabilidad ajena, duplicado en
   cada consumidor, y la app sigue sin poder declarar sus roles.
2. **Insert directo a la DB para sembrar el admin.** Salta hashing, invariantes y eventos de
   seguridad. Backdoor auto-infligido.
3. **Añadir `ModelName()` con un literal y seguir.** *Ya se probó y salió mal* (v0.0.32, §2.3).
4. **Dejar `RoleCodeAdmin`/`ResourceAll` como "defaults útiles".** Un default que **concede
   todo** no es útil: es un fail-open. El arnés exige que no escribir nada sea lo seguro.
5. **`Config{AdminRole: "admin"}` con strings.** Mueve el literal de sitio sin cerrar la clase
   de bug: sigue siendo un `string` confundible con un recurso o una acción.
6. **Librería nueva `tinywasm/rbac`.** Descartada por §3.1: añade una dependencia a todos y
   separa el vocabulario de la identidad, que es justo lo que hay que unir. *(El repo llegó a
   crearse; se descarta y se elimina.)*

## 6. Qué deuda se salda (y por qué no queda ninguna)

- **Se elimina un duplicado que llevaba meses ahí**: existía un `tinywasm/rbac` (v0.0.4,
  último commit 2026-02-20, ya archivado por el mantenedor) que **reimplementaba** roles y
  permisos con vocabulario divergente (`action byte` frente al `action string` de `user`).
  Dos verdades sobre el mismo concepto. Queda **una** implementación: `user`.
- **`user` v0.0.32 queda obsoleta explícitamente**, no abandonada: la fase E la reemplaza y
  aquí queda constancia de por qué no debe consumirse.
- **La política pasa a ser auditable**: `grep -rn "Grant\|RoleCode" config/` enumera **todo**
  lo que la app concede. Hoy hay que leer el `Bootstrap` de una librería para descubrir que
  existe un rol `*:*`.
- **No se difiere nada.** El tercer estado de acceso de `mcp` (fase C) se resuelve dentro del
  plan, no como "pendiente".

## 7. Qué responder al agente de la nube

> **No uses un insert directo a la DB, y no necesitas `CreateUser`:** `Bootstrap` ya existe y
> hace exactamente eso — siembra el primer admin solo si la tabla está vacía, con hashing e
> idempotencia.
>
> **Pero no lo cablees todavía, y quita el wrapper `AuthModule`.** Tu segunda pregunta destapó
> un problema de diseño real: la librería está decidiendo la política de seguridad de la app
> (crea sola un rol con comodín `*:*` e inyecta un recurso `profile` que la app nunca
> declaró). Eso lo declara el consumidor.
>
> `tinywasm/user` va a romper: `Bootstrap(Seed)` recibirá rol, grants y nombre desde
> `config/auth.go`, y el módulo satisfará `router.APIModule` sin wrapper. **Pausa D1/D2** hasta
> que se publique. Lo que ya tienes de la fase C no se ve afectado.
