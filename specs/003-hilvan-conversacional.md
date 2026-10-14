# 003 — Hilvan conversacional

- **Estado:** Draft
- **Versión:** 0.1
- **Fecha:** 2026-10-14
- **Autor:** Claude Code (propuesta), para revisión de Fernando Giraldo
- **Aprobado por / fecha:** —
- **Relacionado:** [spec 001](001-product-requirements.md) (FR-004, FR-010, FR-013, AC-003, AC-007, AC-013, AC-018, NFR-003, NFR-007, NFR-014), [spec 002](002-chat-interprete.md) (D-037 a D-045), [ADR-0012](../docs/adr/0012-interfaz-tui-propia-y-web-local-minima.md) (`hilvan ui`, aplazada por D-036)

## 1. Resumen

`hilvan chat` debe ser la entrada única a todo lo que hace Hilvan. El usuario escribe lo que quiere en lenguaje natural; un intérprete elegido por el usuario lo convierte en una acción de Hilvan o en una tarea para un agente, y Hilvan muestra la acción exacta antes de hacer nada que cambie algo. Esta spec sustituye la lista cerrada de nueve operaciones de la spec 002 (D-038, D-045) por un modelo de tres niveles de control, de forma que cualquier petición pueda llegar a un agente sin que se debiliten los controles de Hilvan.

## 2. Problema

- Con la lista cerrada, una petición como «revisa este repositorio» o «investiga X» no se puede atender, aunque Hilvan ya sepa crear esa tarea. El usuario tiene que traducir su intención a comandos.
- La aplicación se reduce a un conjunto fijo de operaciones y no es la forma natural de usar Hilvan.
- Abrir todo sin un modelo de control haría que una frase mal interpretada cambiara reglas, perfiles o el repositorio sin que el usuario lo viera.

## 3. Usuarios y casos de uso

- **Pedir trabajo:** «revisa los cambios de la rama feat/x y dime qué riesgos ves»; Hilvan crea una tarea de revisión y propone el perfil.
- **Consultar el estado:** «qué tareas tengo en revisión» o «cuánto he gastado esta semana».
- **Actuar sobre una tarea:** «acepta la tarea 04SXJ1»; Hilvan muestra el diff de la revisión y confirma.
- **Cambiar la configuración:** «añade un perfil de Codex»; Hilvan indica el comando exacto y no lo ejecuta desde el chat (D-047).

## 4. Decisiones propuestas

Estas decisiones están **pendientes de aprobación**. Sustituyen a D-038 y D-045 cuando se aprueben.

- **D-046 — Alcance del chat:** `hilvan chat` puede atender cualquier petición relacionada con Hilvan o con un trabajo que un agente pueda hacer. Las peticiones que no son de Hilvan ni de un agente se responden con una explicación de lo que Hilvan puede hacer.
- **D-047 — Configuración fuera del chat:** cambiar perfiles, reglas de política, clasificaciones o instalar o borrar cualquier cosa de la configuración no se ejecuta desde el chat. El chat propone el comando exacto y el usuario lo ejecuta.
- **D-048 — Escritura desde el chat:** las peticiones que escriben en el repositorio se permiten, siempre con confirmación explícita y en una rama `hilvan/…`, como en las tareas de escritura actuales.
- **D-049 — Tres niveles de control:** consultas sin confirmación; acciones sobre tareas con confirmación del comando exacto; configuración y política solo con comandos (D-047).
- **D-050 — Todo sigue pasando por las comprobaciones existentes:** perfil elegible, clasificación, aprobación de políticas, límites de uso y recomendación de Criterio. El chat no añade ni quita comprobaciones.

## 5. Alcance

### Incluido

- Clasificación de la petición en: consulta de Hilvan, acción sobre tarea, tarea nueva para un agente, configuración (siempre propuesta, nunca ejecutada) o fuera de alcance.
- Tareas nuevas a partir de una petición libre, con el objetivo tomado de la petición y criterios propuestos por Criterio.
- Confirmación del comando exacto antes de cualquier acción que cambie algo.
- Auditoría de cada petición, su clasificación y su resultado, sin el texto en claro (como FR-026).
- Mantener `/profile`, `/interpreter`, `/help` y `/exit` (FR-027 de la spec 002).

### Fuera de alcance

- Ejecución automática sin confirmación, también para consultas que cambian algo.
- Cambios de configuración o de política desde el chat (D-047).
- Interfaz gráfica (`hilvan ui`, D-036); esta spec describe solo la terminal.
- Memoria de la conversación entre frases (NFR-016 de la spec 002 se mantiene).

