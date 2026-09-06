# MASTER PLAN — Una decisión, un dueño: tipografía

Hub de coordinación multi-repo. Empezó como «que la web y el PDF usen la misma
tipografía» y al tirar del hilo apareció el patrón de fondo: **decisiones sin pieza que
las posea**. Una decisión que nadie posee acaba escrita en varios sitios, y nada impide
que diverjan.

> **Estado: 🚧 quedan 5 publicaciones.** Última revisión: 2026-08-04.
> Principios: [CONSTRUCTION_HARNESS.md](CONSTRUCTION_HARNESS.md) (1, 4, 5, 6, 9).

| Repo | Último tag | Estado |
|---|---|---|
| `tinywasm/color` | v0.1.1 | ✅ cerrado — un solo parser hex en el ecosistema |
| `tinywasm/chart` | v0.1.0 | ✅ cerrado — un subpaquete por tipo, sobre `pdf.Canvas` |
| `tinywasm/font` | v0.0.3 | 🚧 pendiente v0.1.0 — `font/docs/PLAN.md` |
| `tinywasm/pdf` | v0.1.1 | 🚧 pendiente v0.1.2 — `pdf/docs/PLAN.md` |
| `tinywasm/css` | v0.4.4 | 🚧 pendiente v0.5.0 — `css/docs/PLAN.md` |
| `tinywasm/ssr` | v0.1.x | 🚧 pendiente v0.2.0 — `ssr/docs/PLAN.md` |
| `tinywasm/assetmin` | v0.4.17 | 🚧 pendiente v0.5.0 — `assetmin/docs/PLAN.md` |
| `tinywasm/app` | — | ✅ no cambia (§3.5) |

**Orden de ejecución:** `font` primero (los demás dependen de su derivación de nombres) →
`css`, `pdf` y `ssr` en paralelo → `assetmin` al final, que consume a los tres.

---

## 1. El flujo, una vez cerrado

Una declaración, tres consumidores. Es el requisito de producto: el PDF y la página
tienen que verse con la misma tipografía, y eso sólo se garantiza si la familia se
escribe **una vez**.

```
config/fonts.go            (proyecto, SIN build tag)
   func Fonts() font.Declaration { return font.Declare("Roboto", "config/fonts") }
        │
        ├──► config/css.go (!wasm)      --font-sans: css.FontStack(Fonts().Family())
        │                                el valor viaja dentro del RootCSS() que ssr extrae
        │
        ├──► ssr                         ejecuta Fonts() y devuelve la Declaration
        │      └──► assetmin               ├─ copia las 4 caras → OutputDir
        │                                  └─ inyecta css.FontFaces(d, AssetsURLPrefix)
        │
        └──► WASM                        pdf.LoadDeclared(config.Fonts())
                                          mismo .ttf que la página ya cacheó
```

### Quién es dueño de qué

| Cosa | Dueño | Por qué nadie más |
|---|---|---|
| El nombre de la familia y de cada cara | `font` | `Family.Face(Style)` es la única derivación; escribir `"Roboto-Bold"` a mano la duplica |
| El texto CSS | `css` | `css/AGENTS.md §5`: la superficie para emitir CSS es ésa y sólo ésa |
| La URL final y los bytes en disco | `assetmin` | `AssetsURLPrefix` y `OutputDir` son suyos; es quien sirve (`http.go:14`) |
| El documento y sus caras registradas | `pdf` | consume la declaración, no la interpreta |

---

## 2. Lo que falta, y por qué en ese orden

### `font` v0.1.0 — un solo nombre para la cara regular

`Face(Regular)` devuelve `"Roboto"` mientras las otras tres llevan sufijo. `faces/README.md`
afirma que los archivos se llaman como `Face()` y al lado tiene `Roboto-Regular.ttf`: la
regla es falsa para una de cada cuatro caras. `pdf` lo tapa aceptando **los dos** nombres,
así que el dev no puede saber cuál es el correcto — hasta que `assetmin` copie usando uno
solo y la web quede sin fuente mientras el PDF sí la tiene.

Va primero porque `css` construye URLs con esa derivación y `assetmin` copia archivos con
ella.

### `pdf` v0.1.2 — una cara ausente falla, no se sustituye

Tres fallbacks silenciosos en `LoadDeclared`. Los dos peores no son de nombres: si falta
la itálica carga la **recta**, si falta la negrita itálica carga la **negrita**, y
`WritePdf()` devuelve `nil`. El documento sale sin cursivas y nadie se entera.
`docs/SPECS.md:6` lo documenta como una capacidad. El caso que lo justificaba —DroidSans,
sin cursiva real— ya está excluido del ecosistema.

### `css` v0.5.0 — `FontFaces(d, urlPrefix)`

