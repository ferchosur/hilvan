# Spike 004 — Web local mínima de solo lectura

- **Estado:** Done
- **Fecha:** 2026-10-07
- **Relacionado:** ADR-0012 (Proposed), SR-032…SR-036, [investigación de UI](../../docs/research/2026-10-07-ui-terminal-y-web.md)

## Pregunta

¿Se puede ofrecer un panel web en vivo de las tareas con **cero dependencias** (`node:http`, SSE y JS nativo) y con los controles de seguridad propuestos para una UI local?

## Procedimiento

```bash
npm run build
cd <proyecto con .hilvan/>
node <hilvan>/spikes/004-ui-web/server.mjs     # imprime http://127.0.0.1:<puerto>/?t=<token>
```

Pruebas con `curl` sobre el proyecto e2e del 2026-10-07 (dos tareas reales):

| # | Prueba | Resultado |
|---|---|---|
| 1 | Petición sin token | **401** |
| 2 | Cabecera `Host: evil.com` (DNS rebinding) | **421** |
| 3 | Token incorrecto | **401** |
| 4 | Token correcto en la URL | **303**: fija la cookie `HttpOnly; SameSite=Strict` y quita el token de la URL |
| 5 | Página con cookie | **200** |
| 6 | `POST` | **405** (solo lectura) |
| 7 | Cabeceras | CSP `default-src 'none'; script-src 'self'…`, `nosniff`, `no-referrer`, `no-store` |
| 8 | `/events` (SSE) | emite `event: tasks` con la lista y reenvía solo cuando cambia |
| 9 | Interfaz de red | escucha solo en `127.0.0.1:<puerto aleatorio>` |

## Conclusión

Viable. El servidor más el cliente ocupan unas 120 líneas, sin dependencias nuevas, con el arranque intacto y la seguridad verificable. La salida del agente se inserta solo con `textContent` (SR-035).

## Impacto

- Valida el enfoque de ADR-0012 (pendiente de que lo acepte el usuario).
- Si se adopta: mover a `src/ui/` como `hilvan ui`, añadir tests automáticos de SR-032…SR-036 y, en M4/M5, acciones `POST` con la cabecera `X-Hilvan-Token` (CSRF).
- El *polling* de 1 s sobre `.hilvan/` es suficiente para un usuario. `fs.watch` se evalúa si hiciera falta.
