# Propuestas pendientes de decisión (sesión del 2026-10-07/08)

- **Fecha:** 2026-10-08
- **Propósito:** conservar las propuestas discutidas en la sesión de la migración a Go, para decidirlas más adelante sin depender de la conversación. **Ninguna está aprobada:** cada una necesita la decisión del usuario y, si se adopta, un cambio de spec o un ADR (AGENTS.md, reglas 1–3).
- **Fuentes externas citadas:**
  - [Mods de Claude Code](https://code.claude.com/docs/en/plugins/mods/overview) (consultado el 2026-10-07);
  - [prompt caching de Anthropic](https://platform.claude.com/docs/en/build-with-claude/prompt-caching) (consultado el 2026-10-07);
  - [claude-router](2026-10-07-claude-router.md).

## 1. Hilvan conversacional (posible spec 002)

**Petición del usuario:** abrir `hilvan` en la terminal y pedirle una tarea o un proyecto en lenguaje natural, como con Claude o Codex. Hilvan haría por debajo todo el flujo: elegir perfil y herramienta, gestionar el contexto y la memoria.

**Choque con la spec actual:** la spec 001 define un CLI de comandos, y ADR-0003 dice "interfaz propia, no embebida en otro agente". Adoptarlo exige una spec nueva (o la 001 v2.0) y un ADR que sustituya en parte a ADR-0003.

**El problema central:** entender lenguaje natural requiere un LLM, y NFR-013 descarta modelos locales. El "cerebro" de la conversación tendría que ser una de las cuentas del usuario (`claude -p` o `codex exec`). De ahí salen tres consecuencias:

- **Fuga entre cuentas:** si el cerebro corre en la cuenta personal y el usuario describe una tarea corporativa, el contenido corporativo llega a la cuenta personal antes de que Hilvan decida nada. **Mitigación propuesta:** Criterio, que es local y no usa la red, clasifica cada mensaje *antes* de enviarlo, y si es corporativo la conversación pasa a la cuenta corporativa.
- **La conversación consume cuota** de la cuenta que hace de cerebro.
- **El cerebro solo propone.** Las aprobaciones (enviar contexto, cambiar de cuenta, aceptar un diff) las muestra Hilvan y las confirma la persona.

**Arquitectura propuesta, por etapas:**

1. **`hilvan mcp`:** las operaciones de Hilvan como herramientas MCP (crear tarea, recomendar, ejecutar, traspasar, revisar). La política se aplica en esa capa, sea quien sea el que llame.
2. **REPL `hilvan`:** el cerebro se elige por clasificación, con esas herramientas y con la memoria del proyecto como contexto (lo que guarda `.hilvan/` más un resumen del proyecto). Un "inicia un proyecto" se convierte en un plan de tareas, cada una con su enrutamiento propuesto y aprobada por la persona.
3. **De regalo:** el mismo servidor MCP permite hablar con Hilvan desde dentro de Claude Code o de Codex.

Los comandos actuales se quedan como base determinista para scripts y CI. El REPL sería en la práctica la TUI de ADR-0012, así que conviene decidir ambas cosas juntas.

## 2. Requisito "Hilvan no aumenta el consumo"

**Contexto:** una crítica a los routers de modelos ("invalidan la caché y gastan más tokens") es técnicamente cierta. La caché dura 5 minutos por defecto, y **cambiar de modelo o de esfuerzo la invalida**. Escribir en caché cuesta 1,25 veces el precio de entrada y leer de ella entre 0,05 y 0,1 veces (docs de Anthropic, 2026-10-07). Hilvan no quiere venderse como herramienta de ahorro, pero tampoco puede aumentar el consumo.

**Situación actual:** Hilvan cambia de modelo solo entre tareas, donde la sesión empieza en frío de todos modos. Planificar con un modelo y ejecutar con otro, con un traspaso compacto entre ambos, es lo que sugería la propia crítica.

**Riesgos detectados:**

1. Dividir el trabajo en demasiadas tareas pequeñas: cada una paga el arranque en frío.
2. **Reanudar con otro modelo:** `RunTask` reanuda la sesión del perfil con el `--model` *actual* del perfil. Si el modelo cambió entre ejecuciones, la caché se invalida. Hoy no está protegido.
3. La revisión cruzada es una ejecución extra (la pide el usuario y se ve en `stats`).
4. **No se puede medir:** el parser de Claude suma la lectura de caché y la escritura en caché en un solo total de entrada.

**NFR propuesto:**

- no cambiar nunca de modelo ni de esfuerzo al reanudar una sesión, con un test;
- guardar por separado la lectura y la escritura de caché y mostrar en `stats` qué parte de la entrada salió de la caché;
- un benchmark de la misma tarea con `claude -p` directo y con `hilvan task run`, para publicar cuántos tokens añade Hilvan;
- que Criterio no divida en exceso las tareas.

Relacionado: la elección de modelo por complejidad (ideas de claude-router) iría en un FR aparte, con el modelo fijo por tarea como regla base.

## 3. Extensiones de Hilvan (idea tomada de los mods)

Engancharse a los eventos *de Hilvan* (`task.new`, `route.decide`, `brief.send`, `run.event`, `run.result`, `handoff.build`, `review.verdict`, `accept`) para observar, modificar o bloquear. Funcionaría igual con Claude y con Codex, cosa que un mod de Claude no permite. Algunos ejemplos:

- reglas que la política declarativa no puede expresar ("si el diff toca `migrations/`, exige revisión cruzada");
- integraciones (abrir un ticket al aceptar, avisar al terminar);
- paneles propios en `hilvan watch`.

**Diseño compatible con Go:** extensiones como **procesos externos** que reciben el evento en JSON por stdin y responden por stdout, con límites y registro en la auditoría. Riesgo: es código con los permisos del usuario, el mismo riesgo que analiza el spike 005.

## 4. Mejora: comandos de solo lectura en tareas `read`

E2E del 2026-10-08: en una tarea de solo lectura, Claude corre con `--permission-mode dontAsk` y no puede usar `ls` ni `git ls-files`. Lo declara y Criterio rebaja el cumplimiento al 66 %. **Propuesta:** permitir comandos de lectura concretos con `--allowedTools` (por ejemplo `Bash(git log:*)`, `Bash(git ls-files:*)`, `Bash(ls:*)`). Requiere un SR nuevo y tests de que siguen bloqueadas las escrituras.

## 5. Borrador de post de LinkedIn (para la release 0.1.0)

Condiciones: publicarlo cuando haya un binario instalable con enlace, y **sin cifras no medidas**. Cuando exista el benchmark de consumo, se añade una línea con el dato.

> Le pedí a Codex que revisara un archivo. Me describió un README que no existe.
> Lo detectó Claude, que revisaba su trabajo.
>
> Por eso construí **Hilvan**, un CLI open source para trabajar con varios agentes y cuentas de IA sin perder el hilo:
>
> 1️⃣ **Cada cuenta es una identidad aislada.** Mi Claude corporativo solo recibe las tareas que necesitan sus herramientas internas. Lo demás va a mi cuenta personal, y nada cambia de cuenta sin mi permiso.
> 2️⃣ **Traspasos revisables.** Si un agente se queda sin cuota a mitad de una tarea, Hilvan prepara un paquete de continuidad (estado, commits, archivos, afirmaciones *no verificadas*), elimina los secretos y me lo enseña antes de enviarlo.
> 3️⃣ **Revisión cruzada.** Un agente de otro proveedor revisa el diff en modo solo lectura antes de que yo lo acepte.
> 4️⃣ **Criterio**, un motor de decisión 100 % local que sugiere qué cuenta y qué agente usar y aprende de mis correcciones. No envía tus prompts a ningún tercero.
>
> Lo que **no** es: una forma de sumar o saltarse cuotas. Es continuidad y control.
>
> Apache-2.0 · un único binario para Linux, Windows y WSL · [enlace]
