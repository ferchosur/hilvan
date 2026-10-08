# Patrones de diseño y calidad

- **Estado:** Accepted
- **Fecha:** 2026-10-07
- **Propósito:** explicar qué patrones usa Hilvan, por qué y dónde, para que cualquier contribución (persona o agente) los mantenga.

## 1. Arquitectura

| Patrón | Dónde | Por qué |
|---|---|---|
| **Hexagonal (puertos y adaptadores)** | `core/` orquesta; `adapters/`, `launchers/`, `gitops/`, `store/` y `config/` son los adaptadores; `cli/` es solo presentación. | El dominio no depende de Claude, Codex, git ni del sistema de archivos. La futura UI web y un posible port a Go reutilizan los mismos casos de uso. |
| **Núcleo puro** | `domain/`, `criterio/`, `launchers/`, `adapters/parsers.go`, `adapters/events.go`, `jsfmt/` | Sin E/S: se prueban en microsegundos, de forma determinista y en cualquier SO. Los launchers de Windows y WSL se prueban en Linux. |
| **Strategy** | `AgentAdapter` por aplicación; `buildInvocation` por lanzador; reglas, modelo y mezcla en Criterio. | Añadir una aplicación o un entorno no toca el núcleo (NFR-005). |
| **Repository** | `ProjectStore` (`.hilvan/`), `loadProfiles`/`saveProfiles`, `loadCriterio`/`saveCriterio` | Persistencia intercambiable: archivos hoy, SQLite mañana (ADR-0007). |
| **Registro de solo añadido + cadena de hash** | `store/audit.go` (JSON canónico compatible con la versión TS) | Auditoría con evidencia de manipulación (SR-017) sin infraestructura. |
| **Inyección de dependencias** | `CheckDeps`, `RunDeps`, `gitops.Git`, `cli.App` | Los tests sustituyen procesos, git y el sistema de archivos sin mocks globales. |
| **Máquina de estados explícita** | `domain/task.go` (`CanTransition`) | Impide transiciones peligrosas; por ejemplo, `blocked` nunca pasa a `running` sin decisión del usuario (AC-007). |
| **Decisión por eventos, no por código de salida** | `adapters/events.go` (`DecideOutcome`) | Los CLIs salen con 0 tras SIGINT (spike 002). El estado se deriva de lo observado. |

## 2. Rendimiento (NFR-013)

| Patrón | Dónde | Efecto medido |
|---|---|---|
| **Binario nativo sin runtime** (Go, ADR-0014) | `cmd/hilvan` | `hilvan --version`: ~3 ms y ~5 MB (antes 149 ms y 66 MB en Node). Presupuesto como test: `cmd/hilvan/budget_test.go`. |
| **Streaming con backpressure y límites** | `process/lines.go`, `adapters/runner.go` | Memoria acotada con salidas enormes de agentes (SR-009). |
| **Feature hashing y modelos dispersos** | `criterio/features.go`, `logistic.go` | ~30 µs por decisión; el modelo pesa decenas de KB y no guarda texto. |
| **Lectura del final del log sin cargarlo entero** | `store/audit.go` (`lastHash`) | Añadir un evento cuesta O(1) aunque el log crezca. |
| **Presupuesto de dependencias** | `go.mod` (2: YAML y `x/text`) | Cada dependencia nueva se justifica en un ADR (ADR-0014). |

## 3. Seguridad

| Patrón | Dónde |
|---|---|
| **Sin shell, argumentos en array, prompt por stdin** (y `.cmd` de Windows entrecomillados, BatBadBut) | `internal/process/`, `internal/adapters/runner.go`, `internal/security/args.go` |
| **Validar en los bordes** (`Validate()` explícito para configuración, estado y modelo; límites de tamaño) | `internal/domain/`, `internal/config/`, `internal/store/project.go`, `internal/criterio/state.go` |
| **E/S segura por defecto** (`O_NOFOLLOW`, `O_EXCL`, escritura atómica, 0600/0700) | `internal/security/fs*.go` |
| **Sanear al mostrar, redactar al guardar** | `internal/security/sanitize.go` (terminal), `internal/security/secrets.go` (logs y briefs) |
| **Mínimo privilegio por perfil** (entorno aislado, MCP estricto, sandbox o permisos según el modo) | `internal/adapters/` |
| **Las acciones sensibles nunca son automáticas** | `internal/criterio/thresholds.go` |

## 4. Usabilidad

- Cada comando termina indicando el **siguiente paso** (`next: …`).
- **IDs cortos**: los últimos 6 caracteres del ULID se resuelven con `resolveTaskId`.
- **Explicaciones** de Criterio: cada sugerencia muestra sus señales y su confianza.
- **Errores accionables**: dicen qué hacer, por ejemplo `run hilvan profile login`, `chmod 600` o `--trust-repo-hooks`.
- Salida legible por defecto y **`--json`** para automatizar.
- **Pendiente:** catálogo de mensajes es/en (hoy el CLI está en inglés y las explicaciones de Criterio en español), `hilvan watch` (ADR-0012) y autocompletado de shell.

## 5. Calidad del desarrollo

- **Go** con `gofmt`, `go vet` y tests en `go run ./scripts/check`; `govulncheck` en CI. Dependencias mínimas (ADR-0014).
- **Tests con fixtures reales** de los CLIs (spike 002) y un **agente simulado** (`internal/testutil`, el propio binario de test) para escenarios difíciles: límites, autenticación, caídas, cancelación, salida enorme y timeout. **Casos dorados** generados por la antigua versión TS para la paridad de datos (plan 002).
- **Pruebas e2e reales** opcionales con Claude Code y Codex, que consumen poca cuota.
- **SDD**: cada cambio de comportamiento se traza a FR, AC o SR. Los ADR registran el porqué.
- **CI** en Linux y Windows, acciones fijadas por SHA y Dependabot (módulos de Go y acciones).
