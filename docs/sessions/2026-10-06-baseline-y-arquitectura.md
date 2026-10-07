# 2026-10-06 — Investigación, baseline v1.0 y arquitectura

- **Agente/herramienta:** Claude Code (rol de arquitecto, en plan mode y aprobado por el usuario)
- **Commits:** rama `spec/001-v1.0`, etiqueta `spec-001-v1.0`; después, rama `feat/m0-esqueleto`

## Objetivo de la sesión

Revisar los pendientes, investigar foros y repositorios similares, resolver Q-001–Q-010 con la opción más viable y dejar todo listo para implementar.

## Hecho

- Investigación ([informe](../research/2026-10-06-foros-y-herramientas.md)): términos de Anthropic, aislamiento con `CLAUDE_CONFIG_DIR`/`CODEX_HOME`, ejecución sin interfaz, ACP, Reddit (mediante un espejo), issues de vibe-kanban, claude-squad, Orcha y herdr.
- Spike 002 ejecutado en este Linux; fixtures reales guardados.
- Spec 001 → v1.0 Approved; ADR-0002–0010; diseño completo; plan del MVP; licencia Apache-2.0.

## Decisiones

D-013–D-024 (ver la spec). Respuestas del usuario: CLI oficial con login propio para Claude; open source desde ya y monetización más adelante; perfiles en dos máquinas. Preguntó si se podía usar TS y Go: se decidió TS ahora y un contrato por stdio que permite añadir un runner en Go después.

## Preguntas abiertas nuevas o resueltas

- Resueltas: Q-001–Q-010.
- Nuevas: Q-011 (nombre público) y Q-012 (monetización).

## Siguiente paso

Terminar M0. Que el usuario ejecute el spike 001 en el equipo de validación (Windows/WSL).

## Continuación — M0 y nombre

- M0 implementado en la rama `feat/m0-esqueleto`: `npm run check` pasa (21 tests) y `ao doctor` detecta Node 22.22.1, git 2.53.0, Claude Code 2.1.292 y codex-cli 0.160.1.
- Desviación técnica menor, documentada en ADR-0002: el build se hace con `tsc` (TS 7) en lugar de tsup, y se añade `cross-spawn` para Windows.
- El usuario propuso el nombre **LOTA** (*Lord Of The Agents*). Se registró en Q-011 junto con los reparos (marca de Tolkien, mensaje contrario al control humano, npm ocupado). Sigue abierta.

## Continuación — nombre Hilvan y M1 en Linux

- El usuario eligió el nombre **Hilvan** entre Comitiva, Hilvan, Models Orchestrator y mantener el nombre de trabajo. Queda registrado como D-025 (spec 001 v1.1, etiqueta `spec-001-v1.1`). Paquete y comando `hilvan`, con alias `hv`.
- M1 implementado y verificado en este Linux con los logins reales. Pasan 50 tests.
- Hallazgo: reutilizar el login por defecto de Claude exige *eliminar* `CLAUDE_CONFIG_DIR`, no apuntarla a `~/.claude`. La prueba creó `~/.claude/.claude.json`, que se eliminó tras comprobar que era un residuo de la prueba (389 bytes, sin datos de cuenta).
- Pendiente: verificar los lanzadores `wsl` y `windows` en el equipo de validación (spike 001).
