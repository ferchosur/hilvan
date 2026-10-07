# ADR-0006 — Identidad por directorio de configuración del CLI oficial

- **Estado:** Accepted
- **Fecha:** 2026-10-06
- **Decisores:** Fernando Giraldo (aprobación), Claude Code (propuesta como arquitecto)
- **Relacionado:** D-001, D-018, NFR-001, NFR-008, NFR-010, NFR-011, AC-002, AC-017, R-006

## Contexto

Varias cuentas del mismo proveedor deben convivir sin mezclarse. Los términos de Anthropic (consultados 2026-10-06) prohíben que un tercero ofrezca el login de Claude.ai o almacene/intermedie tokens, pero permiten que el usuario inicie sesión en el binario oficial sin modificar.

## Decisión

- Cada perfil apunta a un **directorio de configuración propio** del CLI: `CLAUDE_CONFIG_DIR` para Claude Code, `CODEX_HOME` para Codex. El orquestador solo fija esa variable al lanzar el proceso.
- `hilvan profile login <perfil>` abre el **CLI oficial en modo interactivo** con ese directorio para que el usuario complete el login del proveedor. El orquestador **no lee, copia ni almacena** credenciales ni tokens.
- Perfiles con API key: el perfil guarda el **nombre** de la variable de entorno (p. ej. `apiKeyEnv: WORK_ANTHROPIC_KEY`), nunca su valor.
- Herramientas por perfil: Claude se lanza con `--strict-mcp-config --mcp-config <archivo del perfil>`; Codex con la configuración de su `CODEX_HOME`. Lo que no está declarado en el perfil no se carga.
- `hilvan profile check` verifica la identidad activa con los comandos de estado de cada CLI (se concreta en el spike 001).

## Consecuencias

- Positivas: cumplimiento de términos; aislamiento probado por la comunidad; ningún secreto en archivos del proyecto.
- Negativas: el usuario hace login una vez por perfil; si el proveedor usa el llavero del sistema por cuenta, el aislamiento por directorio debe verificarse (spike 001).
