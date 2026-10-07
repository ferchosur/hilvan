# Panorama de orquestadores y gestores de cuentas (actualización)

- **Fecha de consulta:** 2026-10-07
- **Propósito:** ampliar la investigación del 2026-10-06 con las herramientas más citadas en 2026, ver qué copiar y qué evitar, y confirmar la diferenciación de Hilvan.
- **Fuentes:**
  - [Morph — Parallel Coding Agents in 2026](https://www.morphllm.com/parallel-coding-agents)
  - [Superset vs Conductor vs Emdash](https://www.morphllm.com/comparisons/superset-vs-conductor-vs-emdash)
  - [Superset — Conductor vs Crystal](https://superset.sh/compare/conductor-vs-crystal)
  - [AgentsRoom — best multi-agent tools](https://agentsroom.dev/blog/best-multi-agent-coding-tools)
  - [ComposioHQ Agent Orchestrator (docs)](https://mintlify.com/ComposioHQ/agent-orchestrator/introduction)
  - [Landscape Scion/CAO/Composio](https://codex.danielvaughan.com/2026/04/17/agent-orchestrator-landscape-scion-cao-composio/)
  - [CCS — Claude Code Switch](https://github.com/kaitranntt/ccs)
  - [docs CCS](https://docs.ccs.kaitran.ca/)
  - [ccprof](https://docs.rs/crate/ccprof/0.2.0)
- **Nota:** la mayoría de las fuentes son comparativas de proveedores del sector y pueden tener sesgo comercial.

## 1. Herramientas

| Herramienta | Tipo / plataforma | Licencia | Aislamiento | Qué destaca | Varias cuentas / gobierno |
|---|---|---|---|---|---|
| **Conductor** (Melty Labs) | App nativa macOS | Propietaria (gratis) | Worktree por agente | Revisión, fusión y flujo de PR; reutiliza tus suscripciones | No |
| **Crystal → Nimbalyst** | Electron multiplataforma | MIT | Worktree | Commit por iteración, diff en la app, rebase, squash y merge integrados | No |
| **Emdash / Superset** | Escritorio (ADE) | Libre / mixta | Worktree | Agnósticos del agente; paneles por agente | No |
| **Claude Squad** | Terminal (tmux) | AGPL | Worktree | Mínimo y para la terminal; Windows con problemas (#275) | No |
| **vibe-kanban** | Web local | Apache-2.0 | Worktree | Kanban de tareas para agentes; piden limpieza automática (#765) y SDD (#1848) | No |
| **ComposioHQ Agent Orchestrator** | Web local (localhost:3000) | Abierto | Worktree o clon, tmux o Docker | **8 puntos de extensión** (Runtime, Agent, Workspace, Tracker, SCM, Notifier, Terminal, Lifecycle); **"reacciones"**: si falla la CI vuelve al agente, si piden cambios se escala a los 30 min, si está aprobado y en verde avisa a una persona; tareas desde issues de GitHub y Linear | No. **Usa el mismo nombre que nuestro nombre de trabajo** (confirmado: hicimos bien en renombrar a Hilvan) |
| **Orcha** | Web/escritorio/móvil | Abierto | Worktree (recomendado tras incidentes) | Tareas, dependencias, mensajería, métricas, supervisión humana | No |
| **herdr** | Multiplexor de terminal (Rust) | Apache-2.0 | Terminales persistentes | Estados working/blocked/idle, reconexión, API por socket | No |
| **CCS (Claude Code Switch)** | CLI + dashboard React + barra de menú macOS | MIT (2.9k ★) | Directorio por cuenta, con "grupos de contexto compartido" opcionales | Cambio de perfil en < 1 s, analítica de uso y coste, salud de proveedores, **proxy local compatible con Anthropic** que lleva Claude Code a otros proveedores mediante OAuth (CLIProxyAPI) | Sí para cuentas; **no** documenta políticas, clasificación, aprobación de traspasos ni auditoría |
| **ccprof / cc-quick-profile** | CLI / bandeja del sistema | Abiertos | Intercambian `settings.json` | Cambio rápido de configuración | Mínimo |

## 2. Qué confirma esto sobre Hilvan

- **Worktree por tarea** es el estándar del sector. Hilvan ya lo hace (ADR-0008), con commit por ejecución como Crystal.
- **Nadie combina** cuentas múltiples como identidades **con** política de datos, aprobación del contexto exacto, auditoría verificable y recomendación explicable. CCS es el más cercano en cuentas, pero se centra en cambiar perfiles y enrutar proveedores, no en gobierno ni continuidad de tareas.
- **Precaución con el enfoque de proxy:** enrutar tokens OAuth de suscripción mediante un proxy local roza lo que prohíben los términos de Anthropic (no intermediar credenciales ni enrutar en nombre de usuarios). Hilvan mantiene el modelo seguro: **CLI oficial sin modificar, sin tocar tokens** (NFR-010).
- **Windows** sigue mal resuelto en varias herramientas (Conductor solo funciona en macOS; claude-squad tiene incidencias abiertas). La CI de Hilvan en Windows y sus lanzadores WSL/Windows son una ventaja real si se validan con el spike 001.

## 3. Ideas para incorporar (backlog propuesto, después del MVP)

| Idea | Origen | Encaje en Hilvan |
|---|---|---|
| **Reacciones con escalado** (CI falla → reenviar al agente; sin respuesta → escalar) | ComposioHQ | Reglas de Criterio + política: reintentar solo con **el mismo perfil** y con un límite. Cambiar de perfil sigue exigiendo traspaso aprobado. |
| **Tareas desde issues** de GitHub, GitLab o Linear | ComposioHQ, vibe-kanban | Importar el brief desde un issue (`hilvan task new --from-issue`), con clasificación por repositorio. |
| **Notificaciones** (escritorio, webhook, Slack) | ComposioHQ, herdr | Al pasar a `blocked`, `uncertain` o `in_review`. Opcional y local. |
| **Puntos de extensión documentados** | ComposioHQ | Hilvan ya tiene Strategy para adapters y launchers; documentar un contrato de plugins cuando haya demanda. |
| **Analítica de uso por perfil** (tendencia y coste acumulado) | CCS | `hilvan stats` sobre `runs/` y la auditoría; solo con datos que reporta el CLI. |
| **Limpieza automática de worktrees y hook de preparación** | vibe-kanban #765, claude-squad #260 | Ya diseñado (FR-018); el hook de preparación queda pendiente de implementar. |
| **Revisión en la app con marcas de riesgo** | Crystal/Nimbalyst, Conductor | M5 (`hilvan review`) y ADR-0012 (web). |

## 4. Qué evitar

- Proxies o intermediación de credenciales de suscripción (riesgo de términos de uso).
- Bucles autónomos sin puertas humanas en cambios de identidad o de datos.
- Dependencias pesadas de escritorio (Electron) para la experiencia básica.
- Exclusividad de plataforma (solo macOS).
