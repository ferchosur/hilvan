# ADR-0012 — TUI propia sin dependencias y web local mínima

- **Estado:** Accepted (2026-10-11), con la adaptación a Go de la sección de decisión
- **Fecha:** 2026-10-07
- **Decisores:** propuesto por Claude Code; decide Fernando Giraldo
- **Relacionado:** ADR-0003, NFR-013, [investigación de UI](../research/2026-10-07-ui-terminal-y-web.md), SR-032…SR-036

## Contexto

El CLI cubre el MVP, pero los usuarios valoran ver el estado de varios agentes de un vistazo, seguir su salida en vivo y revisar diffs y aprobaciones con comodidad. Hilvan debe seguir siendo ligero (NFR-013) y seguro (análisis de riesgos).

## Opciones consideradas

1. **Ink + React**: productivo, pero añade ~800 KB, 23 dependencias y decenas de ms de arranque.
2. **TUI completa de terceros** (terminal-kit, OpenTUI): pesadas o con binarios nativos.
3. **TUI propia de ANSI** para `watch`, más **web local mínima** (`node:http`, SSE, JS nativo/Web Components), con @clack/prompts para las preguntas.
4. **App de escritorio** (Electron o Tauri): pesada y fuera del enfoque CLI-first.

## Decisión

**Opción 3**, en dos pasos:

- `hilvan watch`: panel en vivo de solo lectura.
- `hilvan ui`: web en `127.0.0.1` con token por sesión, validación de `Host`, CSP estricta y salida insertada como texto.

Ambos usan los mismos casos de uso de `core/`.

**Adaptación a Go (Q-013, ADR-0014).** Se aceptó sin dependencias nuevas: `watch` con un renderizador ANSI propio sobre `os`/`bufio`, y `ui` con `net/http` y `embed` de la biblioteca estándar. No se usa Bubble Tea: añadiría una dependencia externa que justificar, y el panel de solo lectura no la necesita. Los requisitos de seguridad de `ui` (SR-032…SR-036) se aplican tal cual. El spike 004 que los verificó está en Node (`spikes/004-ui-web/server.mjs`), así que su verificación debe repetirse en la implementación en Go antes de marcarlos como cumplidos.

## Consecuencias

- Positivas: cero dependencias nuevas en la UI, arranque intacto, superficie de ataque pequeña y controlada.
- Negativas: hay que mantener un pequeño renderizador ANSI y componentes propios. Se reconsiderará Preact (~4 KB) si la web crece.