`css` ya sabe **pedir** la fuente (`FontStack` → token `FontSans`) pero no **declararla**:
no hay `@font-face`, así que `--font-sans` cae al `system-ui` siguiente. Ése es
literalmente el síntoma original.

### `ssr` v0.2.0 — `Fonts()` como sexto productor

`assetmin` no puede leer la declaración por ningún otro camino: no parsea Go, y
`assetmin.Config` lo construye `tinywasm/app`, que no importa el `config` del proyecto
(§3.5). El programa que `ssr` genera y ejecuta ya invoca cinco productores; éste es el
sexto, y devuelve un `font.Declaration` tipado.

### `assetmin` v0.5.0 — copiar y servir

Recibe la declaración en `SSRAssets`, copia las cuatro caras a `OutputDir`, e inyecta
`css.FontFaces(...)` en el CSS principal vía `AddDynamicContent` (`asset.go:38`).

---

## 3. Decisiones cerradas, con su evidencia

### 3.1 Roboto, subseteada, estática

| 4 caras, TTF subseteado | Crudo | Servido (brotli) |
|---|---|---|
| **Roboto** | 115.096 B | **69.978 B** |
| Inter | 209.288 B | 95.725 B |

Inter era más liviana mientras el TTF se recortaba *sin* tablas de layout —cuando sólo
servía al PDF, que las ignora—. Al conservarlas para que el navegador aplique kerning,
sus tablas OpenType la ponen un 37% por encima. Roboto además es lo que Android ya
renderiza de forma nativa.

**Estática, nunca variable ni OTF:** el motor lee `glyf`/`loca`, ignora los ejes de
variación y no carga CFF. Verificado sobre `Inter-Regular.otf` (`OTTO`, sin `glyf`).
Las caras verificadas viven en `font/faces/`.

### 3.2 Un solo formato: TTF para web y PDF

| | Bytes que viajan | Peticiones |
|---|---|---|
| **Un TTF compartido** (brotli) | **16.132 B** | **1** |
| Un WOFF2 + un TTF | 27.890 B | 2 |

El PDF se genera **en el frontend**, así que pide el mismo archivo que la página ya bajó
para su `@font-face`: acierto de caché, no petición nueva. El navegador lee TTF
(`format("truetype")`); el motor de PDF no lee WOFF2. Escribir `format("woff2")` haría
que el navegador rechace el archivo.

> Una medición anterior daba 14.354 B. Era errónea: recortaba un archivo **ya recortado**,
> y `pyftsubset` no avisa de eso.

### 3.3 `config/fonts.go` no lleva `!wasm`

Un archivo `!wasm` no se compila para el navegador, y el PDF se genera ahí: ese código
necesita saber que la familia es `"Roboto"`. Marcarlo lo dejaría fuera del alcance de su
propio consumidor. Una declaración es identidad, e identidad cruza a WASM.

Su vecino `config/css.go` **sí** lleva el tag, porque devuelve valores. Ambos son el mismo
paquete Go, así que `RootCSS()` llama a `Fonts()` sin intermediarios.

**Regla transversal:** si un archivo de `font` o `color` necesita un build tag, o si
aparece un `[]byte`, la pieza está mal partida.

### 3.4 El `@font-face`: `assetmin` aporta la URL, `css` emite el texto

`css` no puede escribir la URL —no conoce `AssetsURLPrefix` ni `OutputDir`— y `assetmin`
no puede escribir CSS —sería una segunda forma de hacer lo que `css` ya hace—. Así que
`assetmin` llama a `css.FontFaces(d, prefix)`.

Esto **corrige dos versiones anteriores** de estos planes: una proponía que `css`
embebiera la fuente en base64 para esquivar el hueco de `assetmin`; otra, que `assetmin`
formateara la regla con `fmt.Sprintf`. Las dos son *a fork with a friendlier name*.

### 3.5 `ssr` extrae la declaración; `app` no cambia

Hay dos caminos desde `config/fonts.go`, y sólo uno necesita a `ssr`:

- **El valor** —el nombre de la familia dentro de `--font-sans`— no lo necesita:
  `config/fonts.go` y `config/css.go` son el mismo paquete Go, así que `RootCSS()` llama a
  `Fonts().Family()` y viaja dentro de la extracción que ya existe. **Ya funciona hoy.**
- **Los bytes** sí: `assetmin` necesita la `Declaration` para copiar las cuatro caras y
  construir la URL.

> **Corrección.** Una versión anterior de este documento decía que «`ssr` no participa»,
> con el argumento de que el composition root pasaría la declaración por
> `assetmin.Config`. Es falso: `assetmin.Config` lo construye `tinywasm/app`
> (`app/section-build.go:74`), que **no importa el paquete `config` del proyecto** — `app`
> es el CLI que corre *dentro* del proyecto, y ningún proyecto tiene un `main.go` que
> llame a `app`. Esa línea no la puede escribir nadie.

