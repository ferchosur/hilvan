# Requisitos de seguridad (SR)

- **Fecha:** 2026-10-07
- **Origen:** [análisis de riesgos](analisis-de-riesgos.md)
- **Estado de implementación:** ✔ implementado y probado · ◐ parcial · ☐ planificado (hito indicado)

| SR | Requisito | Amenazas | Estado | Dónde / prueba |
|---|---|---|---|---|
| SR-001 | Toda salida de origen externo (agente, repositorio, rutas, títulos) se sanea antes de imprimirla: se eliminan CSI/OSC/DCS/APC, controles C0/C1 (salvo `\n` y `\t`) y caracteres bidi o de ancho cero. | T01, T02 | ✔ | `src/security/sanitize.ts` · `tests/security.test.ts` |
| SR-002 | Los nombres de archivo y rama mostrados en aprobaciones y diffs pasan por SR-001 y por un marcado visible de caracteres no ASCII sospechosos. | T02 | ◐ (saneado ✔; las rutas sensibles se marcan con ⚠ en `hilvan review`) | `sanitize.ts` |
| SR-003 | Los procesos se lanzan con `spawn` y un array de argumentos, sin shell. El prompt viaja por stdin y nunca como argumento. Los valores controlados por el usuario que se pasan a git van tras `--` o validados. | T03, T18 | ✔ | `src/process/run.ts`, `src/adapters/runner.ts`, `src/gitops/` · tests |
| SR-004 | En Windows, los `.cmd` se lanzan con escapado seguro (cross-spawn) y se rechazan argumentos con saltos de línea o caracteres nulos. | T04 | ✔ | `src/security/args.ts` · tests |
| SR-005 | `hilvan doctor` avisa si `profiles.yaml` o `policy.yaml` son escribibles por el grupo u otros, o no pertenecen al usuario. | T05 | ✔ | `src/cli/doctor.ts` |
| SR-006 | Las lecturas de configuración y estado rechazan symlinks y archivos que no son regulares (`O_NOFOLLOW` + `fstat`) y aplican un tamaño máximo. | T05, T06 | ✔ | `src/security/fs.ts` · tests |
| SR-007 | Las escrituras son atómicas: archivo temporal con nombre aleatorio creado con `O_EXCL` y permisos 0600, después `rename`. Se rechazan destinos que sean symlinks y los directorios se crean con 0700. | T06 | ✔ | `src/security/fs.ts` · tests |
| SR-008 | El YAML se parsea con un máximo de 100 alias, claves únicas y un tamaño máximo de 1 MiB. | T07 | ✔ | `src/config/profiles.ts` · tests |
| SR-009 | El JSONL de los agentes se procesa por líneas con una longitud máxima por línea (1 MiB) y un total por ejecución (256 MiB). Al superarlo, la ejecución se corta y queda en `uncertain`. | T08 | ✔ | `src/process/lines.ts`, `src/adapters/runner.ts` · test "output limit" |
| SR-010 | Los perfiles de suscripción eliminan las claves de API heredadas del proveedor, y los de API key solo reciben la variable declarada. | T09 | ✔ (M1) | `src/adapters/*` · tests |
| SR-011 | Claude Code se lanza siempre con `--strict-mcp-config` y solo con el MCP declarado en el perfil. | T10 | ✔ | `src/adapters/claude-code.ts` · test "strict MCP" |
| SR-012 | Antes de la primera ejecución en un repositorio, Hilvan detecta `.claude/settings*.json` con hooks y `.mcp.json`, y muestra un aviso que debe aceptarse una vez por repositorio y versión del archivo (hash). | T10 | ✔ | `src/core/repo-trust.ts`, `hilvan task run --trust-repo-hooks` · test |
| SR-013 | Detector de secretos (patrones conocidos y alta entropía) aplicado a briefs, paquetes de traspaso y a lo que se persiste en `runs/`. Las coincidencias se redactan en almacenamiento y se marcan en las vistas previas. | T11, T12, T24 | ✔ | `src/security/secrets.ts`; briefs, logs y paquetes (`src/core/handoff.ts`) · tests |
| SR-014 | Un paquete de traspaso no se envía sin vista previa y aprobación cuando la política lo exige. Se envía exactamente lo aprobado, verificado por hash. | T11 | ✔ | `src/core/handoff.ts` (`approveHandoff`, sha256 en auditoría) · tests |
| SR-015 | La clasificación nunca se rebaja automáticamente: Criterio solo sugiere endurecerla, con confirmación. | T11 | ✔ | `src/criterio/` · tests |
| SR-016 | `hilvan init` añade `.hilvan/` al `.gitignore` del proyecto y `doctor` avisa si `.hilvan/` está versionado. | T12 | ✔ | `.hilvan/.gitignore` = `*` · test |
| SR-017 | El registro de auditoría es una cadena de hash: cada evento incluye `prev` (SHA-256 del evento anterior) y su propio `hash`. `hilvan audit verify` detecta modificaciones, borrados intermedios y reordenamientos. | T13 | ✔ | `src/store/audit.ts` · tests |
| SR-018 | La auditoría registra hashes y rutas, nunca el contenido de paquetes ni secretos. | T12, T13 | ✔ (por diseño del API) | `audit.ts` |
| SR-019 | La política del proyecto (`.hilvan/policy.yaml`) solo puede añadir restricciones; las reglas que relajan se ignoran con aviso. | T14 | ✔ | `src/policy/evaluate.ts#mergePolicies` (sin efecto "allow"; el proyecto no redefine clasificaciones) · tests |
| SR-020 | Las reglas `enforced` de la política gestionada no pueden aprobarse por encima, ni siquiera con `--yes`. | T15 | ✔ | `mergePolicies` + `runTask` ("cannot be approved") · tests |
| SR-021 | El modelo de Criterio se valida al cargar (esquema, tamaños, valores finitos) y sus pesos tienen un tope (±4). Ninguna decisión de Criterio puede ejecutar por sí sola acciones sensibles. Existe `hilvan criterio reset`. | T16 | ✔ | `src/config/criterio-store.ts`, `thresholds.ts` · tests |
| SR-022 | Criterio no almacena texto: solo pesos por hash de señales. | T17 | ✔ | `features.ts` · test de persistencia |
| SR-023 | Los nombres de rama y rutas de worktree se generan a partir de ids validados (`hilvan/tsk_<ulid>`) y nunca a partir de texto libre. | T18 | ✔ | `src/gitops/` (`hilvan/tsk_<ulid>`) · tests |
| SR-024 | Aceptar una tarea exige que se haya mostrado su diff. Si Criterio estima un riesgo ≥ 7.5, se recomienda revisión cruzada antes de aceptar. | T19 | ✔ | `src/core/review.ts` (`reviewedHead`, aviso de revisión cruzada si el riesgo ≥ 7.5) · tests |
| SR-025 | Cada ejecución tiene un tiempo máximo configurable y una cancelación con escalado (SIGINT y luego SIGTERM). Los límites, la autenticación y la facturación detienen la tarea en `blocked`. | T20 | ✔ | `--timeout`, cancelación SIGINT→SIGTERM→SIGKILL, `blocked` · tests |
| SR-026 | Las dependencias de ejecución son mínimas (hoy 5) y se revisan antes de añadir otras. `npm ci --ignore-scripts` en la CI, lockfile versionado, `npm audit` sin vulnerabilidades altas (0 el 2026-10-07) y verificación de firmas del registro (solo aviso: npm < 10 rechaza claves rotadas en 2025). | T21 | ✔ | `.github/workflows/ci.yml` |
| SR-027 | Dependabot activo para npm y GitHub Actions. | T21, T22 | ✔ | `.github/dependabot.yml` |
| SR-028 | Las GitHub Actions se fijan por SHA de commit y el `GITHUB_TOKEN` tiene permisos mínimos (`contents: read`). | T22 | ✔ | `ci.yml` |
| SR-029 | La publicación en npm se hace con `--provenance` desde la CI, con 2FA y el nombre reservado. | T23 | ☐ release 0.1.0 | — |
| SR-030 | Los directorios de perfil se crean con 0700 fuera de directorios temporales, y `profile check` avisa si los permisos son más abiertos. | T25 | ✔ (M1) | `src/core/profiles.ts` |
| SR-032 | La UI web escucha solo en loopback (`127.0.0.1`/`::1`), en un puerto aleatorio. | UI web | ◐ (spike 004 ✔) | `spikes/004-ui-web` |
| SR-033 | Token de sesión de 256 bits, que pasa de la URL a una cookie `HttpOnly; SameSite=Strict`. Todas las rutas lo exigen y se compara en tiempo constante. | UI web | ◐ (spike 004 ✔) | idem |
| SR-034 | Validación de `Host` contra DNS rebinding y sin CORS. Las mutaciones exigen `X-Hilvan-Token` (CSRF). | UI web | ◐ (Host ✔; CSRF al añadir POST) | idem |
| SR-035 | CSP estricta sin `unsafe-inline` ni CDNs, y salida del agente siempre como texto (`textContent`). | UI web, T01 | ◐ (spike 004 ✔) | idem |
| SR-036 | La UI no expone lectura de archivos arbitrarios: solo `.hilvan/` y diffs de ramas `hilvan/*` saneados. | UI web | ◐ (spike 004 ✔) | idem |
| SR-031 | Los mensajes de error no incluyen valores de variables de entorno ni contenido de archivos de credenciales. | T12 | ✔ (revisión de código) | — |

## Proceso

- Cada SR se marca ✔ solo con una prueba automática o, si no es comprobable así, con una referencia de revisión.
- Las nuevas funcionalidades añaden sus amenazas al análisis y sus SR a esta tabla en el mismo PR.
- Las vulnerabilidades se reportan según [SECURITY.md](../../SECURITY.md).
