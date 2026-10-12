# Contexto para continuar el proyecto

Última actualización: 2026-10-12
Estado: **MVP completo, ya implementado en Go** (plan 002 terminado; el código TypeScript se retiró). 109 tests de Go en CI (ubuntu y windows) y e2e real con Claude Code y Codex. **23 de 25 criterios de aceptación verificados** con el binario de Go ([matriz](docs/plans/verificacion-mvp.md)): AC-015 está pendiente de la verificación con sesión en el equipo de validación y AC-009 diferido. Especificación 001 **v1.4**. Repositorio **público** en GitHub: `ferchosur/hilvan`. Release **0.1.0** publicada el 2026-10-11 (binarios, `SHA256SUMS` y attestations).

Fuentes de verdad:

- Requisitos: [`specs/001-product-requirements.md`](specs/001-product-requirements.md) (v1.4).
- Decisiones: [`docs/adr/`](docs/adr/) (ADR-0014: Go).
- Diseño: [`docs/design/`](docs/design/).
- Seguridad: [`docs/security/`](docs/security/).
- Planes: [`docs/plans/001-mvp.md`](docs/plans/001-mvp.md) y [`002-migracion-go.md`](docs/plans/002-migracion-go.md).
- Historia: [`docs/sessions/`](docs/sessions/).

## Producto

**Hilvan** (D-025): CLI open source (Apache-2.0) para coordinar agentes de IA entre proveedores y entre varias cuentas, con continuidad mediante contexto revisable. Cada cuenta es una identidad aislada; nada cambia de identidad sin el usuario. **Se posiciona como continuidad y gobierno, nunca como forma de sumar o eludir cuotas.** Debe funcionar en casi cualquier PC: binario único sin runtime, sin modelos locales ni terceros (NFR-013).

Entornos del usuario:

- **equipo de validación (Windows + WSL):**
  - la cuenta Claude **corporativa** en Windows nativo (login por defecto de PowerShell);
  - la Claude **personal** y Codex personal en WSL (Ubuntu, usuario `usuario-wsl`, CLIs con nvm);
  - los proyectos están en el disco de Windows (`%USERPROFILE%\Documents\claude`) y los usan ambos entornos.
  
  PowerShell corre en *Constrained Language Mode*, señal de una política de Windows.
- **Este Linux:** Claude y Codex con los logins por defecto.

## Qué funciona hoy (`hilvan …`)

| Comando | Qué hace |
|---|---|
| `doctor` | Comprueba git, claude, codex, WSL, la configuración y los perfiles `wsl` sin ruta del ejecutable. |
| `profile add/list/show/remove/login/check/set/tools` | Un perfil por cuenta y entorno (`CLAUDE_CONFIG_DIR`/`CODEX_HOME`, o `configDir: null` = login por defecto). Para `--launcher wsl`, descubre el ejecutable y el `PATH` de la distro (`profile set --rediscover` tras actualizar Node). `tools` descubre los MCP de la cuenta y `routing: reserved` reserva la cuenta corporativa para tareas que necesitan sus herramientas. |
| `init` | Crea `.hilvan/` en un repositorio git (ignorado por git). |
| `task new/list/show/log/run/recommend/requires/cancel/discard` | Brief → Criterio sugiere tipo, complejidad, clasificación y herramientas → ejecución en un worktree `hilvan/<tarea>` (rutas relativas, usable desde Windows y WSL) → consumo, límites, riesgo y cumplimiento → estados `blocked`/`uncertain` sin reintentos silenciosos; cancelación tras 20 min sin salida (watchdog `--idle-timeout`, motivo `inactivity`); límite duro de 60 min (`--timeout`). |
| `policy show/check` | Política en capas con `deny`/`require_approval`; `enforced` solo en la capa gestionada; la aprobación muestra el brief exacto. |
| `criterio ask/teach/stats/reset` | Motor de decisión local que aprende de las correcciones. |
| `handoff <tarea> --to <perfil>` | Paquete de continuidad con secretos redactados, editable, aprobado y guardado con sha256. |
| `review <tarea> [--by <perfil>]` / `accept` | Diff con riesgo, revisión cruzada de solo lectura y fusión `--no-ff` tras revisar el diff actual. |
| `stats`, `audit verify` | Uso por perfil y calibración de Criterio; cadena de hash de la auditoría. |
| `watch` | Panel en vivo y de solo lectura: tareas, ejecución en curso y última ejecución. `--once` imprime un cuadro; sin terminal se comporta igual. Nunca escribe en `.hilvan/`. |

Calidad: Go con `go run ./scripts/check` (gofmt, vet, tests), `govulncheck` en CI y 2 dependencias (`go.yaml.in/yaml/v3`, `golang.org/x/text`). `hilvan --version`: ~3 ms y ~5 MB (presupuesto como test). Los datos en disco son los mismos que escribía la versión TS: auditoría, `task.json` y modelo de Criterio.