Leer un valor del código del proyecto tiene un solo mecanismo, y es el de `ssr`: compila y
**ejecuta** un programa generado que invoca los productores del paquete (`scanner.go:91-97`
tiene el conjunto cerrado de cinco). `Fonts` es el sexto, y por eso lo que llega es una
`font.Declaration` real y no una cadena reconstruida del AST.

**`tinywasm/app` no cambia ni una línea.** `h.AssetsHandler` ya está registrado como
`FilesEventHandler` (`section-build.go:200`), el `SSRFileWatcher` ya está enganchado
(`:281-285`) y el `UnobservedFiles` ya se agrega (`:214`). El enrutado de `fonts.go` vive
dentro de `assetmin`.

Lo que sigue siendo cierto es la dirección del grafo: `ssr` **importa** `assetmin`, no al
revés, y no tiene ni `os.WriteFile` ni `http.` ni `OutputDir`. Extrae valores; servir es
de `assetmin`.

### 3.6 Los `.ttf` no se recargan en caliente

Editar `config/fonts.go` sí dispara: es un `.go`, y tanto `server` como `client` declaran
`SupportedExtensions() → [".go"]`, así que recompilan; además entra por el enrutado de
`ssr_watcher.go` y reextrae el módulo.

Añadir un archivo `.ttf` no dispara nada hasta el siguiente arranque, **deliberadamente**.
Las cuatro caras se copian una vez, al montar el proyecto: no es un bucle de edición como
el CSS. Y si falta una cara el arranque falla nombrándola, así que no hay servidor
corriendo al que recargarle nada. Cerrar ese hueco costaría meter un binario en un modelo
de asset que concatena texto, o un watcher dentro de `tinywasm/font` que rompería su
invariante de no tocar `os.` ni `[]byte` — a cambio de ahorrar un reinicio, una vez.

### 3.7 `Theme` no es color

Se queda en `pdf`. Lleva `Sizes`, `Spacing`, `Page` y `Margin` en milímetros: es el tema
de un documento. Que contenga colores no lo convierte en uno.

---

## 4. Criterios de cierre

1. **Un proyecto declara su tipografía en un único sitio** (`config/fonts.go`), y web y
   PDF la usan sin que nadie repita el nombre.
2. **La página carga la fuente sin una sola petición externa.** Requisito de producto: los
   despliegues objetivo no tienen internet.
3. **El mismo `.ttf` sirve a la página y al PDF:** una sola descarga, comprobada en el
   panel de red del navegador.
4. **Una cara ausente produce un error que la nombra**, en los tres medios. Nunca un
   documento sin cursivas ni una web con la fuente de respaldo.
5. **Ningún contrato lleva un nombre de fuente en `string`.** `pdf` v0.1.0 cerró ese
   agujero; nada puede reabrirlo.
6. **`grep -rn "Int(16)\|ParseUint" pdf/ css/`** no encuentra ningún parser de hex: queda
   uno, en `color`. ✅ *cerrado*
7. **Un binario que importa `chart/bar` no contiene símbolos de `pie` ni `line`** —
   comprobado con `go tool nm`. ✅ *cerrado*
8. **El test ácido:** un agente sin contexto, guiado sólo por autocompletado, declara una
   tipografía y produce texto acentuado correcto en ambos medios.

---

## 5. Riesgos abiertos

**Consumidores con el patrón viejo.** `pdf` rompió API pública sin retrocompatibilidad
—deliberado: cada símbolo muerto pesa en el binario WASM—. Siguen sin migrar:

- `veltylabs/cotizaciones/print/doc.go`
- `veltylabs/contracts/print/doc.go`

Ambos con cuatro `RegisterFontStyle` apuntando `"I"`/`"BI"` a los archivos rectos, así que
hoy **no tienen cursiva y no lo saben**. Cuando `pdf` v0.1.2 borre los fallbacks, esto
dejará de ser silencioso — que es el objetivo.

**`cmd/demo/` de `pdf` está en `.gitignore`** (patrón `demo`), así que no viaja en el
repositorio y ningún plan lo alcanza; ya se rompió dos veces sin que ningún PR lo notara.
Si se quiere que sea la prueba viva de la API, hay que sacarlo del ignore. Decisión
pendiente.

**`color/docs/PLAN.md` quedó sin borrar**, con `STATUS: running` y `SESSION`, pese a que
su PR está fusionado y v0.1.1 publicado. Por convención el plan se borra en el commit que
publica el trabajo; hay que limpiarlo o el próximo `codejob` sobre ese repo leerá un
estado falso.
