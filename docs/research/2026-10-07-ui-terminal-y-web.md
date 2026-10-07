# Investigación: interfaz de terminal (TUI) y web local

- **Fecha:** 2026-10-07
- **Propósito:** elegir cómo dar a Hilvan una interfaz más rica (panel de tareas en vivo, revisión de diffs, aprobaciones) sin romper NFR-013 (ligereza) ni el modelo de seguridad.
- **Datos:** tamaños y dependencias de npm consultados el 2026-10-07 (`npm view <pkg> dist.unpackedSize dependencies`). Costes de arranque de Node medidos en el [spike 003](../../spikes/003-rendimiento/).

## 1. Qué necesita Hilvan de una interfaz

1. **Panel en vivo:** tareas, estado (`running`, `blocked`, `uncertain`…), perfil, consumo y ventanas de límite. Es la petición más repetida en los foros: "no tener que buscar el agente atascado" (herdr).
2. **Streaming** de la salida del agente durante una ejecución.
3. **Revisión de diffs** antes de aceptar (M5), legible y con marcas de riesgo de Criterio.
4. **Aprobaciones** de paquetes de traspaso (M4): vista previa, edición y aprobar o cancelar, con el contenido exacto que se enviará.
5. Funcionar en **Windows, WSL y Linux**, en terminales modestas y por SSH.

## 2. Terminal (TUI)

| Opción | Tamaño (npm) | Deps | Mantenimiento | Valoración para Hilvan |
|---|---:|---:|---|---|
| **@clack/prompts** (en uso) | 91 KB | 4 | activo (09/2026) | Excelente para preguntas puntuales (crear perfil, confirmar). No sirve para paneles a pantalla completa. |
| @inquirer/prompts | 24 KB (+10 deps) | 10 | activo | Alternativa a clack. No aporta nada nuevo. |
| **Ink 8 + React 19** | 605 KB + 174 KB | 23 | activo | Modelo de componentes muy productivo y usado por CLIs conocidos, pero React añade ~50–80 ms de arranque y decenas de dependencias. **Demasiado pesado** para NFR-013. |
| terminal-kit | 4 MB | 8 | activo | Completo, pero grande y con un API amplio. |
| neo-blessed / blessed | 1.6 MB / — | 0 | **abandonados** (2022/2024) | Descartados por mantenimiento y por Windows. |
| @opentui/core | 13.4 MB | 5 | muy activo | Motor nativo (Zig) de alto rendimiento. Peso excesivo y binarios nativos por plataforma. |
| **ANSI propio (sin dependencias)** | ~5–10 KB de código | 0 | nuestro | Pantalla alternativa, redibujado por diferencias, teclas básicas. Suficiente para un panel de solo lectura y una lista navegable. |
| *(Go)* Bubble Tea + Lip Gloss | — | — | muy activo | La referencia en TUIs ligeras. Relevante si Q-013 decide migrar a Go. |
| *(Rust)* Ratatui | — | — | muy activo | Igual de bueno, pero Rust no se contempla en Q-013. |

**Conclusión TUI:**

- **Mantener @clack/prompts** para las interacciones.
- Construir un **`hilvan watch`** con un renderizador ANSI propio de cero dependencias (pantalla alternativa, refresco a 4–10 Hz, salida siempre saneada con SR-001).
- Ink queda descartado por peso.
- Si se migra a Go, usar Bubble Tea.

## 3. Web local

### Servidor

| Opción | Tamaño | Deps | Valoración |
|---|---:|---:|---|
| **`node:http` (stdlib)** | 0 | 0 | Suficiente: unas pocas rutas JSON y un canal SSE. Sin superficie de ataque añadida. |
| Hono | 1.3 MB | 0 | Router excelente y portable (Node, Bun, Deno, edge). Útil si el API crece. |
| Express 5 | 73 KB + 28 deps | 28 | Muchas dependencias transitivas. Descartado. |
| Fastify | 2.9 MB | 15 | Rápido, pero más de lo necesario. |

### Cliente

| Opción | Tamaño (min+gzip aprox.) | Valoración |
|---|---:|---|
| **JS nativo + Web Components** | 0 | Sin build ni dependencias. Basta para un panel y formularios de aprobación. |
| Preact 11 (+ signals) | ~4–5 KB | Componentes cómodos con muy poco peso. Buen paso siguiente si la UI crece. |
| htmx 2 | ~16 KB | Encaja con HTML servido por el servidor. Bueno para formularios de aprobación. |
| Lit 3 | ~6 KB | Web Components con plantillas. Alternativa a Preact. |
| Alpine.js / petite-vue | 15 KB / 7 KB | petite-vue está sin mantenimiento desde 2022. |

### Transporte en vivo

**SSE** (`text/event-stream`) para las actualizaciones de tareas y la salida de los agentes. Es unidireccional, funciona sobre HTTP/1.1 y no necesita WebSocket. Las acciones (aprobar, cancelar) van por `POST` con token.

### Seguridad de la UI web (nuevos SR propuestos)

| SR | Requisito |
|---|---|
| SR-032 | Escuchar solo en `127.0.0.1` / `::1`, en un puerto aleatorio. Nunca `0.0.0.0`. |
| SR-033 | Token aleatorio de 256 bits por sesión, entregado en la URL de arranque y luego en una cookie `HttpOnly; SameSite=Strict`. Todas las rutas lo exigen. |
| SR-034 | Validar la cabecera `Host` contra `127.0.0.1:<puerto>` y `localhost:<puerto>` (defensa contra DNS rebinding), sin CORS. Las mutaciones exigen además una cabecera `X-Hilvan-Token` (CSRF). |
| SR-035 | CSP estricta (`default-src 'self'`; sin `unsafe-inline`; sin CDNs), recursos servidos desde el propio binario, y la salida del agente insertada como texto (`textContent`), nunca como HTML. |
| SR-036 | La UI no expone rutas que lean archivos arbitrarios. Solo datos de `.hilvan/` y diffs de ramas `hilvan/*`, ya saneados. |

## 4. Recomendación

1. **Incremento 2a — `hilvan watch` (TUI propia, 0 dependencias):** panel en vivo de tareas y ejecuciones, leyendo `.hilvan/` y la auditoría. Coste bajo y valor alto.
2. **Incremento 2b — `hilvan ui` (web local):** `node:http` + SSE + JS nativo/Web Components, con SR-032…SR-036. Pasar a Preact o htmx solo si la complejidad lo justifica. La revisión de diffs (M5) y las aprobaciones (M4) llegan primero en CLI y luego en la web, usando los mismos casos de uso de `core/`.
3. Si **Q-013 = Go**: Bubble Tea para la TUI y `net/http` + `embed` para servir la web desde el mismo binario. El diseño (SSE, token, CSP) es idéntico.

Se registra como **ADR-0012 (Proposed)**. Prototipo de referencia: `spikes/004-ui-web/`.