## Cómo se usa en este PC

- Go 1.27.1 está en `~/.local/go` (el `go` del sistema es 1.17). Usa `export PATH="$HOME/.local/go/bin:$PATH"`.
- `hilvan` es un binario en `~/.local/bin/hilvan`, y `hv` un enlace a él. **Tras cambiar código:** `go build -trimpath -ldflags "-s -w" -o ~/.local/bin/hilvan ./cmd/hilvan`.
- Configuración real del usuario: `~/.config/hilvan/` (perfiles `claude-personal` y `codex`, con los logins por defecto de cada CLI). Los e2e usan carpetas del scratchpad.
- Regla de enrutamiento: la cuenta Claude **corporativa** solo recibe tareas que necesitan sus herramientas (`routing: reserved`); todo lo demás va a la **personal**.
- Antes de cada commit o fusión: `go run ./scripts/check` debe terminar con `check: ok`, y conviene `GOOS=windows go vet ./...`.
- `gh` está autenticado como `ferchosur` con permiso `workflow`. La CI se lanza a mano con `gh workflow run CI --ref <rama>`.

## Decisiones vigentes (detalle en spec y ADR)

**Go (ADR-0014, D-026; sustituye a ADR-0002)** · CLI primero (0003) · adaptadores de proceso no interactivo (0004) · un orquestador con lanzadores native/wsl/windows (0005) · identidad por directorio de configuración del CLI oficial, sin tocar tokens (0006) · persistencia en archivos (0007) · worktree por tarea y ejecución secuencial (0008) · política en capas y aprobación (0009) · Apache-2.0 (0010) · Criterio (0011) · UI: TUI propia y web mínima (0012, Accepted el 2026-10-11, con la adaptación a Go) · herramientas de la cuenta, perfil reservado y `--setting-sources user` (0013). De terceros tomamos ideas, no código (AGENTS.md).

## Hallazgos técnicos clave

- `claude -p` reporta las ventanas `five_hour` y `seven_day`; `claude auth status --json` identifica la cuenta; `codex login status` solo indica si hay sesión.
- Claude sale con 0 tras SIGINT: el estado se decide por los eventos. Hay que cerrar stdin. `CODEX_HOME` debe existir y estar fuera de `/tmp`.
- El login por defecto de Claude guarda la cuenta en `~/.claude.json`: reutilizarlo exige *eliminar* `CLAUDE_CONFIG_DIR`.
- **`codex exec` funciona aunque el servidor en segundo plano de Codex y el CLI tengan versiones distintas** (verificado el 2026-10-08). El aviso de *incompatible feature settings* solo afecta al CLI interactivo.
- **Spike 001 en el equipo de validación:**
  - `wsl.exe -- …` no carga el `PATH` del usuario y la advertencia de proxy sale en UTF-16; el lanzador ya lo resuelve.
  - Los worktrees creados desde Windows sin `--relative-paths` no funcionan desde WSL; Hilvan ya usa rutas relativas.
  - En Windows, `claude` es un `.exe`, así que BatBadBut no le afecta; los `.cmd` se protegen igualmente.
- **ADR-0013 verificado de nuevo con Go (2026-10-08):** un hook `SessionStart` y un `.mcp.json` hostiles no se cargan vía Hilvan; con `claude -p` directo, el hook sí se dispara.
- **Tareas de solo lectura con Claude:** `--permission-mode dontAsk` impide incluso `ls` o `git ls-files` (mejora propuesta en [propuestas pendientes](docs/research/2026-10-08-propuestas-pendientes.md) §4).
- **Mods de Claude Code** (2.1.287+): se ejecutan en `claude -p`, y los de usuario no los bloquea `--setting-sources user` (spike 005, `Planned`, en la rama `spike/005-mods-headless`).

## Diagnóstico del proyecto (2026-10-10)

