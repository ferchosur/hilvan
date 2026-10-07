# Contexto para continuar el proyecto

Última actualización: 2026-10-06
Estado: **baseline aprobado (spec 001 v1.0)**. Diseño del MVP aceptado. **M0 terminado.** **M1 verificado en Linux** (`hilvan profile …` con Claude Code y Codex reales); Windows/WSL pendiente del spike 001. Siguiente: **M2** (tareas y ejecución).

Este documento resume el estado **actual**. Fuentes de verdad:

- Requisitos: [`specs/001-product-requirements.md`](specs/001-product-requirements.md).
- Decisiones técnicas: [`docs/adr/`](docs/adr/).
- Diseño: [`docs/design/`](docs/design/).
- Plan: [`docs/plans/001-mvp.md`](docs/plans/001-mvp.md).
- Historia: [`docs/sessions/`](docs/sessions/).

## Objetivo del producto

**Hilvan** (D-025): herramienta open source (Apache-2.0) para coordinar agentes de IA entre proveedores y entre varias cuentas de una misma aplicación, conservando la continuidad de las tareas mediante paquetes de contexto revisables. Cada usuario elige qué aplicaciones conecta y cuántos perfiles configura. **Se posiciona como continuidad y gobierno, nunca como forma de sumar o eludir cuotas.**

Caso de origen del usuario:

- **equipo de validación (Windows + WSL):** Claude corporativo (con MCP internos), Claude personal y Codex personal.
- **Linux personal:** Claude y Codex.

## Decisiones vigentes (resumen; detalle en spec y ADR)

| Tema | Decisión | Ref. |
|---|---|---|
| Lenguaje | TypeScript, Node ≥ 22; contrato JSON por stdio para permitir runners en Go en el futuro | D-013, ADR-0002 |
| Superficie | CLI `hilvan` primero; UI web local en el incremento 2 | D-014, ADR-0003 |
| SO | Linux, Windows nativo y WSL; macOS "best effort" | D-014 |
| Agentes | Claude Code y Codex CLI, en proceso no interactivo (`claude -p` stream-json, `codex exec --json`) | D-015, D-016, ADR-0004 |
| Entornos | Un orquestador con lanzadores `native`, `wsl` y `windows`; `ssh` en el incremento 2 | D-017, ADR-0005 |
| Identidad | `CLAUDE_CONFIG_DIR` / `CODEX_HOME` por perfil; login con el CLI oficial; Hilvan nunca toca tokens | D-018, ADR-0006 |
| Persistencia | Archivos: `profiles.yaml` y `policy.yaml` del usuario; `.hilvan/` por proyecto con `events.jsonl` | D-019, ADR-0007 |
| Aislamiento | Worktree y rama por tarea que escribe; ejecución secuencial en el MVP | D-020, ADR-0008 |
| Políticas | Clasificaciones configurables; política gestionada `enforced`; aprobación del paquete completo y editable, con hash | D-021–D-023, ADR-0009 |
| Licencia | Apache-2.0, contribuciones con DCO | D-024, ADR-0010 |
| Nombre | **Hilvan**; comando `hilvan` (alias `hv`); estado de proyecto en `.hilvan/` | D-025 |
| Perplexity | Diferido: su API requiere créditos aparte y el usuario no los tiene | D-004 |

Principios que siguen vigentes:

- Elegibilidad antes de recomendar.
- Ningún cambio silencioso de perfil.
- Cuotas separadas.
- Revisión humana antes de aceptar.

## Hallazgos técnicos clave (spike 002, 2026-10-06)

- `claude -p` emite `rate_limit_event` con el porcentaje de uso de las ventanas `five_hour` y `seven_day`: es un dato oficial de cuota por perfil.
- `claude auth status` (JSON) identifica la cuenta. `codex login status` solo indica si hay sesión.
- Tras SIGINT, Claude sale con código 0 y `result.is_error=true`: el estado final se decide por los eventos.
- Hay que cerrar stdin (o enviar el prompt por stdin). `CODEX_HOME` debe existir y no puede estar bajo `/tmp`.
- **El login por defecto de Claude guarda la cuenta en `~/.claude.json`, fuera de `~/.claude`.** Un perfil que lo reutiliza usa `configDir: null` y Hilvan *elimina* `CLAUDE_CONFIG_DIR`. Fijarla en `~/.claude` hace perder la identidad (descubierto en M1).
- Términos de Anthropic: se permite que el usuario inicie sesión en el binario oficial sin modificar; está prohibido manejar tokens u ofrecer el login de Claude.ai. No se puede usar "Claude Code" en el nombre del producto.

## Diferenciación (validada en la investigación de 2026-10-06)

Ninguno de los orquestadores revisados (Orcha, vibe-kanban, claude-squad, herdr, codex-plugin-cc, entre otros) modela **varias cuentas del mismo proveedor como identidades con políticas y herramientas propias**. Los foros piden sobre todo continuidad al llegar a un límite, revisión cruzada entre proveedores, consumo visible, worktrees con limpieza automática y buen soporte de Windows. Todo eso está en el MVP (FR-015 a FR-019).

## Estado de los documentos

| Documento | Estado |
|---|---|
| Spec 001 | **1.0 Approved** (etiqueta `spec-001-v1.0`) |
| ADR-0001 a 0010 | Accepted |
| Diseño (arquitectura, datos, adaptador, amenazas) | Accepted |
| Plan 001 MVP (M0–M5) | Accepted |
| Spike 001 (auth aislada) | In Progress: Linux parcial; **falta Windows/WSL en el equipo de validación** |
| Spike 002 (invocación sin interfaz) | Done |

## Preguntas abiertas (no bloquean el MVP)

- **Q-012 — Monetización:** el usuario quiere tratarlo después del MVP.
- Spike 001 en Windows: si el CLI guarda credenciales en el almacén del sistema y las comparte entre directorios, los perfiles de Windows tendrían que usar API key u otro usuario del sistema.

## Próximos pasos

1. ~~M0~~ terminado: `npm run check` y `hilvan doctor` funcionan en Linux; la CI cubre Windows.
2. El usuario ejecuta el **spike 001** en el equipo de validación (procedimiento en `spikes/001-auth-aislada/`).
3. ~~M1 en Linux~~ hecho. Siguiente, **M2**: `hilvan init`, tareas con brief, ejecución con los adaptadores (stream-json / `exec --json`), worktrees, auditoría, consumo y límites.
4. M2 a M5 según el [plan](docs/plans/001-mvp.md); después, la release 0.1.0.

## Referencias

- [Investigación de foros y herramientas (2026-10-06)](docs/research/2026-10-06-foros-y-herramientas.md)
- [Estado y oportunidades (2026-10-04)](docs/research/estado-y-oportunidades.md): análisis de Orcha, codex-plugin-cc y del patrón Advisor/JEV.
- [Glosario](docs/glosario.md)
- Clones locales de referencia: `../orcha` (commit `b647cab`) y `../herdr`.
