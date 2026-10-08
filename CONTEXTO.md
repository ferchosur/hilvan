# Contexto para continuar el proyecto

Última actualización: 2026-10-07
Estado: **MVP completo en Linux (M0–M5, Criterio y FR-021)**, con 168 tests y e2e real con Claude Code y Codex. **23 de 25 criterios de aceptación verificados** ([matriz](docs/plans/verificacion-mvp.md)); AC-015 está pendiente de Windows/WSL y AC-009 diferido. Especificación 001 **v1.3** (etiqueta `spec-001-v1.3`). Hay decisiones del usuario pendientes (ver abajo).

Fuentes de verdad:

- Requisitos: [`specs/001-product-requirements.md`](specs/001-product-requirements.md) (v1.2).
- Decisiones: [`docs/adr/`](docs/adr/).
- Diseño: [`docs/design/`](docs/design/).
- Seguridad: [`docs/security/`](docs/security/).
- Plan: [`docs/plans/001-mvp.md`](docs/plans/001-mvp.md).
- Historia: [`docs/sessions/`](docs/sessions/).

## Producto

**Hilvan** (D-025): CLI open source (Apache-2.0) para coordinar agentes de IA entre proveedores y entre varias cuentas, con continuidad mediante contexto revisable. Cada cuenta es una identidad aislada; nada cambia de identidad sin el usuario. **Se posiciona como continuidad y gobierno, nunca como forma de sumar o eludir cuotas.** Debe funcionar en casi cualquier PC: ligero, sin modelos locales ni terceros (NFR-013).

Entornos del usuario:

- **PC del trabajo (Windows + WSL):** Claude corporativo, Claude personal y Codex personal.
- **Este Linux:** Claude y Codex (reutilizando los logins por defecto).

## Qué funciona hoy (`hilvan …`)

| Comando | Qué hace |
|---|---|
| `doctor` | Comprueba Node, git, claude, codex y WSL, y avisa de permisos laxos en la configuración. |
| `profile add/list/show/remove/login/check` | Un perfil por cuenta y entorno (`CLAUDE_CONFIG_DIR`/`CODEX_HOME`, o `configDir: null` = login por defecto), login oficial y verificación de la cuenta activa. |
| `init` | Crea `.hilvan/` en un repositorio git (ignorado por git). |
| `task new/list/show/log/run/cancel/discard/recommend` | Brief → Criterio sugiere tipo, complejidad y clasificación → ejecución en un worktree `hilvan/<tarea>` → consumo, límites, riesgo del diff y cumplimiento del brief → estados `blocked`/`uncertain` sin reintentos silenciosos. |
| `policy show/check` | Política en capas (gestionada, usuario, proyecto) con `deny`/`require_approval`; `enforced` solo en la capa gestionada; la aprobación muestra el brief exacto. |
| `criterio ask/teach/stats/reset` | Motor de decisión local (~30 µs/decisión) que aprende de las correcciones. |
| `handoff <tarea> --to <perfil>` | Paquete de continuidad (estado, ejecuciones, commits, archivos y afirmaciones *no verificadas*), con secretos redactados, editable, aprobado y guardado con sha256. Funciona aunque el origen esté bloqueado por límite. |
| `review <tarea> [--by <perfil>]` / `accept` | Diff saneado con el riesgo según Criterio; revisión cruzada de solo lectura con veredicto; aceptar exige el diff revisado y fusiona con `--no-ff`; Criterio aprende de aceptar y descartar. |
| `profile tools/set`, `task requires` | Descubre los MCP de cada cuenta. `routing: reserved` hace que la cuenta corporativa solo reciba tareas que necesitan sus herramientas. Criterio infiere las herramientas requeridas del brief y aprende de tus confirmaciones. |
| `stats` | Uso por perfil (coste estimado, tokens, últimos límites, resultados) y calibración de Criterio. |
| `audit verify` | Verifica la cadena de hash de `.hilvan/events.jsonl`. |

Calidad: **168 tests**, TypeScript estricto, Biome y CI en Linux y Windows. Arranque de `--version`: 149 ms (antes 298).

## Cómo se usa en este PC

- `hilvan` y `hv` están instalados como enlaces en `~/.local/bin` que apuntan a `dist/cli/main.js` (sin sudo; el prefijo global de npm es `/usr/local`). **Tras cambiar código: `npm run build`.**
- Configuración real del usuario: `~/.config/hilvan/` (vacía: aún no hay perfiles creados en este PC). Las pruebas e2e usaron carpetas temporales del scratchpad, que ya no existen.
- Regla de enrutamiento del usuario: la cuenta Claude **corporativa** se usa **solo** para tareas que necesitan sus herramientas o MCP internos (`routing: reserved`); todo lo demás va a la cuenta **personal**.
- Comprobación antes de cada commit o fusión: `npm run check` debe terminar con `rc=0`. No filtres su salida con tuberías: ya ocultó un fallo una vez.

## Decisiones vigentes (detalle en spec y ADR)

