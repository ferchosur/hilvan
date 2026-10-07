# Guía para agentes — agent-orchestrator

Instrucciones comunes para cualquier agente de IA (Claude Code, Codex u otros) que trabaje en este repositorio. `CLAUDE.md` importa este archivo; edite aquí las reglas compartidas.

## Qué es este proyecto

Herramienta para coordinar agentes y cuentas de IA y conservar la continuidad de las tareas mediante contexto y resultados revisables. Nombre de trabajo: `agent-orchestrator` (nombre de producto pendiente).

**Fase actual: definición (SDD — requisitos).** No hay arquitectura aprobada ni implementación. La especificación 001 v0.3 está en `Draft`.

## Reglas obligatorias

1. **No implementar sin baseline aprobado.** No crear código de producto (`src/`, paquetes, dependencias) mientras la especificación correspondiente no esté en estado `Approved`. Los spikes van en `spikes/` y son desechables.
2. **La especificación es la fuente de verdad.** Si una conversación contradice `specs/`, señalarlo y proponer el cambio en la especificación; no lo resuelva de forma implícita.
3. **No inventar decisiones.** Las preguntas abiertas (`Q-xxx`) se resuelven con la persona. Registre la respuesta como decisión (`D-xxx`) en la especificación o como ADR si es de arquitectura.
4. **Distinguir hechos de supuestos.** No afirme capacidades, límites ni términos de proveedores sin fuente verificable y fecha de consulta.
5. **Secretos fuera del repo.** Nunca escriba credenciales, tokens, cookies ni rutas de perfiles de autenticación reales en archivos versionados ni en logs.
6. **Cuotas y términos.** No proponga mecanismos para sumar, compartir o eludir límites de proveedores.

## Mapa del repositorio

| Ruta | Contenido |
|---|---|
| `CONTEXTO.md` | **Memoria viva**: estado actual, decisiones, preguntas críticas y próximos pasos. Léalo primero. |
| `specs/` | Especificaciones SDD numeradas (`NNN-nombre.md`). Fuente de verdad de requisitos. |
| `docs/adr/` | Registros de decisiones de arquitectura (ADR), inmutables una vez aceptados. |
| `docs/design/` | Diseño técnico (bloqueado hasta aprobar requisitos). |
| `docs/plans/` | Planes de implementación derivados del diseño aprobado. |
| `docs/research/` | Análisis, benchmarking e informes de descubrimiento. |
| `docs/sessions/` | Bitácora por sesión (append-only): qué se hizo, qué se decidió, qué sigue. |
| `docs/glosario.md` | Terminología del dominio. Use estos términos de forma consistente. |
| `spikes/` | Pruebas de viabilidad desechables, cada una con informe de resultados. |
| `CHANGELOG.md` | Cambios notables por versión de documentos y, más adelante, del producto. |

## Flujo de sesión

- **Al empezar:** leer `CONTEXTO.md`, la última entrada de `docs/sessions/` y `git log --oneline -10`. (En Claude Code: `/retomar`.)
- **Durante:** cambios pequeños y enfocados; una idea por commit.
- **Al terminar:** actualizar `CONTEXTO.md` (estado, no historia), añadir la entrada de bitácora, actualizar `CHANGELOG.md` si hubo cambios notables y proponer commit. (En Claude Code: `/cerrar-sesion`.)

## Convenciones

- **Idioma:** documentación en español; identificadores de código, nombres de rama y tipos de commit en inglés.
- **Commits:** [Conventional Commits](https://www.conventionalcommits.org/es/v1.0.0/) — `tipo(ámbito): descripción` en español e imperativo. Tipos: `docs`, `spec`, `adr`, `spike`, `feat`, `fix`, `refactor`, `test`, `chore`. Detalle en `CONTRIBUTING.md`.
- **Ramas:** `main` siempre coherente. Trabajo en `spec/NNN-…`, `adr/NNNN-…`, `spike/NNN-…`, `feat/…`.
- **Versiones de especificación:** semver documental (`0.x` = Draft; `1.0` = baseline aprobado). Cada cambio de versión se refleja en el encabezado de la spec y en `CHANGELOG.md`.
- **Identificadores estables:** `D-` decisión, `Q-` pregunta, `FR-` requisito funcional, `NFR-` no funcional, `AC-` criterio de aceptación, `R-` riesgo, `A-` supuesto. No reutilizar ni renumerar; marcar como `Obsoleto` si se retira.
- **Fechas:** ISO 8601 (`2026-10-06`). Nunca fechas relativas ("ayer", "la semana pasada").
