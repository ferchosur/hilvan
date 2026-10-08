# Investigación: claude-router y la elección de modelo por tarea

- **Fecha:** 2026-10-07
- **Propósito:** decidir si [`alexei-led/claude-router`](https://github.com/alexei-led/claude-router) aporta algo a Hilvan, en particular a una futura elección de modelo según la complejidad de la tarea. Hoy esa elección no está en la spec y requeriría un FR nuevo.
- **Fuente:** repositorio clonado en `../claude-router` (commit `7599fb5`, versión 1.3.0, licencia MIT), leído sin ejecutar su código: `README.md`, `docs/architecture.md`, `docs/evaluation.md` y `lib/policy.mjs`.
- **Criterio aplicado:** tomamos ideas, no código (ver `AGENTS.md`, *Código de terceros*).

## 1. Qué es

Un **mod de Claude Code** que, en cada turno de la conversación principal, cambia el modelo y el esfuerzo de la siguiente petición. La configuración por defecto tiene cuatro niveles:

| Nivel | Modelo | Esfuerzo |
|---|---|---|
| `micro` | Haiku 4.5 | — |
| `low` | Sonnet 5.5 | el de la sesión |
| `medium` | Opus 5.5 | `medium` |
| `high` | Opus 5.5 | `xhigh` |

- La decisión la aconseja un **clasificador externo**: Jev (typesafe.ai) o Clef / Clef Flash (Cloudflare Workers AI). Una política local aplica después sus reglas.
- No hace de proxy ni abre puertos: solo reescribe `model` y `effort` en el evento `turn.step`. Claude Code sigue encargándose de la autenticación, las peticiones y el consumo.
- No toca los subagentes, porque la API de mods no indica si su modelo fue explícito o heredado.

## 2. Qué dice su propia evaluación

`docs/evaluation.md` declara que **la versión actual no tiene ningún ahorro medido**:

- Su versión anterior, que funcionaba como *gateway*, costaba un 15,5 % menos que "siempre Opus `xhigh`" y un 4,1 % menos que "el modelo más fuerte necesario, fijo por sesión". El dato sale de 26 sesiones de un solo desarrollador en un día, a precios de lista y sin revisar la calidad de las respuestas.
- Al reproducir la política actual sobre una traza de equipo, con 11.619 decisiones de las que solo 37 tenían consejo del clasificador, hubo **0 decisiones distintas** de las 37 respecto a la versión anterior.

Es un contrapunto útil a afirmaciones como "reduje mi factura un 50 %" (post de LinkedIn revisado el mismo día): **el modelo fijo por sesión bien elegido ya captura casi todo el ahorro**.

## 3. Por qué no lo integramos

1. **Envía tus prompts a un tercero**: el mensaje actual y hasta seis mensajes recientes, de hasta 1.200 caracteres cada uno. Incumple NFR-013 (sin servicios externos) y la clasificación de datos: con una cuenta corporativa sería una fuga.
2. **Solo funciona con Claude y decide turno a turno.** Hilvan decide por tarea y entre proveedores (Claude y Codex).
3. **Depende de la API de mods**, que cambia entre versiones, y del proceso interactivo de Claude Code.

## 4. Ideas que adoptamos como insumo de diseño

| # | Idea en claude-router | Traducción a Hilvan |
|---|---|---|
| 1 | **El clasificador aconseja; una política local decide.** Reglas de inercia, votos consecutivos para cambiar, y quedarse con el modelo actual ante `uncertain` o si no hay consejo. | Criterio ya clasifica en local. Le falta una capa de política separada que convierta la complejidad sugerida en un modelo, con umbrales explícitos y explicación (`reason`), igual que `decide()` devuelve `{ tier, reason }`. |
| 2 | **Subir un nivel tras un fallo**, solo una vez por firma de fallo, y mantenerlo N turnos. | Si una ejecución termina en `uncertain`, `failed` o con `changes_requested` en la revisión cruzada, **sugerir** repetirla con el nivel siguiente. Siempre con confirmación, sin reintentos silenciosos (FR de estados `blocked`/`uncertain`). |
| 3 | **Cambiar de modelo tiene coste**: el modelo nuevo vuelve a escribir la caché del prompt. Un cambio a un modelo más barato puede salir más caro en ese turno. | Fijar el modelo **por tarea**. Cambiarlo solo al crear una tarea o en un traspaso, nunca en una ejecución que reanuda una sesión (`--resume`). |
| 4 | **Comprobar que el contexto cabe**: no elegir un modelo cuya ventana no aguante la siguiente petición, con un 20 % de margen. Si no se sabe, no cambiar (`context-unknown`). | Validar el tamaño del paquete de traspaso y del brief contra la ventana del modelo destino antes de proponerlo. |
| 5 | **Plazos y cortacircuitos** para el clasificador: tiempo máximo y pausa tras tres fallos. | Poco relevante: Criterio es local y tarda microsegundos. Sí aplica a cualquier futura consulta externa (p. ej. `claude mcp list`). |
| 6 | **Evaluación honesta**: reproducir las decisiones sobre registros, sin guardar prompts ni rutas, y mostrar costes como escenarios estimados, no como ahorro. | Una herramienta de calibración de Criterio que reproduzca las decisiones sobre `.hilvan/events.jsonl`, que ya no guarda texto. `hilvan stats` debe seguir mostrando estimaciones, nunca "ahorro". |

## 5. Siguiente paso

Si se aborda la elección de modelo por complejidad, redactar un FR en la spec (o en la spec 002 si se adopta el modo conversacional) que recoja las ideas 1 a 4, con el modelo fijado por tarea como regla base. No hace falta spike: el adaptador ya admite `--model` por perfil y por petición (`src/adapters/claude-code.ts`, `src/adapters/codex.ts`).
