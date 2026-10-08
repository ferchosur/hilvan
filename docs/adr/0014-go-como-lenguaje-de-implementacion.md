# ADR-0014 — Go como lenguaje de implementación

- **Estado:** Accepted
- **Fecha:** 2026-10-07
- **Decisores:** Fernando Giraldo (aprobación), Claude Code (propuesta como arquitecto)
- **Relacionado:** Q-013, D-026, NFR-012, NFR-013, SR-004, SR-026, SR-028, SR-029, [spike 003](../../spikes/003-rendimiento/), [plan 002](../plans/002-migracion-go.md). Sustituye a [ADR-0002](0002-typescript-node-y-contrato-por-stdio.md).

## Contexto

ADR-0002 eligió TypeScript sobre Node.js por afinidad con los CLIs objetivo y por tener una sola cadena de herramientas. Después se aprobó NFR-013: Hilvan debe funcionar en equipos modestos, con presupuestos medidos de arranque y memoria.

El spike 003 midió el arranque de cada lenguaje:

| | Arranque | Memoria |
|---|---|---|
| Go | ~1 ms | 15 MB |
| Node | ~132 ms | 58 MB |
| CLI real de Hilvan en TypeScript, ya optimizado | 149 ms | 66 MB |

Go produce además un binario estático sin runtime, y su CPU fue la mejor en la carga de Criterio. El usuario necesita instalar Hilvan en un equipo de validación con Windows y WSL, donde no depender de Node simplifica mucho la instalación.

El MVP está completo en TypeScript, pero no hay release ni usuarios. Es el momento más barato para cambiar.

## Opciones consideradas

1. **Seguir en TypeScript** y optimizar (empaquetado, carga perezosa). Se probó: el techo está en ~143 ms y ~59 MB, limitado por el propio Node.
2. **Go para todo el producto.** Binario único, buen control de procesos y multiplataforma nativa. Cuesta una reescritura de ~6.500 líneas y ~2.200 de tests.
3. **Híbrido** (núcleo en Go y CLI en TS, o al revés). Dos cadenas de herramientas y dos CI, y no elimina la dependencia de Node.

## Decisión

**Go (versión estable ≥ 1.23; desarrollo con 1.27) para todo el producto**, con un binario `hilvan` (y `hv` como alias del mismo binario).

La migración sigue el [plan 002](../plans/002-migracion-go.md) bajo cuatro condiciones:

- **Paridad, no rediseño:** mismos comandos, flags, salidas y formatos en disco (`docs/design/modelo-de-datos.md`).
- **El código TypeScript se congela** y sirve de especificación ejecutable hasta el corte.
- **Paridad verificada** con tests dorados y una prueba diferencial TS frente a Go.
- **Criterio conserva sus modelos aprendidos:** mismo *feature hashing* FNV-1a sobre unidades UTF-16 y misma tokenización.

**Dependencias.** Biblioteca estándar por defecto. Cada dependencia externa se justifica:

| Necesidad | Elección | Motivo |
|---|---|---|
| YAML (`profiles.yaml`, `policy.yaml`) | `go.yaml.in/yaml/v3` | `gopkg.in/yaml.v3` se archivó el 2025-04-01. Esta es su continuación compatible, mantenida por la organización YAML ([fuente](https://github.com/Go-yaml/yaml), consultada el 2026-10-07). |
| Normalización Unicode (NFD) para Criterio | `golang.org/x/text/unicode/norm` | Proyecto oficial de Go. Reimplementarlo sería frágil. |
| Subcomandos y flags (sustituye a commander) | Se decide en G4 entre `spf13/cobra` y un despachador propio sobre `flag` | Hay que admitir flags después de los argumentos posicionales y flags repetibles, como hoy. |
| Preguntas interactivas (sustituye a @clack/prompts) | Implementación propia, por líneas | Funciona en Windows, WSL y por SSH sin modo *raw*. Coherente con ADR-0012. |
| Validación (sustituye a zod) | Funciones `Validate()` explícitas por tipo | Mismos límites y mensajes, sin reflexión. |

**Seguridad de procesos en Windows (SR-004).** `cross-spawn` escapa hoy los argumentos al lanzar *shims* `.cmd` de npm, como `codex.cmd`. `os/exec` de Go **no** lo hace con archivos por lotes: ante BatBadBut (CVE-2024-24576), Go solo actualizó su documentación ([fuente](https://www.securityweek.com/batbadbut-command-injection-vulnerability-affects-multiple-programming-languages/), consultada el 2026-10-07). Por eso, en G2, el lanzador de Windows debe hacer una de estas dos cosas:

- resolver el *shim* de npm a `node <script>` y no pasar nunca por `cmd.exe`;
- o, si el destino es `.cmd`/`.bat`, rechazar los argumentos con metacaracteres de `cmd.exe` (`% ! ^ & | < > " ( )`).

En ambos casos debe haber tests en la CI de Windows. El prompt sigue viajando por stdin.

**Distribución** (se implementa en la Release 0.1.0):

- binarios por plataforma (linux, windows y darwin; amd64 y arm64) en GitHub Releases, con *attestations* de procedencia de GitHub en lugar del provenance de npm (SR-029) y sumas sha256;
- `go install` para quien tenga Go;
- un paquete npm envoltorio con binarios por plataforma es opcional y se decidirá en la release.

**Herramientas:** `gofmt`, `go vet`, `go test` y `govulncheck` en CI (ubuntu y windows, acciones fijadas por SHA, SR-028), reunidos en un único comando de comprobación que sustituye a `npm run check`.

El contrato JSON normalizado entre núcleo y adaptadores de ADR-0002 se mantiene.

## Consecuencias

**Positivas:**

- Arranque y memoria muy por debajo de los presupuestos de NFR-013.
- Instalación sin Node: un solo archivo.
- Compilación cruzada trivial para Windows y WSL.
- Las extensiones futuras (ver la investigación sobre mods) se harán como procesos externos que hablan JSON, lo que encaja con Go.

**Negativas:**

- Semanas sin funciones nuevas.
- Riesgo de regresiones, mitigado con la paridad verificada.
- Se rehace la distribución y el provenance.
- Los colaboradores necesitan Go.
- El Agent SDK de Claude no tiene versión en Go. Impacto bajo, porque ADR-0004 usa los CLIs como procesos.

**Seguimiento:**

- Re-verificar los 23 AC ya verificados con el binario de Go (G5).
- Medir los presupuestos de NFR-013 como tests.
- Retirar `src/`, `tests/` y `package.json` al cortar.
