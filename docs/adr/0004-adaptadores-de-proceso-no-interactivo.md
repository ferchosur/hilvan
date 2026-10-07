# ADR-0004 — Adaptadores de proceso no interactivo para Claude Code y Codex

- **Estado:** Accepted
- **Fecha:** 2026-10-06
- **Decisores:** Fernando Giraldo (aprobación), Claude Code (propuesta como arquitecto)
- **Relacionado:** D-012, D-015, D-016, FR-014, Q-003, Q-009, [contrato](../design/contrato-adaptador.md)

## Contexto

El orquestador debe entregar instrucciones a un agente, recibir estado y salida, cancelar y reanudar. Hay tres vías: proceso no interactivo, puente a una sesión interactiva abierta, o un protocolo estándar (ACP).

## Opciones consideradas

1. **Proceso no interactivo por ejecución** — `claude -p --output-format stream-json --verbose` y `codex exec --json`; ambos documentan eventos, códigos de salida, salida estructurada (`--json-schema` / `--output-schema`) y reanudación por id de sesión.
2. **Puente a sesión interactiva** (escribir en una terminal/tmux) — frágil, heurístico, difícil de auditar; D-012 lo desaconseja.
3. **ACP** — estándar prometedor, pero v2 en borrador y el adaptador de Claude se basa en el Agent SDK (orientado a API key según los términos de Anthropic).

## Decisión

**Opción 1.** Cada ejecución lanza un proceso con `spawn` y un array de argumentos (`shell: false`), lee JSONL por stdout y lo traduce a eventos normalizados. Cancelación: SIGINT, y SIGTERM tras un plazo. Reanudación con `--resume <id>` / `codex exec resume <id>`. Adaptadores iniciales: `claude-code` y `codex`. ACP queda como **adaptador genérico posterior**; el puente interactivo, descartado para el MVP. Perplexity diferido (D-004).

## Consecuencias

- Positivas: control determinista, auditable y probado por la comunidad en CI.
- Negativas: depende del formato de eventos de cada CLI (R-008) → fixtures reales (spike 002) y detección de capacidades (`system/init.capabilities`).
- En `claude -p` sin `--bare` se cargan hooks y `.mcp.json` del repositorio; se mitiga con `--strict-mcp-config` y ajustes explícitos (NFR-011, ver [amenazas](../design/amenazas.md)).
