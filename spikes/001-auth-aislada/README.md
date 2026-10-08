# Spike 001 — Autenticación aislada de varias cuentas por entorno

- **Estado:** In Progress (Linux verificado parcialmente; Windows/WSL pendiente)
- **Fecha inicio:** 2026-10-06
- **Límite de tiempo:** 3 h
- **Relacionado:** D-018, NFR-008, AC-002, AC-015, AC-017, ADR-0005, ADR-0006

## Pregunta

¿Pueden convivir dos cuentas de Claude Code (y una de Codex) en el mismo PC, en Windows nativo y en WSL, cada una con su directorio de configuración, sin mezclar credenciales y con verificación automática de la identidad activa?

## Hipótesis

Sí, usando `CLAUDE_CONFIG_DIR` / `CODEX_HOME` por perfil. Riesgo: en Windows o macOS, el CLI podría guardar credenciales en el almacén del sistema y compartirlas entre directorios.

## Entorno

- **Linux personal:** Claude Code 2.1.292 y codex-cli 0.160.1.
- **equipo de validación (Windows + WSL):** dos cuentas de Claude (corporativa y personal) y una de Codex personal. Versiones por registrar.

## Procedimiento (lo ejecuta el usuario; requiere login interactivo)

1. Crear directorios fuera de `/tmp`, por ejemplo `~/.hilvan-profiles/claude-corp`, `~/.hilvan-profiles/claude-personal` y `~/.hilvan-profiles/codex-personal`.
2. Hacer login en cada uno con el CLI oficial:
   - `CLAUDE_CONFIG_DIR=~/.hilvan-profiles/claude-corp claude`, luego `/login` y salir.
   - Repetir con `claude-personal`.
   - `mkdir -p … && CODEX_HOME=~/.hilvan-profiles/codex-personal codex login`.
3. Verificar cada perfil con `CLAUDE_CONFIG_DIR=… claude auth status`: `email` y `orgName` deben ser distintos. Con un directorio vacío debe dar `loggedIn: false`.
4. Ejecutar `claude -p "di OK" --output-format json` en cada perfil y comprobar que `rate_limit_event` muestra ventanas de uso distintas.
5. Repetir los pasos 1 a 4 en **Windows nativo (PowerShell)** con `$env:CLAUDE_CONFIG_DIR=…`, y desde Windows hacia WSL con `wsl.exe -d <distro> --cd <dir> -- env CLAUDE_CONFIG_DIR=… claude auth status`.
6. Cerrar sesión en un perfil y confirmar que el otro sigue con la sesión iniciada.

No registrar emails, organizaciones ni rutas reales en este documento: usar "cuenta A" y "cuenta B".

## Resultados

