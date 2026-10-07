# ADR-0009 — Clasificación, políticas gestionadas y aprobación de paquetes

- **Estado:** Accepted
- **Fecha:** 2026-10-06
- **Decisores:** Fernando Giraldo (aprobación), Claude Code (propuesta como arquitecto)
- **Relacionado:** D-008, D-010, D-021, D-022, D-023, FR-004, FR-011, AC-003, AC-004, AC-012, AC-014, Q-006, Q-007, Q-008

## Contexto

El sistema debe excluir perfiles no elegibles, pedir aprobación en cruces sensibles y respetar restricciones organizacionales que el usuario no puede anular, sin servidor central.

## Decisión

- **Clasificación:** cada tarea tiene una etiqueta; valores por defecto `public < personal < corporate < confidential`, configurables por proyecto. Cada perfil declara `accepts: [...]`.
- **Política en capas** (modelo similar a los *managed settings* de Claude Code):
  1. **Gestionada** — archivo de solo lectura para el usuario (`/etc/hilvan/policy.yaml` o `%ProgramData%\hilvan\policy.yaml`). Sus reglas `enforced` **no se pueden aprobar por encima** (AC-014).
  2. **Usuario** — `policy.yaml` del directorio de configuración.
  3. **Proyecto** — `.hilvan/policy.yaml`, solo puede **restringir** más.
- Reglas expresadas como `{ from: <clasificación|perfil>, to: <perfil|etiqueta de perfil>, effect: deny | require_approval }`. Evaluación: denegaciones gestionadas → requisitos de herramientas/capacidades → aprobación requerida → permitido. El resultado incluye **motivos legibles** (AC-012).
- **Aprobación (Q-008):** se aprueba el **paquete completo**; antes, el usuario puede editarlo (Markdown en `$EDITOR`). Se envía exactamente lo aprobado y se registra su SHA-256 en `events.jsonl`. Cancelar no envía nada (AC-004).

## Consecuencias

- Positivas: restricciones obligatorias sin infraestructura; auditable; simple de explicar.
- Negativas: la política gestionada depende de permisos del SO (un administrador local podría cambiarla); suficiente para el MVP individual.
