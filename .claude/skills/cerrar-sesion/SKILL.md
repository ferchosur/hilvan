---
name: cerrar-sesion
description: Cierra la sesión de trabajo — actualiza CONTEXTO.md, escribe la entrada de bitácora, actualiza CHANGELOG.md y propone commits. Usar al terminar una sesión o cuando el usuario diga "cerremos", "guarda el progreso".
disable-model-invocation: true
---

Cierra la sesión de trabajo del proyecto agent-orchestrator. Tema opcional de la sesión: $ARGUMENTS

1. Revisa `git status` y `git diff` (incluido lo ya preparado) para saber qué cambió en la sesión, junto con lo conversado.
2. **CONTEXTO.md:** actualiza la fecha de "Última actualización" y las secciones afectadas para reflejar el estado **actual** (no la historia). Retira preguntas resueltas y añade las nuevas. Mantén las referencias a IDs (`D-`, `Q-`, `FR-`, `AC-`).
3. **Bitácora:** crea `docs/sessions/AAAA-MM-DD-tema.md` con la plantilla de `docs/sessions/README.md`. Si ya existe una para hoy con el mismo tema, añade un sufijo `-2`.
4. **CHANGELOG.md:** añade a `[Unreleased]` los cambios notables. Si cambió la versión de una spec, añade una fila al historial de especificaciones y actualiza `specs/README.md`.
5. Verifica que los enlaces relativos que tocaste apuntan a archivos existentes.
6. Propón uno o varios commits con Conventional Commits (ver `CONTRIBUTING.md`), agrupando por tipo (`spec`, `adr`, `docs`…). Muestra los mensajes y **pide confirmación** antes de hacer commit. No hagas push.

Nunca incluyas secretos ni datos corporativos en estos archivos.
