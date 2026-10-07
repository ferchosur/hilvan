# Investigación: foros, herramientas similares y viabilidad técnica

- **Fecha de consulta:** 2026-10-06
- **Propósito:** resolver Q-001–Q-010 de la [especificación 001](../../specs/001-product-requirements.md) con evidencia y detectar capacidades que hagan la herramienta más útil.
- **Método:** documentación oficial de proveedores, issues abiertos de repositorios similares y búsquedas en r/ClaudeCode y r/ClaudeAI (a través de un espejo de solo lectura, porque reddit.com no permitía acceso directo).
- **Límites:** las opiniones de los foros son anecdóticas y los votos indican interés, no representatividad. Las versiones de los CLIs cambian a menudo; revalidar antes de cada hito.

## 1. Términos de uso y autenticación (hallazgo determinante)

Fuente: [Claude Code — Legal and compliance](https://code.claude.com/docs/en/legal-and-compliance).

- La autenticación OAuth de los planes Free/Pro/Max/Team/Enterprise está destinada al "uso ordinario de Claude Code y otras aplicaciones nativas de Anthropic".
- Los terceros **no pueden** ofrecer el inicio de sesión de Claude.ai en sus aplicaciones, enrutar solicitudes con credenciales de suscripción en nombre de sus usuarios, ni recoger, almacenar o intermediar credenciales o tokens de sesión.
- **Sí está permitido** que el usuario final inicie sesión con su propia suscripción en el **binario de Claude Code sin modificar**, incluso cuando otra plataforma lo aloja o lo ejecuta.
- No se puede usar "Claude Code" ni "Anthropic" como parte del nombre de un producto.
- Los límites publicitados de Pro/Max suponen un uso individual y ordinario.

La prensa del sector informó que la aplicación técnica de esta restricción empezó en enero de 2026 ([AlternativeTo](https://alternativeto.net/news/2026/2/anthropic-officially-bans-using-subscription-authentication-for-third-party-claude-use), [WinBuzzer](https://winbuzzer.com/2026/02/19/anthropic-bans-claude-subscription-oauth-in-third-party-apps-xcxwbn/)). En foros se comenta que Anthropic asocia el uso a la máquina para impedir sumar cuentas.

**Implicación:** el orquestador solo puede **lanzar el CLI oficial** con un directorio de configuración por perfil. El login lo hace el usuario con el flujo de Anthropic y el orquestador nunca lee los tokens. El producto debe presentarse como herramienta de continuidad y gobierno, nunca como forma de sumar o eludir cuotas. Esto confirma D-007 y las cuotas separadas.

## 2. Aislamiento de varias cuentas en el mismo equipo

| Herramienta | Mecanismo | Evidencia |
|---|---|---|
| Claude Code | `CLAUDE_CONFIG_DIR` contiene credenciales, historial, ajustes y plugins | [fireup.pro](https://fireup.pro/news/multiple-claude-code-accounts), [agentsroom.dev](https://agentsroom.dev/blog/how-to-use-multiple-claude-code-accounts); herramientas comunitarias [ccswitch](https://pkg.go.dev/github.com/binoyPeries/ccswitch), [clauth](https://docs.rs/crate/clauth/0.7.3), [claude-account](https://pypi.org/project/claude-account/) |
| Codex CLI | `CODEX_HOME` contiene `auth.json`, `config.toml` e historial; `CODEX_API_KEY` para modo API | [Referencia de variables de Codex](https://codex.danielvaughan.com/2026/06/03/codex-cli-environment-variables-runtime-configuration-headless-ci-container-deployment/), [codex-profiles](https://community.openai.com/t/codex-profiles-switch-codex-accounts-without-copying-auth-json/1380415), [prodex](https://docs.rs/crate/prodex/0.2.37) |

Pendiente de verificar en el spike 001: el comando fiable para saber qué cuenta está activa en cada directorio y si el llavero del sistema (en macOS o Windows) rompe el aislamiento por directorio.

## 3. Ejecución no interactiva

### Claude Code (`claude -p`), versión 2.1.292 en este equipo

Fuente: [Run Claude Code programmatically](https://code.claude.com/docs/en/headless).

- `--output-format stream-json --verbose` produce eventos en JSON por líneas:
  - `system/init` informa el modelo, las herramientas, `mcp_servers` y `mcp_server_errors`, además de `capabilities`.
  - `system/api_retry` incluye `error` con valores como `rate_limit`, `authentication_failed`, `billing_error` u `overloaded`.
  - `result` incluye `session_id`, `total_cost_usd` (estimado en el cliente) y `permission_denials`.
- Opciones útiles:
  - `--resume <session_id>` continúa una sesión.
  - `--json-schema` pide una salida estructurada.
  - `--strict-mcp-config --mcp-config` limita los servidores MCP.
  - `--permission-mode` y `--permission-prompts none` controlan los permisos en ejecuciones desatendidas.
- Cancelación: SIGINT termina el turno; SIGTERM termina el proceso con código 143 y el turno queda sin resultado.
- **Riesgo:** sin `--bare`, `-p` ejecuta los hooks y el `.mcp.json` del repositorio sin pedir confianza. `--bare` no usa el login de suscripción, así que no sirve para perfiles con suscripción.

### Codex CLI (`codex exec`), versión 0.160.1 en este equipo

Fuente: [Codex non-interactive mode](https://learn.chatgpt.com/docs/non-interactive-mode).

- `--json` produce los eventos `thread.started`, `turn.started`, `turn.completed`, `turn.failed`, `item.*` y `error`, con uso de tokens.
- Opciones útiles:
  - `codex exec resume <id>` continúa una sesión.
  - `--cd` fija el directorio de trabajo.
  - `--sandbox` acepta `read-only` (predeterminado), `workspace-write` o `danger-full-access`.
  - `--output-schema` pide una salida estructurada.
  - `--ephemeral` no guarda la sesión.
  - `--ignore-user-config` ignora la configuración del usuario.

### Windows y WSL

`wsl.exe -d <distro> --cd <dir> -- <cmd>` transmite stdin, stdout y stderr entre Windows y Linux, así que un proceso Node en Windows puede lanzar agentes en WSL usando tuberías ([Microsoft — WSL interop](https://learn.microsoft.com/en-us/windows/dev-environment/wsl-interop)). La interoperación también funciona en sentido contrario, desde WSL hacia los ejecutables de Windows.

## 4. Agent Client Protocol (ACP)

Fuentes: [agentclientprotocol.com](https://agentclientprotocol.com/overview/introduction), [índice](https://agentclientprotocol.com/llms.txt), [registro de agentes](https://agentclientprotocol.com/registry).

- Estándar abierto, originado en Zed en agosto de 2025, para la comunicación entre un cliente o editor y un agente. Usa JSON-RPC sobre stdio en local y HTTP o WebSocket en remoto.
- Contempla sesiones (crear, retomar, listar, cerrar), envío de prompts, cancelación, solicitud de permisos, planes y elicitaciones. La versión 1 es estable y la versión 2 está en borrador.
- El registro incluye Claude Agent, Codex CLI, Cline y Amp, entre otros. Los adaptadores de Claude se basan en el Agent SDK.

**Implicación:** ACP sirve de modelo para nuestro contrato de adaptador y es buen candidato a "adaptador genérico" después del MVP. No conviene como base del MVP: el adaptador de Claude depende del Agent SDK (que Anthropic orienta a autenticación con API key) y la versión 2 aún cambia.

## 5. Foros: necesidades y patrones de los usuarios

Búsquedas en [r/ClaudeCode](https://redlib.groet-infra.nl/r/ClaudeCode/search?q=handoff+codex+context&restrict_sr=on&sort=top&t=year) y [r/ClaudeAI](https://redlib.groet-infra.nl/r/ClaudeAI/search?q=codex+handoff+context+limit&restrict_sr=on&sort=top&t=year).

| Tema | Lo que dicen los usuarios | Ejemplos (votos) |
|---|---|---|
| Continuidad | Perder el contexto al cambiar de herramienta o al llegar a un límite es el mayor dolor. Se resuelve a mano con archivos de traspaso (STATE, DECISIONS, FINDINGS) en disco en lugar de memoria compactada. | "How I use sub-agents without burning through Fable 5.1" (395), "How to refactor 50k lines…" (170) |
| Revisión cruzada | Modelos de distintos proveedores detectan los puntos ciegos del otro. El revisor escribe hallazgos en archivos. | "I made Claude Code and Codex talk to each other" (227) |
| Reparto por coste | Un orquestador caro decide y trabajadores baratos ejecutan o exploran. "La calidad del brief importa más que el modelo". | "Multiple AI coding agents in parallel against Linear tasks" (759) |
| Robustez | Bloqueos y expiración de tareas cuando un agente se cae; conflictos al tocar los mismos archivos; deriva sin supervisión; trabajos a medias. | mismo hilo |
| Transparencia de uso | El porcentaje de uso no se traduce en tokens y el panel tiene retraso. Piden ver el consumo en tiempo real. | megahilo de límites de r/ClaudeAI |
| Instrucciones compartidas | Claude Code admite `AGENTS.md` desde la versión 2.1.277, lo que facilita que Claude y Codex compartan instrucciones. | [codeongrass](https://codeongrass.com/blog/claude-code-vs-codex-heavy-users-limits-costs-switching/) |

Herramientas que surgen del mismo problema: [ai-handoff / claude-handoff-revive](https://github.com/sofumel/claude-handoff-revive) (traspaso de Claude a Codex) y el skill de handoff para Claude Code ([mejba.me](https://www.mejba.me/handoff-skill-claude-code-multi-session)).

## 6. Funcionalidades pendientes en repositorios similares

| Repositorio | Issues abiertos más votados y relevantes | Lección |
|---|---|---|
| [vibe-kanban](https://github.com/BloopAI/vibe-kanban/issues) | #765 limpieza automática de worktrees (14), #1848 soporte de desarrollo guiado por especificaciones (7), #3396 exportación (9) | Limpiar worktrees y soportar briefs o especificaciones son necesidades reales. |
| [claude-squad](https://github.com/smtg-ai/claude-squad/issues) | #56 varios repositorios (8), #275 binario de Windows roto (4), #260 hook de preparación por worktree (3) | Windows suele quedar mal resuelto: es una oportunidad. Hace falta un hook de preparación. |
| [Orcha](https://github.com/open-orcha/orcha/blob/main/docs/orcha-roadmap-and-findings.md) (clon local `b647cab`) | Compartir una sola copia de trabajo mezcló ramas; los agentes con worktree propio no se vieron afectados. Hay una puerta de verificación humana. | El aislamiento por worktree debe ser el comportamiento por defecto. |
| [herdr](https://herdr.dev) (clon local) | Estados por agente (trabajando, bloqueado, inactivo), reconexión, API por socket | Mostrar el estado de un vistazo es muy valorado. |

Panorama de orquestadores: [Tembo](https://www.tembo.io/blog/ai-agent-orchestration-tools), [OpenAlternative](https://openalternative.co/blog/best-ai-coding-agent-orchestrators), [Augment](https://augmentcode.com/tools/open-source-agent-orchestrators). La mayoría resuelve el paralelismo con worktrees; **ninguno de los revisados modela varias cuentas del mismo proveedor como identidades con políticas y herramientas propias**. Esa sigue siendo la diferenciación.

## 7. Nombre

Ya existe un producto llamado "Agent Orchestrator" en [Product Hunt](https://www.producthunt.com/p/agent-orchestrator/agent-orchestrator) y hay repositorios con el mismo nombre. Hay que elegir un nombre distinto antes de publicar (Q-011). Los términos de Anthropic impiden usar "Claude" en el nombre.

## 8. Conclusiones aplicadas

Estas conclusiones se registran como D-013 a D-024 en la especificación 001 v1.0 y como ADR-0002 a ADR-0010:

1. Ejecutar mediante el CLI oficial sin modificar, con un directorio de configuración por perfil y sin manejar tokens.
2. Usar adaptadores de proceso no interactivo (`claude -p`, `codex exec`) con un contrato propio inspirado en ACP.
3. Usar un solo proceso orquestador con lanzadores por destino: nativo, WSL y Windows (SSH queda para después).
4. Dar a cada tarea que escribe código su propio worktree, limpiarlo automáticamente y tratar la revisión como un diff.
5. Incorporar al MVP la revisión cruzada, el consumo visible, los estados bloqueado e incierto y los briefs de tarea.
6. Aplazar Perplexity: requiere créditos de API aparte de la suscripción, y el usuario no los tiene.
