# Plan 002 — Migración a Go

- **Estado:** Accepted
- **Fecha:** 2026-10-07
- **Origen:** [spec 001 v1.4](../../specs/001-product-requirements.md) (D-026) · [ADR-0014](../adr/0014-go-como-lenguaje-de-implementacion.md) · [spike 003](../../spikes/003-rendimiento/) · [spike 006](../../spikes/006-paridad-go/)
- **Aprobado por / fecha:** Fernando Giraldo, 2026-10-07

| Hito | Estado |
|---|---|
| G0 — Decisión y esqueleto | ✔ 2026-10-07 (CI de Go pendiente de ejecutarse en GitHub: el repositorio aún no tiene remoto) |
| G1 — Núcleo puro | ✔ 2026-10-07: `domain`, `security`, `jsfmt`, `criterio`, `policy`, `config`. Paridad: 2.507 operaciones de Criterio iguales a TS (tolerancia 1e-9; pesos de `model.json` idénticos), `task.json` byte a byte, ULID, tokenización, FNV-1a y JSON canónico |
| G2–G5 | Pendientes |

## Contexto

El usuario confirmó el 2026-10-07 **Q-013 = Go**. El spike 003 midió para Go un arranque de ~1 ms frente a ~145 ms en Node, 15 MB frente a 60–80 MB de memoria y un binario sin runtime. Eso cumple mucho mejor NFR-013 (ligereza) y simplifica la instalación en el PC del trabajo (Windows + WSL). Es el momento más barato para migrar: el MVP está completo (168 tests, 23/25 AC) pero aún no hay release, paquete publicado ni usuarios. La configuración real de este PC (`~/.config/hilvan/`) está vacía.

**Resultado buscado:** el mismo producto, con los mismos comandos, flags, salidas y formatos en disco, reimplementado en Go con paridad verificada. Después se retira el código TypeScript.

## Principios

1. **Paridad, no rediseño.** No hay funciones nuevas durante la migración. Las mejoras pendientes (comprobación del servidor de Codex, requisito "no aumentar el consumo", spike 005) se implementan después, ya en Go.
2. **Formatos en disco idénticos** (`docs/design/modelo-de-datos.md`): `profiles.yaml`, `policy.yaml`, `.hilvan/project.json`, `task.json`, `events.jsonl` (cadena de hash), paquetes de traspaso con sha256 y estado de Criterio (`version: 1`). Así se pueden comparar ambas versiones sobre los mismos datos y no hace falta migrar nada.
3. **El TypeScript es la especificación ejecutable.** Se congela en `main` (solo correcciones) hasta que Go lo sustituya. Sus tests y los fixtures reales de `tests/fixtures/` se portan.
4. **Mínimas dependencias** (NFR-013, SR-026): biblioteca estándar siempre que sea razonable. Cada dependencia externa se justifica en el ADR.

## Trampas de paridad detectadas (requieren tests dorados)

- **Cadena de auditoría:** `canonical()` (`src/store/audit.ts:30`) usa `JSON.stringify` de JS. `encoding/json` de Go escapa `<`, `>`, `&`, U+2028 y U+2029 y formatea los números de otra manera. Hay que escribir un serializador canónico propio y comprobar que Go **verifica un `events.jsonl` generado por TS**.
- **Feature hashing de Criterio:** `fnv1a` (`src/criterio/features.ts:13`) recorre **unidades UTF-16** (`charCodeAt`), no bytes ni runas. Go debe usar `utf16.Encode`; si no, los pesos aprendidos dejan de corresponder. Lo mismo con `tokenize`: NFD + eliminación de `\p{M}` (`golang.org/x/text/unicode/norm`, o una tabla propia si se quiere cero dependencias).
- **Coma flotante:** la regresión logística y la softmax pueden diferir en ±1 ULP (`Math.exp` frente a `math.Exp`). Las comparaciones se hacen con tolerancia y la decisión final debe coincidir.
- **Windows:** `cross-spawn` resuelve los *shims* `.cmd` de npm (`claude.cmd`, `codex.cmd`). En Go, ejecutar `.cmd` con argumentos tiene implicaciones de seguridad (BatBadBut, CVE-2024-24576). Hay que verificar el comportamiento de `os/exec` en la versión elegida.
- **Validación:** zod se sustituye por funciones `Validate()` explícitas por tipo, con los mismos límites (tamaños máximos, regex de ids, límites del estado de Criterio).

## Estructura propuesta

```
go.mod                      # module hilvan (provisional; ruta definitiva al publicar el repositorio)
cmd/hilvan/main.go          # binario `hilvan`; `hv` = mismo binario (enlace o copia)
internal/domain  security  config  store  criterio  policy
internal/process  launchers  adapters  gitops  core  cli
internal/testutil/fakeagent # port de tests/fixtures/fake-agent.mjs (sin Node en los tests de Go)
testdata/                   # fixtures reales (copiados de tests/fixtures) y archivos dorados
scripts/parity/             # comparación diferencial TS frente a Go (temporal)
```

Se mantiene el mismo reparto de módulos que `docs/design/arquitectura.md`. `domain/` sigue sin E/S.

