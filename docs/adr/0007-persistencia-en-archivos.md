# ADR-0007 — Persistencia en archivos locales

- **Estado:** Accepted
- **Fecha:** 2026-10-06
- **Decisores:** Fernando Giraldo (aprobación), Claude Code (propuesta como arquitecto)
- **Relacionado:** D-019, FR-009, NFR-004

## Contexto

Hay que guardar perfiles, proyectos, tareas, ejecuciones, paquetes de contexto, aprobaciones y auditoría, de forma recuperable tras reinicios y fácil de inspeccionar.

## Opciones consideradas

1. **SQLite** — consultas potentes; `node:sqlite` aún madura y los módulos nativos complican Windows.
2. **Archivos** (YAML/JSON/Markdown/JSONL) — inspeccionables, versionables y sin dependencias; suficiente para un usuario y ejecución secuencial.

## Decisión

**Archivos**, detrás de una interfaz de repositorio (`Store`) para poder cambiar a SQLite sin tocar el dominio:

- Configuración de usuario: `profiles.yaml` y `policy.yaml` en `$XDG_CONFIG_HOME/hilvan/` (Linux), `%APPDATA%\hilvan\` (Windows) o `~/Library/Application Support/hilvan/` (macOS).
- Estado del proyecto: carpeta `.hilvan/` en la raíz del repositorio (`tasks/<id>/task.json`, `brief.md`, `handoff-*.md`, `runs/<run>.jsonl`), y `events.jsonl` **append-only** como registro de auditoría.
- Escrituras atómicas (archivo temporal + renombrado). `.hilvan/` se añade a `.gitignore` del proyecto por defecto.

## Consecuencias

- Positivas: cero dependencias nativas; el usuario y los agentes pueden leer el estado; depuración sencilla.
- Negativas: sin consultas complejas ni concurrencia multi-proceso → migrar a SQLite si llega el paralelismo intensivo o la UI web lo requiere.