- El renombre de `agent-orchestrator` a Hilvan está aplicado; el nombre anterior solo aparece en el historial y referencias históricas de la spec.
- Verificación local con Go 1.27.1: `go run ./scripts/check`, `GOOS=windows go vet ./...` y `go test -race ./...` pasan. `git fsck` y `git diff --check` también pasan; el árbol quedó limpio.
- Cobertura observada con `go test -cover ./...`: adapters 94.2 %, criterio 95.7 %, core 81.1 %, CLI 30.3 %, gitops 48.5 % y store 69.7 %.
- Release 0.1.0 publicada el 2026-10-11 (SR-029): workflow `Release` al subir tags `v*.*.*`, binarios para seis plataformas, `SHA256SUMS` y attestations. La firma de `hilvan.exe` queda aplazada: requiere un certificado de pago y la decisión es del usuario.
- README y modelo de amenazas alineados con spec v1.4 y 23/25 AC (2026-10-11, rama `docs/alinear-readme-v1.4`).
- `docs/plans/001-mvp.md`: la distribución de la release 0.1.0 pasa a GitHub Releases (ADR-0014).
- Hallazgos de bajo riesgo: `go.mod` marca como indirectas las dos dependencias que `go mod tidy -diff` identifica como directas; `stats` puede mostrar `0 runs ()` para un perfil que solo hizo revisiones; CI no ejecuta el detector de carreras.
- Mejora propuesta, no aprobada: fijar el modelo efectivo al iniciar una tarea y reutilizarlo al reanudar la sesión. Actualmente `RunTask` vuelve a tomar el modelo actual del perfil.
- Mejora propuesta, no aprobada: permitir herramientas de lectura explícitas en ejecuciones Claude `read`; hoy `dontAsk` impide incluso `ls` y `git ls-files`. Requiere requisito de seguridad y pruebas de que las escrituras siguen bloqueadas.
- Cobertura de tests menor en CLI/gitops/store es oportunidad de refuerzo, no defecto funcional confirmado.

## Decisiones pendientes del usuario

Detalle de las propuestas 2 y 3 en [propuestas pendientes](docs/research/2026-10-08-propuestas-pendientes.md).

1. ~~ADR-0012 — UI~~: aceptado el 2026-10-11 con la adaptación a Go; `hilvan watch` implementado. Pendiente: `hilvan ui` (web), después de validar `watch`.
2. ~~Watchdog de inactividad~~ (decidido el 2026-10-12): 20 minutos por defecto, configurable con `--idle-timeout` en `task run` y `review`. La revisión cruzada tiene límite duro de 30 minutos (`review --timeout`). Valor provisional hasta medir: cada ejecución guarda `maxSilenceSeconds` en `task.json`.
3. **Perfil ocupado y revisión cruzada:** qué pasa si el perfil ya tiene una ejecución, y si la revisión guarda un resumen de conclusiones o el transcript completo. Ver [nota AgentBridge](docs/research/2026-10-12-agentbridge.md).
4. **Visión conversacional** (Hilvan como REPL en lenguaje natural): propuesta de spec 002 con un "cerebro" elegido por clasificación. Sin decidir.
5. **Requisito "Hilvan no aumenta el consumo"** (propuesto): no cambiar de modelo ni de esfuerzo al reanudar, separar lectura y escritura de caché en `stats`.
6. **Q-012 — Monetización** (después del MVP).

## Próximos pasos

1. **Post de LinkedIn:** borrador listo en español e inglés. Lo revisa y publica el usuario; falta confirmar los hashtags.
2. **AC-015:** verificación con perfiles reales en el equipo de validación (Windows y WSL), spike 001 fase C. La hace el usuario; con eso se cierra.
3. **Medir silencios** en más tareas reales, incluidas las que ejecutan tests o instalaciones, y fijar el valor del watchdog con esos datos (`maxSilenceSeconds`). Después, la especificación si cambia el comportamiento.
4. **Spike 005 (mods):** ejecutarlo y decidir la mitigación.
5. **`hilvan ui` (ADR-0012):** después de validar `watch`. Al implementarlo, repetir en Go la verificación de SR-032…036, que hoy solo está hecha en Node (spike 004).
6. **Mejoras propuestas** (requieren aprobación y actualización de spec/SR): comandos de solo lectura en tareas `read`, resumen estructurado en el traspaso, catálogo de mensajes es/en, `stats` sin `0 runs ()`, lanzador SSH y fijar el modelo al reanudar.
7. **Limpieza:** ramas locales ya fusionadas; el spike 004 todavía cita ADR-0012 como "Proposed".

## Referencias

- [Propuestas pendientes](docs/research/2026-10-08-propuestas-pendientes.md) · [AgentBridge](docs/research/2026-10-12-agentbridge.md) · [Investigación de foros y herramientas](docs/research/2026-10-06-foros-y-herramientas.md) · [UI terminal y web](docs/research/2026-10-07-ui-terminal-y-web.md) · [claude-router](docs/research/2026-10-07-claude-router.md) · [estado y oportunidades](docs/research/estado-y-oportunidades.md)
- [Análisis de riesgos](docs/security/analisis-de-riesgos.md) · [Requisitos SR](docs/security/requisitos.md) · [Patrones](docs/design/patrones.md)
- [Glosario](docs/glosario.md) · [Bitácora](docs/sessions/)
- Clones de referencia: `../orcha` (`b647cab`), `../herdr` y `../claude-router` (`7599fb5`).
