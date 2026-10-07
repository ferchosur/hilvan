# ADR-0005 — Un único orquestador con lanzadores por destino

- **Estado:** Accepted
- **Fecha:** 2026-10-06
- **Decisores:** Fernando Giraldo (aprobación), Claude Code (propuesta como arquitecto)
- **Relacionado:** D-011, D-017, FR-013, NFR-009, Q-010, AC-015

## Contexto

Un perfil puede vivir en Windows nativo, en una distribución WSL o (en el caso del usuario) en otra máquina. Hay que decidir si el orquestador lanza procesos en otros entornos o si hay un runner en cada uno.

## Opciones consideradas

1. **Runner residente por entorno** — aislamiento fuerte, pero instalación, actualización y protocolo de red por entorno.
2. **Un orquestador con lanzadores** — el destino declara cómo envolver el comando: `native` (directo), `wsl` (`wsl.exe -d <distro> --cd <dir> -- <cmd>`), `windows` desde WSL (interop) y, más tarde, `ssh` (`ssh <host> -- <cmd>`). Stdio viaja por la tubería en todos los casos.

## Decisión

**Opción 2.** El *lanzador* es una función pura que transforma `(destino, comando, args, env, cwd)` en la invocación final; las variables de entorno del perfil (`CLAUDE_CONFIG_DIR`, `CODEX_HOME`) se fijan **dentro** del entorno destino (para WSL vía `env VAR=… cmd` o `WSLENV`). Las rutas se traducen con `wslpath` cuando cruzan entornos. `ssh` se implementa en el incremento 2 con el mismo mecanismo.

## Consecuencias

- Positivas: nada que instalar en cada entorno salvo el propio CLI del agente; un solo lugar de auditoría.
- Negativas: el orquestador debe poder ver los repositorios de cada destino (rutas traducidas); entornos remotos requieren acceso SSH ya configurado.
- Seguimiento: si las tuberías cruzadas resultan poco fiables, reconsiderar un runner (ADR-0002 deja el contrato listo).