TypeScript/Node ≥ 22 (ADR-0002, **en revisión por Q-013**) · CLI primero (0003) · adaptadores de proceso no interactivo (0004) · un orquestador con lanzadores native/wsl/windows (0005) · identidad por directorio de configuración del CLI oficial, sin tocar tokens (0006) · persistencia en archivos (0007) · worktree por tarea y ejecución secuencial (0008) · política en capas y aprobación (0009) · Apache-2.0 (0010) · Criterio (0011) · UI: TUI propia y web mínima (0012, **Proposed**) · herramientas de la cuenta, perfil reservado e inferencia de herramientas, lanzamiento con `--setting-sources user` (0013).

## Hallazgos técnicos clave

- `claude -p` reporta el uso de las ventanas `five_hour` y `seven_day` (`rate_limit_event`). `claude auth status --json` identifica la cuenta. `codex login status` solo indica si hay sesión.
- Claude sale con 0 tras SIGINT, así que el estado se decide por los eventos. Hay que cerrar stdin. `CODEX_HOME` debe existir y estar fuera de `/tmp`.
- **Codex tiene un servidor en segundo plano** (`codex app-server --managed-daemon`, en `~/.codex/packages/app-server-daemon/`) que se actualiza solo. El 2026-10-07 iba por la 0.161.0 mientras el CLI de npm seguía en la 0.160.1, y el CLI interactivo mostró *Background server has incompatible feature settings* con un menú de tres opciones. No lo causó Hilvan: el servidor usa `~/.codex` y no un `CODEX_HOME` de prueba.
- **El login por defecto de Claude guarda la cuenta en `~/.claude.json`.** Reutilizarlo exige *eliminar* `CLAUDE_CONFIG_DIR`, no apuntarla a `~/.claude`.
- **E2E 2026-10-07:** en una tarea de solo lectura, Codex respondió sin leer el archivo y describió un README inexistente (alucinación). **La revisión cruzada real de Claude lo detectó** (`VERDICT: changes_requested`), lo que demuestra el valor de FR-015.
- **Spike 003 (rendimiento):** Go arranca en 1 ms y usa 15 MB, frente a ~145 ms y 60–80 MB en Node; además genera un binario sin runtime. Python es lento en CPU y Java pesado.
- **ADR-0013:** `--setting-sources user` bloquea los hooks y el MCP del repositorio (verificado) y conserva los conectores de la cuenta. `--strict-mcp-config` no bloqueaba los hooks y apagaba las herramientas de la cuenta.
- **Spike 004:** web local en vivo con `node:http` + SSE y cero dependencias; 9 de 9 controles de seguridad verificados.

## Decisiones pendientes del usuario

1. **Q-013 — ¿Migrar a Go?** Recomendación técnica: sí, por NFR-013 y para facilitar la instalación. El código TypeScript y sus tests servirían de especificación. Requiere instalar Go ≥ 1.23 (aquí hay 1.17).
2. **ADR-0012 — UI:** aceptar `hilvan watch` (TUI propia) y `hilvan ui` (web local mínima).
3. **Spike 001 en el PC del trabajo:** verificar el aislamiento de cuentas en Windows y WSL (procedimiento en `spikes/001-auth-aislada/`). Comprobar también que `hilvan profile tools claude-corp` ve los MCP de la organización y que siguen activos con `--setting-sources user`; después, `hilvan profile tools claude-corp --apply` y `hilvan profile set claude-corp --routing reserved`.
4. **Q-012 — Monetización** (después del MVP).

## Próximos pasos

1. Que el usuario decida Q-013 (Go) y ADR-0012 (UI).
2. Validar en el PC del trabajo (spike 001): perfiles en Windows y WSL (AC-015).
3. **Pendiente:** comprobar si `codex exec` (el adaptador de Hilvan) choca con ese servidor cuando las versiones del CLI y del servidor no coinciden. En modo no interactivo nadie puede elegir una opción del menú, así que la ejecución podría fallar o colgarse. Si ocurre, el adaptador debe detectarlo y dejar la tarea en `blocked` con un mensaje claro.
4. Mejoras: resumen estructurado pedido al agente de origen en el traspaso (`--json-schema`/`--output-schema`), catálogo de mensajes es/en, `hilvan stats`, `hilvan watch`/`ui` (si se acepta ADR-0012) y lanzador SSH para las dos máquinas.
5. Release 0.1.0: README final, empaquetado (bundle) y npm con provenance (SR-029).

## Referencias

- [Investigación de foros y herramientas](docs/research/2026-10-06-foros-y-herramientas.md) · [UI terminal y web](docs/research/2026-10-07-ui-terminal-y-web.md) · [estado y oportunidades](docs/research/estado-y-oportunidades.md)
- [Análisis de riesgos](docs/security/analisis-de-riesgos.md) · [Requisitos SR](docs/security/requisitos.md) · [Patrones](docs/design/patrones.md)
- [Glosario](docs/glosario.md) · [Bitácora](docs/sessions/)
- Clones de referencia: `../orcha` (`b647cab`) y `../herdr`.
