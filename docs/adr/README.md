# Architecture Decision Records

Una decisión de arquitectura por archivo, formato [MADR](https://adr.github.io/madr/) simplificado. Copie [`0000-plantilla.md`](0000-plantilla.md) como `NNNN-titulo-corto.md` (o use `/nueva-adr` en Claude Code).

Las decisiones de **producto** (`D-xxx`) se registran en la especificación; aquí van las de **arquitectura y técnica** (cómo se construye).

| ADR | Título | Estado | Fecha |
|---|---|---|---|
| [0001](0001-metodo-sdd-y-registro-de-decisiones.md) | Método SDD y registro de decisiones | Accepted | 2026-10-06 |
| [0002](0002-typescript-node-y-contrato-por-stdio.md) | TypeScript sobre Node.js y contrato JSON por stdio | Accepted | 2026-10-06 |
| [0003](0003-cli-primero-y-plataformas.md) | CLI primero; Linux, Windows y WSL | Accepted | 2026-10-06 |
| [0004](0004-adaptadores-de-proceso-no-interactivo.md) | Adaptadores de proceso no interactivo para Claude Code y Codex | Accepted | 2026-10-06 |
| [0005](0005-un-orquestador-con-lanzadores.md) | Un único orquestador con lanzadores por destino | Accepted | 2026-10-06 |
| [0006](0006-identidad-por-directorio-de-configuracion.md) | Identidad por directorio de configuración del CLI oficial | Accepted | 2026-10-06 |
| [0007](0007-persistencia-en-archivos.md) | Persistencia en archivos locales | Accepted | 2026-10-06 |
| [0008](0008-worktree-por-tarea-y-ejecucion-secuencial.md) | Worktree por tarea y ejecución secuencial en el MVP | Accepted | 2026-10-06 |
| [0009](0009-clasificacion-politicas-y-aprobacion.md) | Clasificación, políticas gestionadas y aprobación de paquetes | Accepted | 2026-10-06 |
| [0010](0010-licencia-apache-2.md) | Licencia Apache-2.0 | Accepted | 2026-10-06 |

Candidatos futuros: lanzador SSH, UI web local, paralelismo, adaptador ACP, migración a SQLite si el volumen lo exige.
