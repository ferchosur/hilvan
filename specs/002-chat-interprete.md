# 002 — Chat conversacional con perfil intérprete

- **Estado:** Approved
- **Versión:** 1.0
- **Fecha:** 2026-10-14
- **Autor:** Claude Code (propuesta)
- **Aprobado por / fecha:** Fernando Giraldo, 2026-10-14 (D-037 a D-045)
- **Relacionado:** [spec 001](001-product-requirements.md) (D-035 diferida, FR-004, FR-010, FR-013, AC-003, AC-018, NFR-003, NFR-007, NFR-014), [nota AgentBridge](../docs/research/2026-10-12-agentbridge.md), [propuestas pendientes](../docs/research/2026-10-08-propuestas-pendientes.md)

## 1. Resumen

Hoy crear y lanzar una tarea exige varios comandos con muchas opciones. Esta spec propone un modo conversacional, `hilvan chat`: el usuario escribe lo que quiere en lenguaje natural, un perfil marcado como **intérprete** propone un comando de una lista cerrada, Hilvan lo muestra completo y lo ejecuta solo si el usuario confirma. El intérprete no ejecuta nada por sí mismo.

## 2. Problema

- Crear una tarea de lectura sencilla requiere escribir `task new` con `--objective`, `--criteria` y `--mode`, además de `task run` con `--profile` y `--approve`. Para un usuario nuevo, eso frena la adopción.
- La información que el usuario necesita para decidir (qué se ejecuta, con qué cuenta, cuánto cuesta) ya existe en Hilvan, pero está repartida en varios comandos.
- Un asistente que actúa sin mostrar el comando rompe la transparencia que el producto exige (FR-004, AC-003).

## 3. Usuarios y casos de uso

- **Usuario que empieza:** escribe "lee README.md y dime su primera línea"; ve el comando propuesto y lo confirma.
- **Usuario habitual:** escribe "revisa lo que hizo la tarea de ayer con Codex"; Hilvan propone `review` con el perfil correspondiente.
- **Usuario con datos corporativos:** escribe una petición sobre un proyecto corporativo; Hilvan no envía el texto a un intérprete que no acepte esa clasificación.

## 4. Decisiones propuestas

Estas decisiones están **pendientes de aprobación**. Hasta que se aprueben, ninguna se implementa.

- **D-037 — Intérprete por perfil:** un perfil puede marcarse como intérprete (`interpreter: true`). Por defecto, el perfil con `routing: default` de mayor preferencia. Solo puede haber un intérprete activo por aplicación.
- **D-038 — Lista cerrada de operaciones:** el intérprete solo puede proponer operaciones de la lista de la sección 6 (FR-022). Cualquier otra respuesta se descarta y el usuario recibe un aviso.
- **D-039 — Confirmación obligatoria:** ningún comando se ejecuta sin confirmación explícita del usuario en la sesión. No hay modo de ejecución automática en esta versión.
- **D-040 — Llamadas aisladas del intérprete:** cada interpretación es una llamada sin sesión reanudada, sin herramientas y sin escrituras. No reutiliza la sesión de ninguna tarea.
- **D-042 — Comandos con barra:** `hilvan chat` admite comandos locales que no llaman a ningún modelo: `/profile <id>` elige el perfil que ejecuta las tareas, `/interpreter <id>` elige el perfil que interpreta las frases, `/help` y `/exit`. Los dos perfiles se eligen por separado. Acordado por el usuario el 2026-10-14.
- **D-044 — Idioma de las frases:** el intérprete acepta frases en español e inglés desde el principio. Acordado por el usuario el 2026-10-14 (Q-015).
- **D-045 — Alcance del primer entregable:** prototipo con un solo perfil intérprete y lista cerrada de operaciones; la clasificación completa y la auditoría detallada de cada interpretación quedan para la versión siguiente. La auditoría mínima (FR-026) sí se incluye. Acordado por el usuario el 2026-10-14 (Q-016).
- **D-043 — Persistencia:** la elección de `/profile` y `/interpreter` se guarda en la configuración de Hilvan y se usa en la siguiente sesión. Antes de cada ejecución, el sistema muestra qué perfil está activo. Acordado por el usuario el 2026-10-14.

## 5. Alcance

### Incluido

- Comando `hilvan chat`: bucle de lectura de frases, propuesta, confirmación y ejecución.
- Configuración de un perfil intérprete.
- Validación de la respuesta del intérprete contra la lista cerrada.
- Vista previa de clasificación, perfil de destino, coste estimado y comando exacto antes de confirmar.
- Registro de cada interpretación y de cada confirmación en la auditoría.

### Fuera de alcance

- Ejecución automática sin confirmación (D-039).
- Intérpretes de varias aplicaciones a la vez.
- Que el intérprete modifique archivos, configuración o políticas.
- Conversación de varias vueltas con memoria entre frases. Cada frase se interpreta sola (D-040).
- Interfaz gráfica (ADR-0012, `hilvan ui`).

## 6. Requisitos funcionales

### FR-027 — Comandos con barra

