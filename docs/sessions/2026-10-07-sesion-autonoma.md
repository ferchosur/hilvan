# 2026-10-07 — Sesión autónoma: Criterio, seguridad, M2, M3, rendimiento y UI

- **Agente/herramienta:** Claude Code, en modo autónomo con plan aprobado mientras el usuario descansaba.
- **Commits:** de `a5d125d` (Criterio) a la fusión de M3 y los cierres posteriores.

## Objetivo de la sesión

Implementar todo lo posible con Hilvan ligero y sin terceros. Ampliar la investigación (UI), medir lenguajes, hacer un análisis de riesgos e implementar los requisitos de seguridad, aplicar patrones de diseño y proponer otras tareas.

## Hecho

1. **Criterio** (ADR-0011, spec v1.2): motor de decisión local al estilo JEV. ~30 µs por decisión y 24 tests.
2. **Seguridad:**
   - análisis STRIDE (25 amenazas) y requisitos SR-001…SR-036;
   - saneado de terminal, detector de secretos, E/S segura y límites de YAML y JSONL;
   - auditoría con cadena de hash;
   - CI endurecida, Dependabot y `SECURITY.md`.
3. **Spike 003:** benchmark en Node, Go, Python y Java con paridad verificada → Q-013. Optimización del arranque del CLI (298 → 149 ms).
4. **M2:** tareas y ejecución completas, con e2e real en Claude Code y Codex.
5. **SR-012:** confianza explícita en los hooks del repositorio.
6. **Investigación de UI** (TUI y web) → ADR-0012 *Proposed*. **Spike 004:** web local mínima segura.
7. **Patrones de diseño** documentados, README en inglés, código de conducta y plantillas de issues y PR.
8. **M3:** política en capas, aprobación con vista del brief, `task recommend` y `policy show|check`.

## Decisiones (tomadas en autonomía, dentro del plan)

- Criterio: reglas, regresión logística y Beta/Thompson; sin términos `bias` ni de longitud en el modelo de texto (evitan que pocos ejemplos sesguen todo).
- Etiquetar con baja confianza → `confirm`, no `escalate`. Endurecer la clasificación siempre se confirma.
- Codex se ejecuta con `-` (prompt por stdin). Las tareas de escritura hacen commit automático en la rama de la tarea con `--no-verify` (los hooks del repositorio no son confiables).
- `.hilvan/.gitignore` con `*` en lugar de tocar el `.gitignore` del usuario.
- `npm audit signatures` solo avisa (npm < 10 rechaza claves rotadas en 2025).

## Hallazgos

- En la prueba real, Codex alucinó el contenido de un README sin leerlo. Esto refuerza la revisión cruzada (M5).
- Go es objetivamente más ligero (Q-013). La decisión queda para el usuario.
- La rotación de claves de npm afecta a npm 9 (el instalado aquí).

## Preguntas abiertas nuevas o resueltas

- Nuevas: Q-013 (Go) y ADR-0012 (UI, Proposed).
- Siguen abiertas: Q-012 y el spike 001 en Windows.

## Continuación — M4, M5 y cierre del MVP

9. **Panorama 2026** de orquestadores y gestores de cuentas: CCS, el más cercano en cuentas, usa un proxy local de OAuth (riesgo de términos de uso) y no tiene gobierno de datos. ComposioHQ usa el nombre "agent-orchestrator".
10. **M4:** `hilvan handoff` con paquete mecánico (funciona aunque el origen esté bloqueado), afirmaciones del agente marcadas como no verificadas, secretos redactados, edición, aprobación, sha256 y política origen → destino.
11. **M5:** `hilvan review` (diff, riesgo y rutas sensibles), `--by` (revisión cruzada) y `accept` (exige el diff revisado, checkout limpio y rama base). Criterio aprende de aceptar y descartar.
12. **FR-018:** hook de preparación por worktree.
13. **Matriz de verificación:** 20 de 22 AC verificados.

**E2E real destacado:** Claude, como revisor cruzado, detectó y rechazó la respuesta alucinada de Codex.

## Siguiente paso

Decisiones del usuario: Q-013 (Go) y ADR-0012 (UI). Validación en Windows/WSL (spike 001).
