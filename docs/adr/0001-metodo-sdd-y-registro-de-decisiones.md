# ADR-0001 — Método SDD y registro de decisiones

- **Estado:** Accepted
- **Fecha:** 2026-10-06
- **Decisores:** Fernando Giraldo
- **Relacionado:** D-006

## Contexto

El proyecto está en definición, con muchas preguntas abiertas sobre proveedores, autenticación y entornos. Varias personas y agentes de IA (con distintas cuentas y sesiones) trabajarán en él, por lo que el contexto debe sobrevivir entre sesiones y las decisiones deben ser trazables.

## Opciones consideradas

1. **Desarrollo exploratorio directo** — prototipar y documentar después.
2. **SDD con documentos versionados en git** — requisitos → diseño → plan → implementación → verificación.

## Decisión

Elegimos **SDD con documentos versionados en git** (ya acordado como D-006), con esta distribución:

- Decisiones de producto → `D-xxx` en `specs/`.
- Decisiones de arquitectura → ADR en `docs/adr/`.
- Estado actual → `CONTEXTO.md`; historia → `docs/sessions/`; cambios notables → `CHANGELOG.md`.
- La incertidumbre técnica se reduce con spikes desechables en `spikes/` antes del diseño.

## Consecuencias

- Positivas: cualquier agente o persona puede retomar el trabajo leyendo `CONTEXTO.md`; las decisiones tienen ID y motivo.
- Negativas: sobrecoste documental; hay que mantener `CONTEXTO.md` al día al cerrar cada sesión.
- Seguimiento: las skills `/retomar` y `/cerrar-sesion` reducen el coste de mantenimiento.
