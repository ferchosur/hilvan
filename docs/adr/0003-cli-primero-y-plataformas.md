# ADR-0003 — CLI primero; Linux, Windows y WSL

- **Estado:** Accepted
- **Fecha:** 2026-10-06
- **Decisores:** Fernando Giraldo (aprobación), Claude Code (propuesta como arquitecto)
- **Relacionado:** D-014, Q-001, Q-002, Q-004

## Contexto

Hay que decidir la superficie inicial (escritorio, web local, terminal) y los sistemas operativos. El usuario trabaja en un Linux personal y en un PC Windows con WSL. Las herramientas similares suelen fallar en Windows (claude-squad #275).

## Opciones consideradas

1. **App de escritorio** (Electron/Tauri) — atractiva, pero mucho coste inicial y empaquetado por SO.
2. **UI web local primero** — buena para diffs y aprobaciones; exige servidor, frontend y autenticación local desde el día uno.
3. **CLI primero, UI web local después** — entrega valor rápido; las aprobaciones se resuelven con prompts interactivos y `$EDITOR`.

## Decisión

**CLI `hilvan`** como primera superficie, con interfaz propia (no embebida en otro agente). La **UI web local** llega en el incremento 2, reutilizando el mismo núcleo. Soporte de **Linux, Windows nativo y WSL**, con CI en `ubuntu-latest` y `windows-latest`; macOS "best effort".

## Consecuencias

- Positivas: MVP pequeño; la lógica queda separada de la presentación desde el inicio.
- Negativas: revisar diffs grandes en terminal es menos cómodo (mitigado con `git difftool` y la UI del incremento 2).
