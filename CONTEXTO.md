# Contexto para continuar el proyecto

Última actualización: 2026-10-07
Estado: **MVP en construcción. M0, M1 (Linux), Criterio, M2 y M3 terminados**, con e2e real verificado con Claude Code y Codex. Siguiente: **M4 (traspaso entre perfiles)** y **M5 (revisión y aceptación)**. Hay decisiones del usuario pendientes (ver abajo).

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
| `audit verify` | Verifica la cadena de hash de `.hilvan/events.jsonl`. |

Calidad: **144 tests**, TypeScript estricto, Biome y CI en Linux y Windows. Arranque de `--version`: 149 ms (antes 298).

## Decisiones vigentes (detalle en spec y ADR)

TypeScript/Node ≥ 22 (ADR-0002, **en revisión por Q-013**) · CLI primero (0003) · adaptadores de proceso no interactivo (0004) · un orquestador con lanzadores native/wsl/windows (0005) · identidad por directorio de configuración del CLI oficial, sin tocar tokens (0006) · persistencia en archivos (0007) · worktree por tarea y ejecución secuencial (0008) · política en capas y aprobación (0009) · Apache-2.0 (0010) · Criterio (0011) · UI: TUI propia y web mínima (0012, **Proposed**).

## Hallazgos técnicos clave

- `claude -p` reporta el uso de las ventanas `five_hour` y `seven_day` (`rate_limit_event`). `claude auth status --json` identifica la cuenta. `codex login status` solo indica si hay sesión.
- Claude sale con 0 tras SIGINT, así que el estado se decide por los eventos. Hay que cerrar stdin. `CODEX_HOME` debe existir y estar fuera de `/tmp`.
- **El login por defecto de Claude guarda la cuenta en `~/.claude.json`.** Reutilizarlo exige *eliminar* `CLAUDE_CONFIG_DIR`, no apuntarla a `~/.claude`.
- **E2E 2026-10-07:** en una tarea de solo lectura, Codex respondió sin leer el archivo y describió un README inexistente (alucinación). Es evidencia del valor de la revisión cruzada (FR-015, M5). Criterio no puede detectarlo por sí solo.
- **Spike 003 (rendimiento):** Go arranca en 1 ms y usa 15 MB, frente a ~145 ms y 60–80 MB en Node; además genera un binario sin runtime. Python es lento en CPU y Java pesado.
- **Spike 004:** web local en vivo con `node:http` + SSE y cero dependencias; 9 de 9 controles de seguridad verificados.

## Decisiones pendientes del usuario

1. **Q-013 — ¿Migrar a Go antes de M4?** Recomendación técnica: sí, por NFR-013 y para facilitar la instalación. El código TypeScript y sus tests servirían de especificación. Requiere instalar Go ≥ 1.23 (aquí hay 1.17).
2. **ADR-0012 — UI:** aceptar `hilvan watch` (TUI propia) y `hilvan ui` (web local mínima).
3. **Spike 001 en el PC del trabajo:** verificar el aislamiento de cuentas en Windows y WSL (procedimiento en `spikes/001-auth-aislada/`).
4. **Q-012 — Monetización** (después del MVP).

## Próximos pasos

1. Que el usuario decida Q-013 y ADR-0012.
2. **M4:** `hilvan handoff` con paquete de continuidad (resumen estructurado con `--json-schema`/`--output-schema` más hechos de git), detector de secretos, edición, aprobación y hash.
3. **M5:** `hilvan review` (diff), `--by` (revisión cruzada de solo lectura), `accept`/`discard`, resultados que alimentan a Criterio.
4. Catálogo de mensajes es/en, `hilvan watch` y release 0.1.0 (npm con provenance).

## Referencias

- [Investigación de foros y herramientas](docs/research/2026-10-06-foros-y-herramientas.md) · [UI terminal y web](docs/research/2026-10-07-ui-terminal-y-web.md) · [estado y oportunidades](docs/research/estado-y-oportunidades.md)
- [Análisis de riesgos](docs/security/analisis-de-riesgos.md) · [Requisitos SR](docs/security/requisitos.md) · [Patrones](docs/design/patrones.md)
- [Glosario](docs/glosario.md) · [Bitácora](docs/sessions/)
- Clones de referencia: `../orcha` (`b647cab`) y `../herdr`.
