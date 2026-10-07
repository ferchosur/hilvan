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
- **PC del trabajo (Windows + WSL):** dos cuentas de Claude (corporativa y personal) y una de Codex personal. Versiones por registrar.

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
- **Windows / WSL:** pendiente.

## Conclusión

Pendiente.

## Impacto

Confirma o modifica ADR-0006 y ADR-0005. Si Windows comparte credenciales entre directorios, los perfiles de Windows deberán usar API key o un usuario del sistema distinto.
