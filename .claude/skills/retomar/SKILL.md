---
name: retomar
description: Recupera el contexto del proyecto al iniciar una sesión — estado actual, última bitácora, commits recientes y próximos pasos. Usar al empezar a trabajar o cuando el usuario pregunte "dónde quedamos".
---

Recupera el contexto del proyecto Hilvan y resúmelo para el usuario.

1. Lee `CONTEXTO.md` completo.
2. Lista `docs/sessions/` y lee la entrada más reciente (excluye `README.md`).
3. Ejecuta `git status --short` y `git log --oneline -10`.
4. Revisa el encabezado (estado/versión) de cada archivo en `specs/` y la tabla de `docs/adr/README.md`.

Responde en español, de forma breve:

- **Estado:** fase SDD y versión/estado de cada spec.
- **Última sesión:** fecha, qué se hizo y siguiente paso declarado.
- **Cambios sin commit:** si los hay.
- **Preguntas bloqueantes:** las `Q-` que impiden avanzar.
- **Propuesta:** el siguiente paso concreto que recomiendas.

Si detectas incoherencias (p. ej. `CONTEXTO.md` dice una versión distinta de la spec), señálalas.
