# Guía para agentes — Hilvan

Instrucciones comunes para cualquier agente de IA (Claude Code, Codex u otros) que trabaje en este repositorio. `CLAUDE.md` importa este archivo; edite aquí las reglas compartidas.

## Qué es este proyecto

Herramienta para coordinar agentes y cuentas de IA y conservar la continuidad de las tareas mediante contexto y resultados revisables. Nombre del producto: **Hilvan** (de *hilvanar*: unir piezas con puntadas). Comando `hilvan`, con alias `hv` (D-025): un binario de Go sin runtime (D-026, ADR-0014).

**Fase actual: MVP completo en Go** (plan 001, hitos M0–M5; migración a Go en el plan 002). La especificación 001 está `Approved` y el diseño en `docs/design/`.

## Reglas obligatorias

1. **Implementar solo lo aprobado.** El código de producto (`cmd/`, `internal/`) debe corresponder al alcance de una spec `Approved` y a un hito del plan. Lo que esté fuera de alcance requiere antes un cambio de spec o un ADR. Los spikes van en `spikes/` y son desechables.
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
| `docs/design/` | Diseño técnico del MVP: arquitectura, modelo de datos, contrato de adaptador, amenazas. |
| `docs/plans/` | Planes de implementación derivados del diseño aprobado. |
| `docs/research/` | Análisis, benchmarking e informes de descubrimiento. |
| `docs/sessions/` | Bitácora por sesión (append-only): qué se hizo, qué se decidió, qué sigue. |
| `docs/glosario.md` | Terminología del dominio. Use estos términos de forma consistente. |
| `spikes/` | Pruebas de viabilidad desechables, cada una con informe de resultados. |
| `cmd/hilvan/` | Punto de entrada del binario `hilvan` y su presupuesto de arranque y memoria. |
| `internal/` | Código Go por paquetes (ver `docs/design/arquitectura.md`): `domain`, `security`, `jsfmt`, `config`, `criterio`, `policy`, `process`, `launchers`, `store`, `adapters`, `gitops`, `core`, `cli`, `testutil`. Los tests van junto al código (`*_test.go`) y los datos en `testdata/` (eventos reales de los CLIs y casos dorados). |
| `scripts/check/` | Comprobación única antes de cada commit: `go run ./scripts/check`. |
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
- **Código:** Go (versión de `go.mod`), formateado con `gofmt`. Sin dependencias nuevas sin justificarlas en un ADR (hoy solo `go.yaml.in/yaml/v3` y `golang.org/x/text`). `internal/domain` no hace E/S. Procesos sin shell y con array de argumentos, siempre a través de `internal/process`. Antes de hacer commit: `go run ./scripts/check` (gofmt, vet y tests).
- **Código de terceros:** de otros repositorios tomamos ideas, no código; se reimplementan a partir de su descripción y se citan en `docs/research/`. Solo se copia código si es muy robusto y no es razonable reproducirlo a partir de la idea; en ese caso, antes de copiarlo, se pide permiso a la persona, se verifica que la licencia sea compatible con Apache-2.0 y se conservan la atribución y el aviso de licencia.
- **Fechas:** ISO 8601 (`2026-10-06`). Nunca fechas relativas ("ayer", "la semana pasada").
