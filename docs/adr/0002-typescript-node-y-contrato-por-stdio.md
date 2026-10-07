# ADR-0002 — TypeScript sobre Node.js y contrato JSON por stdio

- **Estado:** Accepted
- **Fecha:** 2026-10-06
- **Decisores:** Fernando Giraldo (aprobación), Claude Code (propuesta como arquitecto)
- **Relacionado:** D-013, NFR-005, NFR-012

## Contexto

El producto lanza y controla CLIs de agentes (Claude Code, Codex) en Linux, Windows nativo y WSL. Será open source y se mantendrá con poca gente. El usuario preguntó si se podía combinar TypeScript con Go.

## Opciones consideradas

1. **TypeScript/Node** — mismo ecosistema que los CLIs objetivo; multiplataforma sin compilar; un solo lenguaje para CLI y futura UI web.
2. **Python** — legible; empaquetado en Windows y UI web requieren más piezas.
3. **Go** — binario único, buen control de procesos; UI web como proyecto aparte.
4. **TypeScript + Go desde el inicio** — dos toolchains, dos CI, más superficie de errores.

## Decisión

**TypeScript (ESM, `strict`) sobre Node.js ≥ 22** para todo el MVP. El núcleo se comunica con los adaptadores mediante **eventos JSON normalizados** que pueden viajar por stdio, de modo que en el futuro un runner en Go (u otro lenguaje) pueda ejecutarse dentro de WSL o en una máquina remota hablando el mismo contrato.

Herramientas: npm, tsup (build), Vitest (tests), Biome (lint + formato), zod (validación), commander (CLI), @clack/prompts (interacción), yaml.

> **Nota de implementación (2026-10-06, M0):** el build usa `tsc` (TypeScript 7, compilador nativo) en lugar de tsup, cuya compatibilidad con TS 7 no está garantizada; se añade `cross-spawn` para lanzar de forma segura los *shims* `.cmd` de los CLIs instalados con npm en Windows. No cambia la decisión.

## Consecuencias

- Positivas: una sola cadena de herramientas; contribuciones accesibles; los mismos tipos sirven al CLI y a la UI web.
- Negativas: el usuario necesita Node instalado (los CLIs objetivo ya lo requieren o lo traen).
- Seguimiento: si se necesita un runner remoto ligero, evaluarlo en Go con este contrato (nuevo ADR).
