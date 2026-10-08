# 2026-10-07/08 — Migración a Go, GitHub y validación en el equipo de validación

- **Agente/herramienta:** Claude Code, con el usuario presente; una sesión de Claude Code en el WSL del equipo de validación coordinada por Remote Control.
- **Commits:** de `315348a` (spike 005) a la fusión del PR #1 (`feat/go`).

## Objetivo de la sesión

Valorar ideas externas (mods de Claude Code, claude-router), decidir Q-013 y migrar Hilvan a Go con paridad verificada, publicar el repositorio y avanzar la validación en el equipo de validación (spike 001).

## Hecho

1. **Spike 005 (mods de Claude Code), `Planned`:** los mods se ejecutan en `claude -p` y los de usuario no los bloquea `--setting-sources user`.
2. **Investigación de claude-router:** se toman ideas, no código. Nueva regla en AGENTS.md: *Código de terceros*.
3. **Q-013 = Go** (D-026, ADR-0014, plan 002), del G0 al G5:
   - **paridad verificada** con casos dorados generados por el código TS: 2.507 operaciones de Criterio, parsers, auditoría y `task.json`;
   - **prueba diferencial de la CLI:** 61/61 pasos idénticos;
   - **e2e real** con Claude y Codex;
   - **retirada del TypeScript.**
4. **GitHub:** repositorio privado `ferchosur/hilvan`, CI en ubuntu y windows y `workflow_dispatch`.
5. **Spike 001 en el equipo de validación**, a través de la sesión remota y del usuario:
   - aislamiento sin login verificado en WSL, en Windows nativo y de Windows a WSL;
   - el PATH de nvm y el UTF-16 de `wsl.exe`;
   - los worktrees necesitan `--relative-paths`;
   - PowerShell corre en *Constrained Language Mode*.

## Decisiones

- **Q-013:** Go (decidida por el usuario). Despachador de CLI propio sin dependencias, en lugar de cobra.
- **Repositorio:** privado hasta la release 0.1.0; ramas `main`, `feat/go` y `spike/005`.
- **Distribución de cuentas del usuario:** corporativa en Windows nativo y personal en WSL, con los proyectos en el disco de Windows compartidos.

## Errores corregidos gracias a la CI y a la prueba diferencial

- `hilvan accept` fallaba sin identidad de git (primera CI en ubuntu).
- El log de ejecución duplicaba la línea `result` y perdía las líneas sin eventos (prueba diferencial). Corregido en TS y en Go.

## Pendiente

- La fase C del spike 001 con sesión iniciada la ejecuta el usuario; el filtro de permisos de la sesión remota la bloqueó. Con ella se cierra AC-015.
- ADR-0012 (UI), la visión conversacional (spec 002) y el requisito "no aumentar el consumo" esperan decisión del usuario.
