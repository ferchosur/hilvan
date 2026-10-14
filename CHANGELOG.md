# Changelog

Cambios notables del proyecto. Formato basado en [Keep a Changelog](https://keepachangelog.com/es-ES/1.1.0/). Las especificaciones usan versionado documental (ver `CONTRIBUTING.md`); el producto usará [SemVer](https://semver.org/lang/es/) cuando exista código.

## [Unreleased]

### Publicación

- Repositorio público y release **0.1.0** (2026-10-11): binarios para linux, darwin y windows en amd64 y arm64, `SHA256SUMS` y attestations de procedencia (SR-029, ADR-0014). Firma de `hilvan.exe` aplazada.

### Añadido
- Búsqueda web en tareas de lectura (SR-038, D-041): `WebSearch` y `WebFetch` por defecto para tareas `public` y `personal`. Configurable por perfil con `tools.web` y `tools.webClassifications`.

- Watchdog de inactividad: una ejecución de agente sin salida durante 20 minutos (`--idle-timeout`) se cancela y queda como `cancelled` con el motivo `inactivity`. Cada chunk de salida reinicia el contador. La revisión cruzada tiene además un límite duro de 30 minutos (`review --timeout`), y `task run` mantiene 60 minutos (`--timeout`). El silencio máximo de cada ejecución se guarda en `maxSilenceSeconds` para decidir el valor con datos.
- `hilvan watch`: panel en vivo y de solo lectura con las tareas del proyecto y la ejecución en curso (ADR-0012). `--once` imprime un solo cuadro; sin terminal, se comporta igual. Nunca escribe en `.hilvan/`.

### Documentación

- Especificación 002 (borrador 0.1): chat conversacional con perfil intérprete, lista cerrada de operaciones y confirmación obligatoria. Pendiente de aprobación (D-037 a D-040).
- Especificación 001 **v1.5**: decisiones D-027–D-036 (perfil ocupado, contenido de la revisión, límites de ejecución, modelo al reanudar, lectura en tareas de Claude, mods, monetización, firma de Windows, spec 002 y `ui`), NFR-014 (consumo) y SR-037 (comandos de solo lectura). Sin cambios de alcance del MVP.
- ADR-0012 aceptado con la adaptación a Go (2026-10-11); la verificación de SR-032…036 se hizo en Node y debe repetirse en Go al implementar `ui`.
- Nota de investigación sobre AgentBridge: ideas reutilizables y límites para Hilvan ([docs/research/2026-10-12-agentbridge.md](docs/research/2026-10-12-agentbridge.md)).
- Se registra el diagnóstico del 2026-10-10: verificación local en Go 1.27.1, cobertura por paquete, brechas de documentación y prioridades para cerrar AC-015 y preparar SR-029. Véase [bitácora](docs/sessions/2026-10-10-diagnostico-del-proyecto.md).
- Documentación alineada con spec 001 v1.4 y 23/25 AC: estado de ambos README, control de hooks y MCP en el modelo de amenazas (T4, SR-011/012, ADR-0013) y distribución de la release 0.1.0 en GitHub Releases en lugar de npm (ADR-0014).

### Corregido

- **Log de ejecución:** cada línea del agente se guarda una sola vez (antes la línea `result` se duplicaba y las líneas sin eventos no se guardaban). Detectado por la prueba diferencial; corregido en TS y en Go.
- **`hilvan accept`** ya no falla en equipos sin identidad de git (detectado por la primera CI).

### Cambiado

- `task show` muestra el resultado de la última ejecución (antes solo sus métricas). El texto guardado pasa de 4000 a 50 000 caracteres; el registro de la ejecución conserva el flujo completo.
- Tareas de solo lectura con Claude: lista explícita de herramientas (`Read`, `Glob`, `Grep`, `ls`, `git ls-files`) en lugar de bloquear también la lectura (D-031). Verificado con la CLI real: `Write` y `Bash` de escritura se bloquean (SR-037).
- Perfil ocupado: un perfil que ya ejecuta una tarea, en cualquier proyecto, rechaza una nueva ejecución con un mensaje claro (D-027).
- Revisión cruzada: el transcript completo queda en el registro local de la ejecución y la tarea guarda solo un resumen de 4000 caracteres (D-028).
- El modelo efectivo se fija al primer uso y se reutiliza al reanudar (D-030); `task.json` lo guarda en `model`.
- **Hilvan es ahora un binario de Go** (plan 002 completado, G5): se retira la implementación TypeScript, la CI solo usa Go (ubuntu y windows, `govulncheck`) y Dependabot vigila los módulos de Go. Instalación: `go build -o ~/.local/bin/hilvan ./cmd/hilvan`. E2E real con Claude Code y Codex; AC-025 verificado de nuevo; 23/25 AC con Go; `hilvan --version` en ~3 ms y ~5 MB, presupuesto medido como test.
- **Migración a Go** (D-026, spec 001 v1.4, ADR-0014 sustituye a ADR-0002, [plan 002](docs/plans/002-migracion-go.md)). G0: esqueleto `cmd/hilvan` (`--version` en 1,4 ms y 13 MB, frente a 164 ms y 66 MB), comando `go run ./scripts/check`, job de Go en CI (ubuntu y windows, `govulncheck`) y spike 006 de paridad (0 discrepancias). La implementación TypeScript queda congelada como referencia hasta G5.
- **Migración a Go, G1:** núcleo puro portado (`internal/domain`, `security`, `jsfmt`, `criterio`, `policy`, `config`) con casos dorados generados por el código TS (`scripts/parity/gen-g1.ts`): 2.507 operaciones de Criterio reproducidas, `task.json` idéntico byte a byte y modelos aprendidos compatibles.
- **Migración a Go, G2:** procesos sin shell (con `.cmd` de Windows protegido contra BatBadBut), lanzadores con traducción de rutas Windows↔WSL y `PATH` explícito para WSL, salida UTF-16 de `wsl.exe`, almacén del proyecto, auditoría compatible con la cadena escrita por TS y adaptadores de Claude Code y Codex, con paridad verificada (`scripts/parity/gen-g2.ts`).
- **Migración a Go, G3:** worktrees con rutas relativas (usables desde Windows y WSL, spike 001), identidad de git de respaldo y núcleo completo: tareas, perfiles (con descubrimiento del ejecutable y el `PATH` en WSL), recomendación, traspaso, revisión cruzada, aceptación y uso por perfil. Los tests del ciclo de vida con repos git reales también corren en Windows.
- **Migración a Go, G4:** CLI completa en Go (`internal/cli`), sin dependencias de terceros, con la misma ayuda, mensajes y códigos de salida. Prueba diferencial TS frente a Go (61/61 pasos idénticos) en un nuevo job de CI `parity`. Nuevo: `profile add --launcher wsl` descubre el ejecutable y el `PATH` en la distro; `profile set --rediscover` los actualiza tras actualizar Node.
- **Nombre del producto: Hilvan** (D-025, spec 001 v1.1): paquete y comando `hilvan` (alias `hv`), directorio de configuración `hilvan/`, estado de proyecto `.hilvan/`, variable `HILVAN_CONFIG_DIR`.

### Añadido

- **Herramientas por cuenta y perfil reservado** (FR-021, ADR-0013, spec 001 v1.3):
  - `routing: reserved`, para que la cuenta corporativa solo reciba tareas que necesitan sus herramientas;
  - Criterio infiere las herramientas requeridas a partir del brief (alias predefinidos, alias del perfil y aprendizaje), con confirmación;
  - `hilvan task requires` y aviso en `task recommend`;
  - `hilvan profile tools [--apply]` descubre los MCP de la cuenta;
  - `hilvan profile set` y opciones `--routing`, `--mcp`, `--alias`.
- **Seguridad:** Claude Code se lanza con `--setting-sources user`. Verificado en real: el hook y el MCP hostiles del repositorio no se ejecutan, y los conectores de la cuenta siguen disponibles. Sustituye a `--strict-mcp-config` y a la confianza por hash (SR-011, SR-012).
- **`hilvan stats`:** uso por perfil en el proyecto con solo lo que reportan los CLIs: ejecuciones por estado, coste estimado, tokens, últimos límites, aceptadas y descartadas; incluye la calibración de Criterio. Incluye el consumo de las revisiones cruzadas.
- **Hook de preparación por worktree** (FR-018): `hilvan init --setup "npm ci"` se ejecuta sin shell, con timeout y auditoría, en cada worktree nuevo.
- **M5 — revisión, revisión cruzada y aceptación:**
  - `hilvan review <tarea>` muestra el diff saneado, el riesgo según Criterio y las rutas sensibles marcadas, y registra el *head* revisado;
  - `hilvan review <tarea> --by <perfil>` pide una revisión de solo lectura a otro perfil, con política y aprobación, y adjunta su veredicto (`VERDICT: approve|changes_requested`) sin cambiar la identidad de la tarea;
  - `hilvan accept` exige haber revisado el diff actual, un checkout limpio y estar en la rama base; fusiona con `--no-ff`, limpia el worktree y la rama, y Criterio aprende (Beta por perfil y tipo, y modelo de aceptación);
  - `task discard` enseña el rechazo.

  E2E real: Claude, como revisor, rechazó la respuesta alucinada de Codex.
- **M4 — traspaso entre perfiles:**
  - `hilvan handoff <tarea> --to <perfil> [--edit] [--approve] [--print]`;
  - el paquete de continuidad se arma con lo que Hilvan observó: estado, ejecuciones, commits y archivos de la rama, y las afirmaciones del agente anterior **marcadas como no verificadas**. Funciona aunque el perfil de origen esté bloqueado por límite;
  - secretos redactados, edición en `$EDITOR`, aprobación explícita, y se guarda y audita exactamente lo aprobado (sha256);
  - la política (incluidas las reglas obligatorias) se evalúa entre el perfil de origen y el de destino;
  - el nuevo perfil recibe el paquete en su prompt y nunca reanuda la sesión de otra identidad.
- **M3 — elegibilidad, política y recomendación:**
  - política en capas: gestionada, de usuario y de proyecto;
  - reglas `deny` y `require_approval` con selectores por clasificación, perfil, etiqueta y aplicación;
  - `enforced` solo en la capa gestionada, y el proyecto solo puede restringir;
  - evaluación con motivos visibles y aprobación obligatoria que muestra el brief exacto antes de enviarlo (sin aprobación no se envía nada);
  - `hilvan task recommend` (Criterio ordena solo los perfiles elegibles; el margen de cuota sale de los límites reportados);
  - comandos `hilvan policy show|check` y `hilvan task run --approve`.
- **M2 — tareas y ejecución:**
  - comandos `hilvan init` y `hilvan task new|list|show|log|run|cancel|discard`;
  - brief estructurado, con sugerencias de Criterio para tipo, complejidad y endurecimiento de la clasificación;
  - ejecución con los adaptadores claude-code (`-p` stream-json, `--strict-mcp-config`, permisos según el modo) y codex (`exec --json`, sandbox según el modo), con el prompt por stdin;
  - eventos normalizados y estado final decidido por los eventos: `completed`, `blocked` (límite, autenticación o facturación), `failed`, `cancelled` o `uncertain`;
  - worktree y rama `hilvan/<tarea>` por tarea de escritura, con commit automático en la rama de la tarea y diff medido;
  - riesgo del diff y probabilidad de cumplir el brief estimados por Criterio;
  - consumo y uso de las ventanas de límite visibles en cada ejecución;
  - bloqueo de una ejecución activa por proyecto, con recuperación de ejecuciones interrumpidas (pasan a `uncertain`);
  - Ctrl+C y `--timeout` cancelan la ejecución;
  - logs de ejecución con secretos redactados y auditoría con cadena de hash;
  - aviso y confianza explícita para los hooks del repositorio (SR-012).

  Verificado con un agente simulado y con ejecuciones reales de Claude Code y Codex.
- **Rendimiento:** carga perezosa de comandos y caché de compilación (`hilvan --version` pasa de 298 a 149 ms). Spike 003 con benchmark en Node, Go, Python y Java (Q-013).
- **Seguridad:**
  - [análisis de riesgos](docs/security/analisis-de-riesgos.md): 25 amenazas STRIDE con matriz de riesgo;
  - [requisitos SR-001…SR-031](docs/security/requisitos.md) trazados a pruebas;
  - saneado de secuencias ANSI, OSC, bidi y de ancho cero en la salida a terminal;
  - detector y redactor de secretos;
  - lectura y escritura seguras (sin symlinks, `O_NOFOLLOW`, `O_EXCL`, 0600/0700, límites de tamaño);
  - límites en YAML (alias, claves únicas, tamaño) y en flujos JSONL;
  - argumentos sin saltos de línea ni bytes NUL;
  - auditoría con cadena de hash y `hilvan audit verify`;
  - `doctor` avisa de permisos laxos;
  - CI con acciones fijadas por SHA, permisos mínimos, `--ignore-scripts` y `npm audit`;
  - Dependabot y `SECURITY.md`.
- **Criterio** (ADR-0011, FR-020, spec 001 v1.2): motor de decisión local al estilo JEV, sin red ni dependencias. Responde preguntas tipadas (tipo de tarea, complejidad, sensibilidad de los datos, riesgo del diff, cumplimiento del brief) con confianza y explicación. Combina reglas en español e inglés con regresión logística en línea, ordena perfiles con Beta y muestreo de Thompson, decide por umbrales si actuar solo, confirmar o escalar, y mide su calibración con Brier. Unos 30 µs por decisión. Comandos `hilvan criterio ask|teach|stats|reset`.
- **M1 — perfiles y destinos (Linux verificado):** `hilvan profile add|list|show|remove|login|check`; adaptadores claude-code y codex (aislamiento de entorno, login oficial, estado de autenticación); lanzadores `native`, `wsl` y `windows` (estos dos con tests unitarios, pendientes de verificar en Windows); `profiles.yaml` con escritura atómica (0600/0700); detección de directorios compartidos, de cuentas repetidas entre perfiles y de claves de API heredadas.
- **M0 — esqueleto del CLI `hilvan`:** proyecto TypeScript (Node ≥ 22.12, ESM), Biome, Vitest, build con `tsc`, CI en Ubuntu y Windows con Node 22 y 24; esquemas zod de perfil, destino, tarea y máquina de estados; rutas de configuración por SO; carga validada de `profiles.yaml`; comando `hilvan doctor`.
- **Especificación 001 v1.0 aprobada** (baseline): resuelve Q-001–Q-010 (D-013–D-024); FR-015–FR-019 (revisión cruzada, consumo visible, estados bloqueado/incierto, worktree por tarea, brief); NFR-010–NFR-012; AC-018–AC-021; R-006–R-008.
- ADR-0002 a ADR-0010: stack, superficie, adaptadores, lanzadores, identidad, persistencia, worktrees, políticas y licencia.
- Diseño: arquitectura, modelo de datos, contrato de adaptador y modelo de amenazas.
- Plan 001 del MVP (hitos M0–M5).
- Investigación de foros, herramientas similares y términos de uso (2026-10-06).
- Spike 002 (invocación sin interfaz) completado, con fixtures reales; spike 001 iniciado.
- Licencia Apache-2.0 y DCO.
- Espacio de trabajo profesional: `AGENTS.md`, `CLAUDE.md`, `CONTRIBUTING.md`, `CHANGELOG.md`, glosario, jerarquía `docs/` (adr, design, plans, research, sessions), `spikes/`, plantillas de spec, ADR y spike.
- Skills de Claude Code: `/retomar`, `/cerrar-sesion`, `/nueva-adr`, `/nuevo-spike`.
- ADR-0001: método SDD y registro de decisiones.
- Repositorio git con convenciones de commits y ramas.

### Cambiado

- `docs/estado-y-oportunidades.md` movido a `docs/research/`; enlaces actualizados.

## Historial de especificaciones

| Fecha | Documento | Versión | Estado | Cambios |
|---|---|---|---|---|
| 2026-10-07 | 001 — Requisitos de producto | 1.4 | Approved | D-026: Go como lenguaje (resuelve Q-013); D-013 obsoleta. ADR-0014. |
| 2026-10-07 | 001 — Requisitos de producto | 1.3 | Approved | FR-021 (herramientas por cuenta, perfil reservado e inferencia de herramientas), AC-023–AC-025. ADR-0013. |
| 2026-10-07 | 001 — Requisitos de producto | 1.2 | Approved | FR-020 (Criterio), NFR-013 (ligereza), AC-022; FR-012 con regla de cuotas por aplicación. ADR-0011. |
| 2026-10-06 | 001 — Requisitos de producto | 1.1 | Approved | Nombre Hilvan (D-025, resuelve Q-011). |
| 2026-10-06 | 001 — Requisitos de producto | 1.0 | Approved | Baseline: decisiones D-013–D-024, FR-015–FR-019, NFR-010–NFR-012, AC-018–AC-021; Perplexity diferido. |
| 2026-10-06 | 001 — Requisitos de producto | 0.3 | Draft | D-011, D-012; FR-013, FR-014; NFR-008, NFR-009; AC-015–AC-017; Q-009, Q-010 (destinos y adaptadores de ejecución, Windows/WSL). |
| — | 001 — Requisitos de producto | 0.2 | Draft | D-007–D-010; FR-011, FR-012; configuración dinámica de aplicaciones/perfiles y elegibilidad antes de recomendar. |
| — | 001 — Requisitos de producto | 0.1 | Draft | Versión inicial. |
