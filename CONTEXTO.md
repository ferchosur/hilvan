# Contexto para continuar el proyecto

Última actualización: 2026-10-08
Estado: **MVP completo, ya implementado en Go** (plan 002 terminado; el código TypeScript se retiró). 109 tests de Go en CI (ubuntu y windows) y e2e real con Claude Code y Codex. **23 de 25 criterios de aceptación verificados** con el binario de Go ([matriz](docs/plans/verificacion-mvp.md)): AC-015 está pendiente de la verificación con sesión en el equipo de validación y AC-009 diferido. Especificación 001 **v1.4**. Repositorio privado en GitHub: `ferchosur/hilvan`.

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
| `task new/list/show/log/run/recommend/requires/cancel/discard` | Brief → Criterio sugiere tipo, complejidad, clasificación y herramientas → ejecución en un worktree `hilvan/<tarea>` (rutas relativas, usable desde Windows y WSL) → consumo, límites, riesgo y cumplimiento → estados `blocked`/`uncertain` sin reintentos silenciosos. |
| `policy show/check` | Política en capas con `deny`/`require_approval`; `enforced` solo en la capa gestionada; la aprobación muestra el brief exacto. |
| `criterio ask/teach/stats/reset` | Motor de decisión local que aprende de las correcciones. |
| `handoff <tarea> --to <perfil>` | Paquete de continuidad con secretos redactados, editable, aprobado y guardado con sha256. |
| `review <tarea> [--by <perfil>]` / `accept` | Diff con riesgo, revisión cruzada de solo lectura y fusión `--no-ff` tras revisar el diff actual. |
| `stats`, `audit verify` | Uso por perfil y calibración de Criterio; cadena de hash de la auditoría. |

Calidad: Go con `go run ./scripts/check` (gofmt, vet, tests), `govulncheck` en CI y 2 dependencias (`go.yaml.in/yaml/v3`, `golang.org/x/text`). `hilvan --version`: ~3 ms y ~5 MB (presupuesto como test). Los datos en disco son los mismos que escribía la versión TS: auditoría, `task.json` y modelo de Criterio.

## Cómo se usa en este PC

- Go 1.27.1 está en `~/.local/go` (el `go` del sistema es 1.17). Usa `export PATH="$HOME/.local/go/bin:$PATH"`.
- `hilvan` es un binario en `~/.local/bin/hilvan`, y `hv` un enlace a él. **Tras cambiar código:** `go build -trimpath -ldflags "-s -w" -o ~/.local/bin/hilvan ./cmd/hilvan`.
- Configuración real del usuario: `~/.config/hilvan/` (aún sin perfiles en este PC). Los e2e usan carpetas del scratchpad.
- Regla de enrutamiento: la cuenta Claude **corporativa** solo recibe tareas que necesitan sus herramientas (`routing: reserved`); todo lo demás va a la **personal**.
- Antes de cada commit o fusión: `go run ./scripts/check` debe terminar con `check: ok`, y conviene `GOOS=windows go vet ./...`.
- `gh` está autenticado como `ferchosur` con permiso `workflow`. La CI se lanza a mano con `gh workflow run CI --ref <rama>`.

## Decisiones vigentes (detalle en spec y ADR)

**Go (ADR-0014, D-026; sustituye a ADR-0002)** · CLI primero (0003) · adaptadores de proceso no interactivo (0004) · un orquestador con lanzadores native/wsl/windows (0005) · identidad por directorio de configuración del CLI oficial, sin tocar tokens (0006) · persistencia en archivos (0007) · worktree por tarea y ejecución secuencial (0008) · política en capas y aprobación (0009) · Apache-2.0 (0010) · Criterio (0011) · UI: TUI propia y web mínima (0012, **Proposed**) · herramientas de la cuenta, perfil reservado y `--setting-sources user` (0013). De terceros tomamos ideas, no código (AGENTS.md).

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

## Decisiones pendientes del usuario

Detalle de las propuestas 2 y 3 en [propuestas pendientes](docs/research/2026-10-08-propuestas-pendientes.md).


1. **ADR-0012 — UI:** aceptar `hilvan watch` (TUI propia) y `hilvan ui` (web local mínima).
2. **Visión conversacional** (Hilvan como REPL en lenguaje natural): propuesta de spec 002 con un "cerebro" elegido por clasificación y las operaciones de Hilvan como herramientas MCP. Sin decidir.
3. **Requisito "Hilvan no aumenta el consumo"** (propuesto): no cambiar de modelo ni de esfuerzo al reanudar, separar lectura y escritura de caché en `stats` y medir con un benchmark directo frente a Hilvan.
4. **Q-012 — Monetización** (después del MVP).

## Próximos pasos

1. **Spike 001, fase C con sesión** (la ejecuta el usuario; el filtro de permisos de la sesión remota la bloqueó):
   - `auth status` de `claude-personal` y `codex-personal` en WSL;
   - `claude.exe` lanzado desde WSL en la carpeta compartida.
   
   Después, perfiles reales en el equipo de validación: `hilvan profile add --launcher wsl …` y `profile tools claude-corp --apply`. Con eso se cierra AC-015.
2. Spike 005 (mods): ejecutarlo y decidir la mitigación.
3. Mejoras:
   - comandos de solo lectura permitidos en tareas `read` (`--allowedTools`);
   - resumen estructurado del agente de origen en el traspaso;
   - catálogo de mensajes es/en;
   - `stats` sin `0 runs ()` para perfiles que solo revisaron;
   - `hilvan watch`/`ui` si se acepta ADR-0012;
   - lanzador SSH.
4. Release 0.1.0:
   - binarios por plataforma con *attestations* (SR-029);
   - comprobar si la política de Windows del equipo de validación exige firmar `hilvan.exe`;
   - README final y post de lanzamiento (borrador en [propuestas pendientes](docs/research/2026-10-08-propuestas-pendientes.md) §5).

## Referencias

- [Propuestas pendientes](docs/research/2026-10-08-propuestas-pendientes.md) · [Investigación de foros y herramientas](docs/research/2026-10-06-foros-y-herramientas.md) · [UI terminal y web](docs/research/2026-10-07-ui-terminal-y-web.md) · [claude-router](docs/research/2026-10-07-claude-router.md) · [estado y oportunidades](docs/research/estado-y-oportunidades.md)
- [Análisis de riesgos](docs/security/analisis-de-riesgos.md) · [Requisitos SR](docs/security/requisitos.md) · [Patrones](docs/design/patrones.md)
- [Glosario](docs/glosario.md) · [Bitácora](docs/sessions/)
- Clones de referencia: `../orcha` (`b647cab`), `../herdr` y `../claude-router` (`7599fb5`).