## 6. Requisitos funcionales

### FR-028 — Clasificación de la petición

El sistema debe clasificar cada petición en una de estas categorías antes de actuar: consulta de Hilvan, acción sobre una tarea, tarea nueva para un agente, propuesta de configuración o fuera de alcance. La categoría y el comando propuesto se muestran al usuario.

### FR-029 — Tareas nuevas desde una petición libre

Una petición de trabajo debe convertirse en una tarea con el objetivo tomado de la petición y criterios propuestos. Modo de solo lectura por defecto; el modo de escritura requiere que el usuario lo pida y lo confirme (D-048).

### FR-030 — Confirmación del comando exacto

Antes de cualquier acción que cambie el estado de una tarea, de un perfil o del repositorio, el sistema debe mostrar el comando exacto y pedir confirmación. Sin confirmación no hay ejecución.

### FR-031 — Configuración solo como propuesta

Una petición de cambio de configuración o de política se responde con el comando exacto que el usuario debe ejecutar, sin ejecutarlo.

### FR-032 — Registro de peticiones

Cada petición debe registrarse en la auditoría con su categoría, el hash del texto, el comando propuesto y el resultado (confirmado, rechazado o no ejecutado), sin guardar el texto en claro.

## 7. Requisitos no funcionales

- **NFR-017 — Coste visible:** las peticiones que llaman a un modelo (clasificación, interpretación o tarea de agente) muestran su coste estimado, igual que NFR-007 y NFR-015.
- **NFR-018 — Control por nivel:** ninguna petición puede pasar de un nivel de control a otro sin confirmación del usuario (D-049).

## 8. Criterios de aceptación

- **AC-034:** dado «revisa los cambios de la rama feat/x», cuando el usuario lo escribe en el chat, entonces el sistema propone crear una tarea de revisión, muestra el perfil y pide confirmación antes de crearla.
- **AC-035:** dado «añade un perfil de Codex», cuando el usuario lo escribe, entonces el sistema muestra el comando exacto y no modifica la configuración.
- **AC-036:** dado una petición que escribe en el repositorio, cuando el usuario no confirma el modo de escritura, entonces la tarea se crea en solo lectura o no se crea.
- **AC-037:** dado una petición fuera de alcance, cuando el usuario la escribe, entonces el sistema explica lo que Hilvan puede hacer y no ejecuta nada.
- **AC-038:** dado una petición de consulta («tareas en revisión»), cuando el usuario la escribe, entonces el sistema responde sin pedir confirmación y no modifica ningún dato.
- **AC-039:** dado cualquier petición, cuando se consulta la auditoría, entonces aparecen categoría, comando propuesto y resultado, y no el texto en claro.

## 9. Riesgos y supuestos

- **R-012:** una petición mal clasificada lleva a una acción distinta de la que el usuario quería. Mitigación: el comando exacto se muestra y se confirma antes de actuar (FR-030).
- **R-013:** una tarea de escritura creada desde el chat cambia más de lo esperado. Mitigación: rama propia, revisión antes de aceptar (AC-010) y confirmación del modo (AC-036).
- **A-011:** el intérprete de la spec 002 puede clasificar cualquier petición en una categoría con salida estructurada. Verificado para la lista cerrada en Linux; pendiente con el esquema de categorías y en Windows.

## 10. Preguntas abiertas

- **Q-017 — Consultas que cambian algo:** ¿una consulta como «sincroniza las tareas» cuenta como consulta o como acción? Propuesta: acción, con confirmación.
- **Q-018 — Límite de tareas por petición:** ¿una petición puede crear varias tareas a la vez? Propuesta: no; una petición, una tarea, y la siguiente se pide aparte.
- **Q-019 — Clasificación de la petición:** ¿la petición se clasifica con el mismo intérprete o con reglas locales? Propuesta: el intérprete, con las reglas de clasificación de FR-025 ya en vigor.

## 11. Próxima revisión

Para pasar a `Approved`: aprobar o modificar D-046 a D-050, responder Q-017 a Q-019 y verificar A-011. Al aprobar, D-038 y D-045 pasan a obsoletos (sin renumerar) y se actualiza la spec 002. Después, plan de implementación por fases: consultas, tareas nuevas, acciones sobre tareas y, por último, la clasificación completa de FR-025.
