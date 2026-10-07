# Patrones de diseño y calidad

- **Estado:** Accepted
- **Fecha:** 2026-10-07
- **Propósito:** explicar qué patrones usa Hilvan, por qué y dónde, para que cualquier contribución (persona o agente) los mantenga.

## 1. Arquitectura

| Patrón | Dónde | Por qué |
|---|---|---|
| **Hexagonal (puertos y adaptadores)** | `core/` orquesta; `adapters/`, `launchers/`, `gitops/`, `store/` y `config/` son los adaptadores; `cli/` es solo presentación. | El dominio no depende de Claude, Codex, git ni del sistema de archivos. La futura UI web y un posible port a Go reutilizan los mismos casos de uso. |
| **Núcleo puro** | `domain/`, `criterio/`, `launchers/`, `adapters/parsers.ts`, `adapters/events.ts` | Sin E/S: se prueban en microsegundos, de forma determinista y en cualquier SO. Los launchers de Windows y WSL se prueban en Linux. |
| **Strategy** | `AgentAdapter` por aplicación; `buildInvocation` por lanzador; reglas, modelo y mezcla en Criterio. | Añadir una aplicación o un entorno no toca el núcleo (NFR-005). |
| **Repository** | `ProjectStore` (`.hilvan/`), `loadProfiles`/`saveProfiles`, `loadCriterio`/`saveCriterio` | Persistencia intercambiable: archivos hoy, SQLite mañana (ADR-0007). |
| **Registro de solo añadido + cadena de hash** | `store/audit.ts` | Auditoría con evidencia de manipulación (SR-017) sin infraestructura. |
| **Inyección de dependencias por objetos `deps`** | `DoctorDeps`, `CheckDeps`, `RunDeps`, `Git` | Los tests sustituyen procesos, git y el sistema de archivos sin mocks globales. |
| **Máquina de estados explícita** | `domain/task.ts` (`canTransition`) | Impide transiciones peligrosas; por ejemplo, `blocked` nunca pasa a `running` sin decisión del usuario (AC-007). |
| **Decisión por eventos, no por código de salida** | `adapters/events.ts#decideOutcome` | Los CLIs salen con 0 tras SIGINT (spike 002). El estado se deriva de lo observado. |

## 2. Rendimiento (NFR-013)

| Patrón | Dónde | Efecto medido |
|---|---|---|
| **Carga perezosa por grupo de comandos** | `cli/index.ts` (`GROUPS`, `selectGroups`) | `hilvan --version`: 298 → 149 ms. Cada grupo solo importa lo suyo. |
| **Caché de compilación de V8** | `cli/main.ts` (`module.enableCompileCache`) | Reutiliza el bytecode entre ejecuciones. |
| **Streaming con backpressure y límites** | `process/lines.ts`, `adapters/runner.ts` | Memoria acotada con salidas enormes de agentes (SR-009). |
| **Feature hashing y modelos dispersos** | `criterio/features.ts`, `logistic.ts` | ~30 µs por decisión; el modelo pesa decenas de KB y no guarda texto. |
| **Lectura del final del log sin cargarlo entero** | `store/audit.ts#lastHash` | Añadir un evento cuesta O(1) aunque el log crezca. |
| **Presupuesto de dependencias** | `package.json` (5 de ejecución) | Cada dependencia nueva debe justificarse con su coste de arranque (ver spike 003). |

## 3. Seguridad

| Patrón | Dónde |
|---|---|
| **Sin shell, argumentos en array, prompt por stdin** | `process/run.ts`, `adapters/runner.ts`, `security/args.ts` |
| **Validar en los bordes** (zod para configuración, estado y modelo; límites de tamaño) | `config/`, `store/project.ts`, `config/criterio-store.ts` |
| **E/S segura por defecto** (`O_NOFOLLOW`, `O_EXCL`, escritura atómica, 0600/0700) | `security/fs.ts` |
| **Sanear al mostrar, redactar al guardar** | `security/sanitize.ts` (terminal), `security/secrets.ts` (logs y briefs) |
| **Mínimo privilegio por perfil** (entorno aislado, MCP estricto, sandbox o permisos según el modo) | `adapters/*` |
| **Las acciones sensibles nunca son automáticas** | `criterio/thresholds.ts` |

## 4. Usabilidad

- Cada comando termina indicando el **siguiente paso** (`next: …`).
- **IDs cortos**: los últimos 6 caracteres del ULID se resuelven con `resolveTaskId`.
- **Explicaciones** de Criterio: cada sugerencia muestra sus señales y su confianza.
- **Errores accionables**: dicen qué hacer, por ejemplo `run hilvan profile login`, `chmod 600` o `--trust-repo-hooks`.
- Salida legible por defecto y **`--json`** para automatizar.
- **Pendiente:** catálogo de mensajes es/en (hoy el CLI está en inglés y las explicaciones de Criterio en español), `hilvan watch` (ADR-0012) y autocompletado de shell.

## 5. Calidad del desarrollo

- **TypeScript estricto** (`noUncheckedIndexedAccess`, `exactOptionalPropertyTypes`) y **Biome** (lint y formato) en `npm run check`.
- **Tests con fixtures reales** de los CLIs (spike 002) y un **agente simulado** (`tests/fixtures/fake-agent.mjs`) para escenarios difíciles: límites, autenticación, caídas, cancelación, salida enorme y timeout.
- **Pruebas e2e reales** opcionales con Claude Code y Codex, que consumen poca cuota.
- **SDD**: cada cambio de comportamiento se traza a FR, AC o SR. Los ADR registran el porqué.
- **CI** en Linux y Windows con Node 22 y 24, acciones fijadas por SHA y Dependabot.
