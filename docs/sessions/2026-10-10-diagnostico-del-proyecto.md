# 2026-10-10 — Diagnóstico del estado del proyecto

- **Agente/herramienta:** Codex.
- **Commits:** sin cambios de código ni commits; revisión sobre `main` en `f087882`.

## Objetivo de la sesión

Revisar el estado actual de Hilvan (antes `agent-orchestrator`), contrastar código, especificación, seguridad, pruebas y documentación, y dejar prioridades para continuar.

## Hecho

- Confirmado que el renombre a Hilvan está aplicado; el nombre anterior solo persiste en referencias históricas.
- Contrastada la matriz del MVP: 23/25 criterios verificados; AC-015 parcial hasta probar cuentas iniciadas en Windows/WSL y AC-009 diferido.
- Verificados con Go 1.27.1 `go run ./scripts/check`, `GOOS=windows go vet ./...`, `go test -race ./...`, `git fsck` y `git diff --check`; todo pasó. El árbol de trabajo estaba limpio.
- Medida cobertura con `go test -cover ./...`: adapters 94.2 %, criterio 95.7 %, core 81.1 %, CLI 30.3 %, gitops 48.5 % y store 69.7 %.
- Halladas inconsistencias de documentación: README español e inglés informan 20/22 y spec v1.2; el README inglés y el modelo de amenazas conservan una descripción obsoleta de hooks; el plan 001 aún menciona distribución npm.
- Confirmado que no existe pipeline/tag de release; SR-029 (binarios, SHA-256 y attestations) está pendiente.
- Registradas mejoras candidatas: fijar el modelo al reanudar; permitir comandos de lectura explícitos para tareas Claude `read`; cubrir más CLI/gitops/store; corregir `stats` para perfiles solo revisores; arreglar metadatos direct/indirect en `go.mod`; considerar `go test -race` en CI.

## Decisiones

- No se cambió código de producto ni se adoptaron propuestas que requieren actualizar spec/ADR/SR.
- Orden sugerido: corregir documentación de estado/seguridad y referencias npm; completar AC-015; preparar release 0.1.0 según SR-029; después resolver las propuestas de modelo y herramientas de lectura.

## Siguiente paso

Actualizar README y documentación obsoleta antes de presentar Hilvan a usuarios; luego realizar la fase C del spike 001 con sesiones reales en el equipo de validación.
