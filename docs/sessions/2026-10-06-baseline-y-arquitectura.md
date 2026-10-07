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

Terminar M0. Que el usuario ejecute el spike 001 en el PC del trabajo (Windows/WSL).
