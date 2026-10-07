# Changelog

Cambios notables del proyecto. Formato basado en [Keep a Changelog](https://keepachangelog.com/es-ES/1.1.0/). Las especificaciones usan versionado documental (ver `CONTRIBUTING.md`); el producto usará [SemVer](https://semver.org/lang/es/) cuando exista código.

## [Unreleased]

### Cambiado

- **Nombre del producto: Hilvan** (D-025, spec 001 v1.1): paquete y comando `hilvan` (alias `hv`), directorio de configuración `hilvan/`, estado de proyecto `.hilvan/`, variable `HILVAN_CONFIG_DIR`.

### Añadido

- **M0 — esqueleto del CLI `hilvan`:** proyecto TypeScript (Node ≥ 22.12, ESM), Biome, Vitest, build con `tsc`, CI en Ubuntu y Windows con Node 22 y 24; esquemas zod de perfil, destino, tarea y máquina de estados; rutas de configuración por SO; carga validada de `profiles.yaml`; comando `hilvan doctor`.
- **Especificación 001 v1.0 aprobada** (baseline): resuelve Q-001–Q-010 (D-013–D-024); FR-015–FR-019 (revisión cruzada, consumo visible, estados bloqueado/incierto, worktree por tarea, brief); NFR-010–NFR-012; AC-018–AC-021; R-006–R-008.
- ADR-0002 a ADR-0010: stack, superficie, adaptadores, lanzadores, identidad, persistencia, worktrees, políticas y licencia.
- Diseño: arquitectura, modelo de datos, contrato de adaptador y modelo de amenazas.
- Plan 001 del MVP (hitos M0–M5).
- Investigación de foros, herramientas similares y términos de uso (2026-10-06).
- Spike 002 (invocación sin interfaz) completado, con fixtures reales; spike 001 iniciado.
- Licencia Apache-2.0 y DCO.
- Espacio de trabajo profesional: `AGENTS.md`, `CLAUDE.md`, `CONTRIBUTING.md`, `CHANGELOG.md`, glosario, jerarquía `docs/` (adr, design, plans, research, sessions), `spikes/`, plantillas de spec, ADR y spike.
- Skills de Claude Code: `/retomar`, `/cerrar-sesion`, `/nueva-adr`, `/nuevo-spike`.
- ADR-0001: método SDD y registro de decisiones.
- Repositorio git con convenciones de commits y ramas.

### Cambiado

- `docs/estado-y-oportunidades.md` movido a `docs/research/`; enlaces actualizados.

## Historial de especificaciones

| Fecha | Documento | Versión | Estado | Cambios |
|---|---|---|---|---|
| 2026-10-06 | 001 — Requisitos de producto | 1.1 | Approved | Nombre Hilvan (D-025, resuelve Q-011). |
| 2026-10-06 | 001 — Requisitos de producto | 1.0 | Approved | Baseline: decisiones D-013–D-024, FR-015–FR-019, NFR-010–NFR-012, AC-018–AC-021; Perplexity diferido. |
| 2026-10-06 | 001 — Requisitos de producto | 0.3 | Draft | D-011, D-012; FR-013, FR-014; NFR-008, NFR-009; AC-015–AC-017; Q-009, Q-010 (destinos y adaptadores de ejecución, Windows/WSL). |
| — | 001 — Requisitos de producto | 0.2 | Draft | D-007–D-010; FR-011, FR-012; configuración dinámica de aplicaciones/perfiles y elegibilidad antes de recomendar. |
| — | 001 — Requisitos de producto | 0.1 | Draft | Versión inicial. |
