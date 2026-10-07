---
name: nuevo-spike
description: Crea la carpeta e informe de un nuevo spike de viabilidad en spikes/ a partir de la plantilla.
disable-model-invocation: true
argument-hint: "<pregunta técnica a responder>"
---

Crea un spike para la pregunta: $ARGUMENTS

1. Determina el siguiente número libre en `spikes/` (3 dígitos).
2. Crea `spikes/NNN-nombre-corto/README.md` a partir de `spikes/_plantilla/README.md`.
3. Reformula la pregunta para que sea concreta y verificable; enlaza los `Q-`, `FR-`, `AC-` relacionados de `specs/`.
4. Propón un límite de tiempo y un procedimiento reproducible. Estado inicial: `Planned`.
5. Actualiza la tabla de `spikes/README.md`.

Recuerda: el código del spike es desechable y nunca debe contener credenciales ni rutas reales de perfiles de autenticación.
