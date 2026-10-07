---
name: nueva-adr
description: Crea un nuevo Architecture Decision Record en docs/adr a partir de la plantilla y lo añade al índice.
disable-model-invocation: true
argument-hint: "<título de la decisión>"
---

Crea un ADR para: $ARGUMENTS

1. Determina el siguiente número libre en `docs/adr/` (4 dígitos).
2. Copia `docs/adr/0000-plantilla.md` a `docs/adr/NNNN-titulo-en-kebab-case.md` (español, sin tildes en el nombre de archivo).
3. Rellena contexto, opciones y consecuencias con lo conversado y con los requisitos aplicables de `specs/` (cita `FR-`, `Q-`, `AC-`). Estado inicial: `Proposed`, salvo que el usuario haya decidido explícitamente.
4. Si faltan datos para alguna opción, márcalo como pendiente; no inventes evidencia sobre proveedores.
5. Añade la fila a la tabla de `docs/adr/README.md` y, si resuelve una pregunta `Q-`, indícalo al usuario para actualizar la spec.
