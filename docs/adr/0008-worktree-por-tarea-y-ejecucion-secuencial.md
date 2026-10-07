# ADR-0008 — Worktree por tarea y ejecución secuencial en el MVP

- **Estado:** Accepted
- **Fecha:** 2026-10-06
- **Decisores:** Fernando Giraldo (aprobación), Claude Code (propuesta como arquitecto)
- **Relacionado:** D-020, FR-018, AC-010, AC-021, Q-005, R-005

## Contexto

Agentes que editan el mismo árbol se pisan. Orcha documentó ramas mezcladas al compartir checkout; vibe-kanban y claude-squad usan worktrees y sus usuarios piden limpieza automática (#765) y hooks de preparación (#260).

## Decisión

- Toda tarea que puede modificar archivos se ejecuta en `git worktree add <dir> -b hilvan/<id-tarea>`, en una carpeta hermana del repositorio (`../.hilvan-worktrees/<repo>/<id>`) para no anidar repositorios.
- Hook de preparación opcional por proyecto (p. ej. `npm ci`), declarado en la configuración del proyecto.
- Revisión = `git diff <base>...hilvan/<id>`; aceptar = fusionar en la rama base; aceptar o descartar elimina worktree y rama.
- El MVP ejecuta **una tarea a la vez**; el paralelismo (incremento 2) solo requiere permitir varias ejecuciones simultáneas, porque el aislamiento ya existe.
- Tareas de solo lectura (revisión, análisis) no crean worktree.

## Consecuencias

- Positivas: diff revisable siempre; sin conflictos entre tareas; paralelismo futuro barato.
- Negativas: requiere repositorio git; dependencias duplicadas por worktree (mitigado con el hook).