## Hitos (rama `feat/go`; cada hito termina con `go vet`, `gofmt`, tests y CI en ubuntu y windows en verde)

| Hito | Entrega | Verificación |
|---|---|---|
| **G0 — Decisión y esqueleto** | D-026 en la spec (v1.4, Q-013 resuelta), **ADR-0014** "Go como lenguaje de implementación", que sustituye a ADR-0002 (que pasa a `Superseded`). Go estable ≥ 1.23 instalado sin sudo en `~/.local/go`. `go.mod`, `cmd/hilvan` con `--version`, CI con Go en ubuntu y windows (acciones fijadas por SHA, SR-028) y `govulncheck`. Mini-spike de riesgos: `.cmd` en Windows, biblioteca YAML y serializador canónico. Comando de comprobación único (`go run ./scripts/check` o similar) que sustituye a `npm run check` en AGENTS.md. | `hilvan --version` < 5 ms; CI en verde. |
| **G1 — Núcleo puro** | `domain`, `security` (secretos, saneado, fs seguro), `config` (rutas por SO, perfiles, política), `criterio` completo, `policy/evaluate`. | Port de `domain`, `config`, `security`, `criterio` y `policy`. Tests dorados: `fnv1a`/`tokenize` y decisiones de Criterio sobre un corpus generado con TS. |
| **G2 — Persistencia y procesos** | `store` (proyecto, tareas, auditoría con cadena compatible), `process` (líneas, ejecución con cancelación y SIGINT), `launchers` (native, wsl, windows), `adapters` (claude-code, codex, parsers, runner). | Port de `adapters`, `adapters-run`, `launchers` y la parte de store. Go verifica un `events.jsonl` de TS. Los fixtures reales de Claude y Codex se parsean igual. |
| **G3 — Orquestación** | `gitops` (worktrees y ramas `hilvan/<tarea>`), `core` (tasks, profiles, recommend, handoff, review, stats). | Port de `tasks`, `profiles`, `handoff`, `review` y `tools-routing`, usando el agente falso en Go. |
| **G4 — CLI** | Todos los comandos con los mismos flags y textos: `doctor`, `profile *`, `init`, `task *`, `policy *`, `criterio *`, `handoff`, `review`, `accept`, `stats`, `audit verify`. Preguntas interactivas (sustituyen a @clack/prompts) y `$EDITOR`. | Port de `doctor` y `config`. **Prueba diferencial** (`scripts/parity`): los mismos escenarios con el agente falso en TS y en Go deben dar la misma salida (normalizando ids y fechas) y los mismos archivos en `.hilvan/`. |
| **G5 — Corte** | E2E real en Linux con Claude Code y Codex (recorrido de demostración del plan 001). Presupuestos de rendimiento como tests (arranque y memoria, NFR-013). Eliminar `src/`, `tests/`, `package.json` y la configuración de Node. Actualizar `arquitectura.md`, `AGENTS.md` (convenciones de código y comprobación), README (instalación con `go build` / binario en `~/.local/bin`), `CONTEXTO.md`, `CHANGELOG.md` y la matriz `verificacion-mvp.md` (re-verificar los 23 AC). Fusionar en `main`. | `verificacion-mvp.md` re-verificada; `hilvan`/`hv` en `~/.local/bin` apuntan al binario de Go. |

La **distribución** (GitHub Releases con binarios por plataforma, attestations de procedencia en sustitución del provenance de npm de SR-029, y opcionalmente un paquete npm envoltorio con binarios por plataforma) se decide en ADR-0014, pero se implementa en la **Release 0.1.0**, fuera de este plan.

## Archivos críticos

- Nuevos: `docs/adr/0014-go-como-lenguaje-de-implementacion.md`, `docs/plans/002-migracion-go.md` (este plan, versionado), `go.mod`, `cmd/hilvan/`, `internal/**`, `.github/workflows/ci.yml` (job de Go).
- Modificados: `specs/001-product-requirements.md` (D-026, v1.4, historial), `docs/adr/0002-…` (estado `Superseded by ADR-0014`), `docs/adr/README.md`, `docs/plans/README.md`, `AGENTS.md`, `CONTEXTO.md`, `CHANGELOG.md`, `docs/design/arquitectura.md` (en G5).
- Referencia de comportamiento (no se modifica hasta G5): `src/**` y `tests/**`, en especial `src/store/audit.ts`, `src/criterio/features.ts`, `src/adapters/parsers.ts`, `src/core/tasks.ts` y `tests/fixtures/`.

## Primer paso tras aprobar

Solo G0, en la rama `feat/go`: spec v1.4 con D-026, ADR-0014, `docs/plans/002-migracion-go.md`, instalación de Go, esqueleto y CI, y el mini-spike de riesgos. Cada hito se presenta antes de continuar con el siguiente.

## Verificación global

- Por hito: `go vet ./...`, `gofmt -l` vacío, `go test ./...` y CI en ubuntu y windows.
- Paridad: tests dorados (G1, G2) y la prueba diferencial TS frente a Go (G4) sin diferencias.
- Al final: los 23 AC verificados de nuevo con el binario de Go, e2e real con Claude Code y Codex, y `hilvan --version` medido (objetivo < 5 ms frente a 149 ms).