El sistema debe procesar en `hilvan chat` los comandos `/profile`, `/interpreter`, `/help` y `/exit` sin enviar el texto a ningún modelo. `/profile` y `/interpreter` sin argumento listan los perfiles elegibles y marcan el activo. Un identificador desconocido se rechaza con el listado de perfiles disponibles.

### FR-022 — Lista cerrada de operaciones

El sistema debe permitir al intérprete proponer solo estas operaciones: `task new`, `task recommend`, `task run`, `task show`, `task log`, `task list`, `review`, `accept`, `stats`. Cualquier otra respuesta se descarta sin ejecutarla.

### FR-023 — Perfil intérprete

El sistema debe permitir marcar un perfil como intérprete y verificar que su aplicación y su clasificación aceptada son compatibles con el mensaje antes de enviarlo.

### FR-024 — Vista previa y confirmación

Antes de ejecutar cualquier operación, el sistema debe mostrar el comando exacto, el perfil que ejecutará, la clasificación, el coste estimado si el proveedor lo informa, y pedir confirmación. Sin confirmación, no hay ejecución.

### FR-025 — Clasificación del mensaje

El sistema debe clasificar el mensaje del usuario antes de enviarlo al intérprete. Si la clasificación no es aceptada por el perfil intérprete, debe pedir al usuario que elija otro perfil o que cancele, y no debe enviar el texto.

### FR-026 — Registro de interpretaciones

El sistema debe registrar en la auditoría cada interpretación (perfil, hash del mensaje, operación propuesta, válida o descartada) y cada confirmación o rechazo, sin guardar el texto del mensaje en claro.

## 7. Requisitos no funcionales

- **NFR-015 — Coste visible:** cada interpretación muestra su coste estimado en la sesión, igual que una tarea (NFR-007). El usuario puede desactivar el intérprete en cualquier momento.
- **NFR-016 — Sin contexto acumulado:** el intérprete no recibe el historial de la conversación ni contenido de tareas anteriores; solo la frase actual y la lista de operaciones.

## 8. Criterios de aceptación

- **AC-031:** dado `/profile <id>`, cuando el usuario ejecuta una tarea después, entonces se usa ese perfil y el sistema lo muestra antes de confirmar; tras reiniciar `hilvan chat`, el mismo perfil sigue activo.
- **AC-032:** dado `/interpreter <id>`, cuando el usuario escribe una frase, entonces la interpreta ese perfil, distinto del perfil que ejecuta si así se eligió.
- **AC-033:** dado un comando con barra, cuando se procesa, entonces no se envía ningún texto a un modelo ni se consume cuota.

- **AC-026:** dado un perfil intérprete configurado, cuando el usuario escribe una frase, entonces el sistema muestra el comando propuesto y no ejecuta nada hasta que el usuario confirma.
- **AC-027:** dado que el intérprete responde con una operación fuera de la lista cerrada, cuando Hilvan la recibe, entonces la descarta, no la ejecuta y lo indica al usuario.
- **AC-028:** dado un mensaje clasificado como corporativo y un intérprete que solo acepta personal, cuando el usuario escribe la frase, entonces el texto no se envía al intérprete.
- **AC-029:** dado que el usuario rechaza un comando propuesto, cuando termina la sesión, entonces no se ha creado ni ejecutado ninguna tarea y la auditoría registra el rechazo.
- **AC-030:** dado una interpretación, cuando se consulta la auditoría, entonces aparecen perfil, hash del mensaje, operación y resultado, y no el texto en claro.

## 9. Riesgos y supuestos

- **R-010:** el intérprete propone una operación válida pero no la que el usuario quería. Mitigación: se muestra el comando exacto y se exige confirmación.
- **R-011:** cada interpretación consume cuota del perfil intérprete. Mitigación: coste visible (NFR-015) y posibilidad de desactivarlo.
- **A-010:** el proveedor de la cuenta intérprete mantiene una interfaz de salida estructurada que Hilvan puede validar. **Verificado en Linux el 2026-10-14** con llamadas reales: Claude Code 2.1.299 (`claude -p --output-format json --json-schema …`) y Codex CLI 0.161.0 (`codex exec --output-schema …`) devolvieron un objeto que cumple el esquema. Pendiente de verificar en Windows.

## 10. Preguntas abiertas

- **Q-014 — Intérprete por defecto:** resuelta parcialmente por D-042 y D-043: el intérprete se elige con `/interpreter` y se guarda; el valor inicial es el perfil `routing: default` de mayor preferencia.
- **Q-015 — Idioma de las frases:** resuelta por D-044 (español e inglés).
- **Q-016 — Prototipo o versión completa:** resuelta por D-045 (prototipo primero).

## 11. Próxima revisión

Estado: aprobada el 2026-10-14. A-010 verificado en Linux; falta verificarlo en Windows antes de publicar la función. Próximo paso: plan de implementación del prototipo y ADR de la salida estructurada por proveedor. Después, un ADR para la elección de salida estructurada por proveedor y el plan de implementación.