- **Linux (2026-10-06, spike 002):** un directorio vacío da `loggedIn: false` y el directorio por defecto da sesión iniciada. `claude auth status` identifica la cuenta y la organización. Falta probarlo con dos cuentas reales.
- **Linux (2026-10-06, M1):** **el login por defecto de Claude no equivale a `CLAUDE_CONFIG_DIR=~/.claude`.** Sin la variable, la cuenta (email y organización) se lee de `~/.claude.json`, en la carpeta personal. Con la variable apuntando a `~/.claude`, Claude busca `~/.claude/.claude.json`, que no existe: la sesión sigue activa pero la cuenta aparece como `null`, y además el CLI crea ese archivo. Decisión: un perfil "login por defecto" usa `configDir: null` y el adaptador **elimina** la variable del entorno en vez de fijarla. En Codex, `CODEX_HOME=~/.codex` sí equivale al valor por defecto, pero se aplica la misma regla por coherencia.
- **equipo de validación, fase A sin login (2026-10-07, ejecutada por una sesión de Claude Code en WSL vía Remote Control):** entorno WSL2 Ubuntu 26.04, Claude Code 2.1.293 en WSL y en Windows nativo, Node 24, Git 2.53/2.55, Go 1.26. **Codex no está instalado** en ese equipo.
  - Directorio de perfil vacío → `loggedIn: false` tanto en WSL como en Windows nativo (`$env:CLAUDE_CONFIG_DIR`). Sin la variable, la sesión por defecto sigue activa (`authMethod: claude.ai`). Al menos con el directorio vacío, no se hereda la sesión por defecto (tampoco en Windows, donde se temía el almacén de credenciales del sistema).
  - `claude auth status` **escribe** en el directorio del perfil (`.claude.json`, `.claude.json.lock`, `backups/`) aunque no haya sesión. `hilvan profile check` no es de solo lectura sobre el directorio del perfil.
  - **Windows → WSL:** sin resultado. `wsl.exe` emitió por stderr una advertencia **en UTF-16** ("Se detectó una configuración de proxy localhost, pero no se refleja en WSL. WSL en modo NAT…") y el reintento lo bloqueó el clasificador de permisos de la sesión remota. Impacto en el lanzador `wsl` (ADR-0005): debe decodificar la salida de `wsl.exe` en UTF-16LE y no confundir esas advertencias con la salida del agente. Pendiente de ejecutarlo en persona.
  - **Codex en WSL** (instalado el 2026-10-07, codex-cli 0.161.0): con `CODEX_HOME` apuntando a un directorio vacío, `codex login status` responde "Not logged in" (código 1); sin la variable hay sesión (código 0). `login status` crea `tmp/` dentro del directorio del perfil. En Windows nativo no hay Codex.
  - **Windows → WSL, ejecutado por el usuario:** `wsl.exe -d <distro> -- env CLAUDE_CONFIG_DIR=… claude auth status --json` falla con `env: 'claude': No such file or directory`. `wsl.exe -- <cmd>` no carga `~/.profile` ni `~/.bashrc`, así que el `PATH` no incluye la carpeta de instalación del CLI (p. ej. `~/.local/bin`). La advertencia de proxy en UTF-16 vuelve a aparecer en stderr. **Impacto en el lanzador `wsl` (`src/launchers/index.ts`):** construye exactamente esa orden y falla salvo que el perfil tenga `executable` con ruta absoluta. Corrección propuesta (en Go, plan 002 G2): al crear el perfil, resolver la ruta absoluta una sola vez con un shell de login (`wsl.exe -d <distro> -- bash -lc 'command -v claude'`), guardarla en `destination.executable` y ejecutar siempre sin shell. `hilvan doctor` debe avisar si falta.
  - **Windows → WSL con `PATH` explícito (usuario, 2026-10-07):** el CLI está instalado con **nvm**, así que su ruta incluye la versión de Node y es un script `#!/usr/bin/env node`. Una ruta absoluta sola no basta: también hace falta que `node` esté en el `PATH`. Con `env PATH=<bin de nvm>:/usr/local/bin:/usr/bin:/bin CLAUDE_CONFIG_DIR=<perfil vacío> claude auth status --json` el resultado es `loggedIn: false`, `authMethod: none`, y `configDirectory` apunta al perfil: **el aislamiento funciona también de Windows a WSL**. La advertencia de proxy sigue saliendo por stderr y el JSON por stdout. **Diseño para G2:** el perfil `wsl` guarda el ejecutable y un `PATH` explícito, descubiertos al crearlo con un shell interactivo (`bash -ic`, porque el `.bashrc` de Ubuntu sale antes de cargar nvm si el shell no es interactivo). Cada ejecución pasa ese `PATH` con `env`, sin shell. `doctor`/`profile check` detectan si la ruta dejó de existir tras actualizar Node.
- **Windows / WSL, con login (fases B y C):** pendiente.

## Conclusión

Pendiente.

## Impacto

Confirma o modifica ADR-0006 y ADR-0005. Si Windows comparte credenciales entre directorios, los perfiles de Windows deberán usar API key o un usuario del sistema distinto.
