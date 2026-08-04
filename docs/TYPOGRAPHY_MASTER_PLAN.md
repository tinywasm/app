# MASTER PLAN — Una decisión, un dueño: tipografía, color y gráficos

Multi-repo coordination hub. Empezó como «que la web y el PDF usen la misma
tipografía» y al tirar del hilo apareció el patrón de fondo: **decisiones sin
pieza que las posea**. La tipografía se escribía dos veces, el color se parsea
dos veces, y los gráficos viven dentro del generador de PDF.

> Dispatch: 2026-08-03 · **Status: 🚧 EN CURSO** — 2 de 6 repos cerrados.
> Coordina: `color/docs/PLAN.md`, `pdf/docs/PLAN.md`, `css/docs/PLAN.md`,
> `chart/docs/PLAN.md`, `assetmin/docs/PLAN.md` y
> `font/docs/GOFONT_PLAN.md`.
> Relacionado: `CONSTRUCTION_HARNESS.md` (principios 1, 4, 5, 9).

---

## 1. Síntoma

Cuatro fallos que parecían independientes:

1. **La app no se ve igual en Safari iOS que en Chrome Android.** `system-ui`
   resuelve a tipografías distintas con métricas distintas.
2. **El PDF no se parece a la web.** Usaba DroidSans, sin glifo `€` —en un
   producto de cotizaciones— y sin cursiva real.
3. **`tinywasm/pdf` corrompía el texto en silencio.** `Señor` salía `SeÃ±or` y
   `WritePdf()` devolvía `nil`. **Cerrado en `pdf` v0.1.0.**
4. **Todo binario paga por los tres tipos de gráfico** aunque dibuje uno.

---

## 2. Causa común

**Una decisión que ninguna pieza posee acaba escrita en varios sitios, y nada
impide que diverjan.**

| Decisión | Dónde se escribía | Consecuencia |
|---|---|---|
| Qué tipografía usa el producto | literal en `css` + rutas en `pdf` | web y PDF divergen |
| Cómo se lee un color hex | `pdf/color.go` + `css/contrast_test.go` | dos implementaciones del mismo cálculo |
| Cómo se dibuja un gráfico | dentro de `pdf` | se enlazan los tres tipos siempre |

La corrección es la misma en los tres casos: **una pieza dueña, y los demás
consumen**.

---

## 3. La partición WASM

`tinywasm/widget` es el precedente: sin un solo build tag, identidad pura, cruza
a WASM. `tinywasm/css` es lo contrario: `!wasm` entero, valores nunca identidad.

| | `widget` | `css` | `font` | `color` |
|---|---|---|---|---|
| Build tag | ninguno | `!wasm` | **ninguno** | **ninguno** |
| Contenido | identidad | valores | identidad | valores pequeños |
| ¿Cruza a WASM? | sí | no | **sí** | **sí** |

`color` cruza porque un color **es** un valor que el frontend maneja: lo dibuja
en un PDF generado en el navegador. `css` no cruza porque sus valores sólo
sirven para emitir texto CSS en build-time.

**Regla transversal:** si un archivo de `font` o `color` necesita un build tag, o
si aparece un `[]byte`, la pieza está mal partida.

---

## 4. Los repos

| # | Repo | Alcance | Plan | Estado |
|---|---|---|---|---|
| — | `tinywasm/font` | Identidad tipográfica: `Family`, `Style`, `Face`, `Declaration` | — | ✅ **v0.0.3 publicado** |
| — | `tinywasm/pdf` | Harness de tipografía; `coreFonts` e `internal/` fuera; −12.082 líneas | — | ✅ **v0.1.0 publicado** |
| 1 | `tinywasm/color` | `Color` + `RGB()` movidos de `pdf`; falta migrar consumidores y recoger `Luminance`/`Contrast` de `css` | `color/docs/PLAN.md` | 🚧 v0.0.1, código base listo |
| 2 | `tinywasm/pdf` | Consumir `font` (`LoadDeclared`), borrar el linaje Type1 residual, exportar el `Canvas`, soltar los gráficos, docs y demo | `pdf/docs/PLAN.md` | 📝 planificado |
| 2 | `tinywasm/css` | `--font-sans` como token alimentado por `font.Family` | `css/docs/PLAN.md` | 📝 planificado |
| 3 | `tinywasm/chart` | Contrato en la raíz, un subpaquete por tipo | `chart/docs/PLAN.md` | 🚧 v0.0.1, código movido sin compilar |
| 4 | `tinywasm/assetmin` | `FontProcessor` + el `@font-face` con sus URLs | `assetmin/docs/PLAN.md` | 📝 planificado |
| — | `tinywasm/font` | `cmd/gofont` para recortar caras | `font/docs/GOFONT_PLAN.md` | 📝 opcional |

**Orden:** `color` primero (los demás pueden depender de él) → `pdf` y `css` en
paralelo → `chart` (necesita el `Canvas` que expone `pdf`) → `assetmin`.

