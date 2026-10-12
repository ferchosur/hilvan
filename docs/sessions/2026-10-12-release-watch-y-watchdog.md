# 2026-10-11/12 — Publicación, `watch`, watchdog y nota sobre AgentBridge

- **Agente/herramienta:** Claude Code (Haiku 5.5), con el usuario presente.
- **Commits relevantes en `main`:** `4d2aec7` (nota AgentBridge), `bfa5652` (watchdog), `b0f49bb` (`watch`), `3c0c1b3` (ADR-0012), `fe128b1` (release 0.1.0).

## Objetivo de la sesión

Cerrar el MVP para publicarlo: repositorio público, primera release, borrador de anuncio en LinkedIn, `hilvan watch` como UI antes de publicar y una revisión de ideas de otros proyectos.

## Hecho

- **Documentación alineada con v1.4 y 23/25 AC** (rama `docs/alinear-readme-v1.4`): READMEs, modelo de amenazas (T4) y plan 001.
- **Workflow de release** (`.github/workflows/release.yml`): al subir un tag `v*.*.*` compila seis binarios, genera `SHA256SUMS` y attestations de procedencia. Una ejecución manual solo compila y comprueba.
- **Historial reescrito** para retirar referencias al entorno de trabajo y al usuario de WSL. Se sustituyeron `PC del trabajo`, `PC de trabajo`, `PC corporativo`, `fernandog` y `WDAC/AppLocker`. Se hizo una copia de seguridad completa antes. Se publicaron las ramas y etiquetas reescritas.
- **Repositorio público** y **release `v0.1.0` publicada** el 2026-10-11: seis binarios, `SHA256SUMS` y attestations. La primera ejecución del tag falló porque las attestations no están disponibles en repositorios privados de cuenta personal; tras hacerlo público, se recreó el tag y el workflow pasó.
- **ADR-0012 aceptado** con la adaptación a Go: `watch` con renderizador ANSI propio y `ui` con `net/http` y `embed`, sin Bubble Tea. Se corrigió su texto: la verificación de SR-032…036 es del spike 004, escrito en Node, y debe repetirse en Go.
- **`hilvan watch`** (`feat/watch`, fusionada): panel de solo lectura, `--once` y `--interval`. No usa `openStore` porque ese escribe en `.hilvan/`; una prueba verifica que no hay escrituras. En un pseudo-terminal verificamos cuatro refrescos con borrado de pantalla.
- **Watchdog de inactividad** (`feat/watchdog-ejecucion`, fusionada): cancela una ejecución sin salida durante `adapters.DefaultInactivity` (10 minutos). Cada chunk de salida reinicia el contador. Se aplica a `task run` y a la revisión cruzada.
- **Ejecuciones reales** en un repositorio de prueba del scratchpad: una tarea de solo lectura con `claude-personal` y otra con `codex`. Ambas terminaron como `completed` y el watchdog no interfirió. El disparo real de los 10 minutos no se ha probado con un agente real.
- **Nota de investigación sobre AgentBridge** (`docs/research/2026-10-12-agentbridge.md`, fusionada). Ideas reutilizables, sin copiar código: solo conclusiones del agente, busy-guard, watchdog, coste de cada envío. Descartado: relevo de cuota entre cuentas (eludir límites) y envíos automáticos entre cuentas sin aprobación (AC-003, D-003).
- **Borrador de LinkedIn** en un archivo local del scratchpad (español e inglés), con revisión en el navegador. Lo publica el usuario.

## Decisiones

- El repositorio se hace público; la release se marca como estable, no como prerelease.
- La firma de `hilvan.exe` se aplaza hasta nueva orden por su coste mensual.
- `hilvan ui` queda para después de validar `watch`.
- Los 10 minutos del watchdog son una elección provisional, no un requisito; pendiente de confirmar.

## Problemas

- Un comando `git push --force` fue denegado por el control de operaciones destructivas. El usuario lo ejecutó directamente.
- Un comando con un error tipográfico (`git git`) se denegó entero; no tuvo efecto.
- El borrador HTML tenía comillas invertidas dentro de un literal de JavaScript, lo que rompía el script; se corrigió.
- Los commits antiguos pueden seguir accesibles en caché de GitHub (refs de la PR #1). No se puede eliminar desde el repositorio.
- Los emails de autor de los commits siguen siendo los personales del usuario; no se cambiaron.

## Qué sigue

Ver «Próximos pasos» en [CONTEXTO.md](../../CONTEXTO.md): post de LinkedIn, AC-015 con el equipo de validación, decisiones sobre watchdog, perfil ocupado y revisión cruzada, y `hilvan ui` después de validar `watch`.
