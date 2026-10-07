# Changelog

Cambios notables del proyecto. Formato basado en [Keep a Changelog](https://keepachangelog.com/es-ES/1.1.0/). Las especificaciones usan versionado documental (ver `CONTRIBUTING.md`); el producto usará [SemVer](https://semver.org/lang/es/) cuando exista código.

## [Unreleased]

### Cambiado

- **Nombre del producto: Hilvan** (D-025, spec 001 v1.1): paquete y comando `hilvan` (alias `hv`), directorio de configuración `hilvan/`, estado de proyecto `.hilvan/`, variable `HILVAN_CONFIG_DIR`.

### Añadido

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
| 2026-10-06 | 001 — Requisitos de producto | 1.1 | Approved | Nombre Hilvan (D-025, resuelve Q-011). |
| 2026-10-06 | 001 — Requisitos de producto | 1.0 | Approved | Baseline: decisiones D-013–D-024, FR-015–FR-019, NFR-010–NFR-012, AC-018–AC-021; Perplexity diferido. |
| 2026-10-06 | 001 — Requisitos de producto | 0.3 | Draft | D-011, D-012; FR-013, FR-014; NFR-008, NFR-009; AC-015–AC-017; Q-009, Q-010 (destinos y adaptadores de ejecución, Windows/WSL). |
| — | 001 — Requisitos de producto | 0.2 | Draft | D-007–D-010; FR-011, FR-012; configuración dinámica de aplicaciones/perfiles y elegibilidad antes de recomendar. |
| — | 001 — Requisitos de producto | 0.1 | Draft | Versión inicial. |