`tinywasm/ssr` **no participa.** Se evaluó añadirle un productor `RenderFonts()`
y se descartó: `config/fonts.go` y `config/css.go` son el mismo paquete Go, así
que `RootCSS()` llama a `Fonts()` directamente y el valor viaja dentro de la
extracción que ya existe. Su `docs/PLAN.md` debe borrarse.

---

## 5. Decisiones tomadas, con su evidencia

### 5.1 Roboto, subseteada, estática

| 4 caras, TTF subseteado | Crudo | Servido (brotli) |
|---|---|---|
| **Roboto** | 115.096 B | **69.978 B** |
| Inter | 209.288 B | 95.725 B |

Inter era la más liviana mientras el TTF se recortaba *sin* tablas de layout
—cuando sólo servía al PDF, que las ignora—. Al conservarlas para que el
navegador aplique kerning, sus tablas OpenType la ponen un 37% por encima.
Roboto además es lo que Android ya renderiza de forma nativa.

**Estática, nunca variable ni OTF.** El motor lee `glyf`/`loca`: ignora los ejes
de variación y no carga CFF. Verificado sobre `Inter-Regular.otf` (`OTTO`, sin
`glyf`). Las caras verificadas viven en `font/faces/`.

### 5.2 Un solo formato: TTF para web y PDF

| | Bytes que viajan | Peticiones |
|---|---|---|
| **Un TTF compartido** (brotli) | **16.132 B** | **1** |
| Un WOFF2 + un TTF | 27.890 B | 2 |

El PDF se genera **en el frontend**, así que pide el mismo archivo que la página
ya bajó para su `@font-face`: acierto de caché, no petición nueva. El navegador
lee TTF (`format("truetype")`); el motor de PDF no lee WOFF2.

> Una medición anterior daba 14.354 B para el TTF compartido. Era errónea:
> recortaba un archivo **ya recortado**, y `pyftsubset` no avisa de eso.

### 5.3 `config/fonts.go` no lleva `!wasm`

Un archivo `!wasm` no se compila para el navegador, y el PDF se genera ahí: ese
código necesita saber que la familia es `"Roboto"`. Marcarlo lo dejaría fuera del
alcance de su propio consumidor. Una declaración es identidad, e identidad cruza.

Su vecino `config/css.go` **sí** lleva el tag, porque devuelve valores. Ambos son
el mismo paquete Go, así que `RootCSS()` llama a `Fonts()` sin intermediarios.

### 5.4 El `@font-face` es de `assetmin`

`css` no puede escribir la URL: no conoce `AssetsURLPrefix` ni `OutputDir`.
`assetmin` los decide. Esto **corrige** la primera versión de `css/docs/PLAN.md`,
que proponía embeber la fuente en base64 dentro del CSS para esquivar el hueco de
`assetmin` — literalmente *a fork with a friendlier name*.

### 5.5 `Theme` no es color

Se queda en `pdf`. Lleva `Sizes`, `Spacing`, `Page` y `Margin` en milímetros: es
el tema de un documento. Que contenga colores no lo convierte en uno.

---

## 6. Criterios de cierre

1. **Un proyecto declara su tipografía en un único sitio** (`config/fonts.go`), y
   web y PDF la usan sin que nadie repita el nombre.
2. **`grep -rn "Int(16)\|ParseUint" pdf/ css/`** no encuentra ningún parser de
   hex: queda uno, en `color`.
3. **Un binario que importa `chart/bar` no contiene símbolos de `pie` ni
   `line`** — comprobado con `go tool nm`, no por inspección de imports.
4. **`web/public/client.wasm` baja de 3.785.258 B.** Entre el linaje Type1
   residual y los 398 KB de gráficos hay margen real; si no baja, algo no se fue.
5. **La página carga la fuente sin una sola petición externa.** Es el requisito
   de producto: los despliegues objetivo no tienen internet.
6. **Ningún contrato nuevo lleva un nombre de fuente en `string`.** `pdf` v0.1.0
   cerró ese agujero; el `Canvas` de los gráficos no puede reabrirlo.
7. **El test ácido:** un agente sin contexto, guiado sólo por autocompletado,
   declara una tipografía y produce texto acentuado correcto en ambos medios.

---

## 7. Riesgo abierto

`pdf` rompe API pública sin retrocompatibilidad —deliberado: cada símbolo muerto
pesa en el binario WASM—. Consumidores conocidos:

- `veltylabs/cotizaciones/print/doc.go`
- `veltylabs/contracts/print/doc.go`

Ambos con el patrón viejo de cuatro `RegisterFontStyle` apuntando `"I"`/`"BI"` a
los archivos rectos, así que hoy **no tienen cursiva y no lo saben**.

`cmd/demo/` de `pdf` está en `.gitignore` (patrón `demo`), así que no viaja en el
repositorio y ningún plan lo alcanza. Si se quiere que sea la prueba viva de la
API, hay que sacarlo del ignore primero — decisión pendiente.
