# 001 — Requisitos de producto

- **Estado:** Approved
- **Versión:** 1.2
- **Fecha:** 2026-10-06
- **Aprobado por / fecha:** Fernando Giraldo, 2026-10-06 (aprobación del plan que resuelve Q-001–Q-010)
- **Relacionado:** [investigación 2026-10-06](../docs/research/2026-10-06-foros-y-herramientas.md), [ADR-0002–0010](../docs/adr/README.md), [plan MVP](../docs/plans/001-mvp.md)
- **Nombre de producto:** Hilvan (D-025)

## 1. Resumen

El producto permitirá que una persona coordine agentes de distintas aplicaciones y varias cuentas por aplicación dentro de un proyecto. La persona elegirá qué aplicaciones habilitar y cuántos perfiles configurar en cada una. Podrá repartir trabajo, continuar una tarea con otro perfil cuando tenga sentido y conservar un registro compartido del objetivo, las decisiones y el estado del trabajo.

El valor central no es fusionar las cuotas de los proveedores ni prometer que una conversación privada se puede trasladar intacta. Es reducir el trabajo manual necesario para escoger al agente apropiado y darle contexto útil y verificable para continuar.

## 2. Problema

Las herramientas de IA suelen trabajar en sesiones aisladas. Cuando una persona cambia de proveedor o cuenta —por especialidad, límites de uso, disponibilidad o herramientas conectadas— debe reconstruir el contexto y volver a explicar el estado del proyecto. Esto consume tiempo y puede causar pérdida de decisiones, duplicación de trabajo o errores.

El problema también aparece al coordinar varios agentes para que investiguen, planifiquen, implementen o revisen distintas partes de un mismo objetivo.

## 3. Usuarios y casos de uso

### Usuario principal

Una persona que ya usa dos o más asistentes o agentes de IA, posiblemente con varias cuentas del mismo servicio, y quiere coordinarlos sin administrar manualmente cada traspaso.

### Casos de uso iniciales

1. Iniciar una tarea con un agente y continuarla con otro sin reconstruir todo el contexto a mano.
2. Elegir una cuenta personal o corporativa del mismo proveedor para una tarea específica.
3. Usar una cuenta con acceso a conectores corporativos para tareas que necesitan esos datos o herramientas.
4. Usar otro perfil para tareas genéricas que la persona considere apropiadas para ese perfil.
5. Pedir a varios agentes que realicen tareas independientes y reunir sus resultados para revisión humana.
6. Pedir a un perfil de otro proveedor que revise el resultado de una tarea (revisión cruzada).
7. Continuar una tarea con otro perfil cuando el primero alcanza un límite de uso, con un paquete de contexto aprobado.
8. (Posterior) Usar un perfil de investigación, por ejemplo Perplexity, que entrega hallazgos con fuentes.

## 4. Principios de producto

- **La persona mantiene el control:** el producto muestra quién realizará cada tarea y solicita autorización en los cruces sensibles de cuenta o contexto.
- **Perfiles explícitos:** cada cuenta es una identidad independiente con credenciales, capacidades, límites y conectores propios.
- **Configuración elegida por el usuario:** el producto no presupone proveedores ni cantidades fijas de cuentas. El usuario habilita las aplicaciones y perfiles que utiliza.
- **Elegibilidad antes de optimización:** primero se filtran perfiles según autorización, clasificación de datos, herramientas y capacidades requeridas. Solo entre perfiles elegibles se recomienda según suficiencia, disponibilidad, consumo/costo conocido y preferencia.
- **Continuidad basada en artefactos:** los traspasos se apoyan en resúmenes de tarea, decisiones, archivos y resultados observables, no en una supuesta transferencia transparente de historiales privados.
- **Cuotas separadas:** el producto puede mostrar o usar las cuentas que la persona haya configurado, pero no combina sus cuotas ni promete eludir límites del proveedor.
- **Trazabilidad:** debe ser posible ver qué agente recibió qué tarea y qué contexto produjo o consumió.
- **Revisión humana:** la persona conserva la decisión final sobre cambios y resultados.

## 5. Decisiones acordadas

- **D-001 — Varias cuentas del mismo proveedor:** el producto debe modelar cuentas distintas como perfiles seleccionables, incluso cuando pertenecen a la misma aplicación.
- **D-002 — Elección por tarea:** la persona puede seleccionar qué perfil ejecuta cada tarea.
- **D-003 — Uso de cuenta personal en trabajo corporativo:** puede permitirse por decisión explícita de la persona para una tarea concreta, sin importar el proveedor. El producto debe mostrar qué contexto se enviará antes de ejecutar.
- **D-004 — Perplexity:** ~~integración opcional en MVP~~ **diferida a después del MVP** (2026-10-06): su API exige créditos aparte de la suscripción. El rol de investigación lo cubre cualquier perfil con búsqueda web. Cuando se integre, será de solo lectura y con resultados citados.
- **D-005 — Nombre:** ~~`agent-orchestrator` como nombre de trabajo~~ sustituido por D-025.
- **D-006 — Implementación guiada por especificaciones:** usar SDD y no iniciar una capacidad sin requisitos aprobados y criterios de aceptación.
- **D-007 — Aplicaciones y perfiles configurables:** el usuario elige las aplicaciones que integra y puede configurar cero, uno o varios perfiles por aplicación. Ningún proveedor ni cantidad fija se considera obligatorio.
- **D-008 — Elegibilidad antes de recomendar:** un perfil solo puede recomendarse si está autorizado para la clasificación de datos y dispone de las capacidades y herramientas requeridas. La optimización no puede invalidar este filtro.
- **D-009 — Recomendación explicable:** la asignación inicial es explícita. El producto puede recomendar un perfil entre los elegibles, explicar los factores usados y esperar confirmación; no cambia de identidad silenciosamente.
- **D-010 — Restricciones obligatorias:** una política organizacional configurada como obligatoria debe excluir perfiles o transferencias prohibidos y no puede ser anulada por la aprobación del usuario. El mecanismo administrativo queda pendiente de diseño.
- **D-011 — Perfil vinculado a un entorno de ejecución:** un perfil identifica tanto la cuenta como la instancia concreta del agente que la usa (por ejemplo, Claude Code en Windows o en una distribución WSL). El orquestador debe dirigir la tarea a ese destino explícito; no basta con seleccionar proveedor o nombre de cuenta.
- **D-012 — Invocación mediante adaptadores:** el orquestador entrega instrucciones a través de un adaptador de ejecución configurado para el entorno del perfil. La vía preferida para un CLI local es iniciar un proceso con argumentos, directorio de trabajo y entrada/salida controlados. No debe depender de escribir en una ventana de terminal que la persona haya abierto manualmente.

- **D-013 — Lenguaje:** TypeScript sobre Node.js ≥ 22. El contrato adaptador↔núcleo es JSON por stdio, independiente del lenguaje, para permitir runners en otros lenguajes (p. ej. Go) más adelante. ADR-0002.
- **D-014 — Superficie y SO (Q-001, Q-002, Q-004):** CLI propio `hilvan` primero; UI web local en el incremento 2 sobre el mismo núcleo. Soporte de Linux, Windows nativo y WSL; macOS "best effort". ADR-0003.
- **D-015 — Integraciones iniciales (Q-003):** adaptadores para Claude Code y Codex CLI. ADR-0004.
- **D-016 — Modo de adaptador (Q-009):** un proceso no interactivo por ejecución (`claude -p` / `codex exec`), con eventos normalizados, cancelación y reanudación por id de sesión. Puente a sesiones interactivas y adaptador ACP genérico: posteriores. ADR-0004.
- **D-017 — Entornos (Q-010):** un solo proceso orquestador con lanzadores por destino: `native`, `wsl` (distribución concreta) y `windows` (desde WSL). Lanzador `ssh` en el incremento 2. Sin runner por entorno en el MVP. ADR-0005.
- **D-018 — Identidad:** cada perfil usa un directorio de configuración propio (`CLAUDE_CONFIG_DIR`, `CODEX_HOME`) del CLI oficial sin modificar; el usuario inicia sesión con el flujo del proveedor; el orquestador nunca lee, copia ni almacena tokens. Perfiles con API key referencian una variable de entorno. ADR-0006.
- **D-019 — Persistencia:** archivos locales (YAML/JSON/Markdown/JSONL); sin base de datos en el MVP. ADR-0007.
- **D-020 — Trabajo paralelo (Q-005):** el MVP ejecuta tareas de forma secuencial, pero cada tarea que modifica código trabaja en su propio git worktree y rama; aceptar = fusionar; limpieza automática. ADR-0008.
- **D-021 — Modelo de uso (Q-006):** producto individual y local, open source. Las políticas de equipo se aplican mediante un archivo de política gestionada de solo lectura para el usuario. ADR-0009.
- **D-022 — Clasificación (Q-007):** etiquetas configurables por proyecto con valores por defecto `public`, `personal`, `corporate`, `confidential`; cada tarea tiene una clasificación y cada perfil declara las que acepta. ADR-0009.
- **D-023 — Aprobación (Q-008):** se aprueba el paquete completo, que puede editarse antes de aprobar; se envía exactamente la versión aprobada y se registra su hash. ADR-0009.
- **D-024 — Licencia:** Apache-2.0. ADR-0010.
- **D-025 — Nombre (Q-011):** el producto se llama **Hilvan** (de *hilvanar*). Paquete npm `hilvan`; comando `hilvan` con alias `hv`; estado de proyecto en `.hilvan/`. Se descartaron "LOTA" (riesgo de marca de Tolkien y mensaje contrario al control humano) y "models orchestrator" (genérico; el producto orquesta cuentas y agentes, no modelos). Decidido por el usuario el 2026-10-06.

## 6. Alcance del MVP

### Incluido

- Crear un proyecto con objetivo, instrucciones y estado persistente.
- Configurar aplicaciones y registrar cero o más perfiles por aplicación, incluidas varias cuentas del mismo proveedor.
- Seleccionar un perfil por tarea y mostrar sus capacidades y conectores disponibles.
- Comprobar autorización, clasificación de datos y herramientas requeridas antes de considerar un perfil elegible; explicar recomendaciones solo entre los perfiles elegibles.
- Configurar para cada perfil su destino de ejecución, incluyendo sistema/entorno, ejecutable o adaptador, y directorio de trabajo autorizado.
- Enviar instrucciones, contexto aprobado y recibir estado/salida mediante el adaptador asociado al perfil.
- Crear tareas, asignarlas a perfiles y coordinar tareas independientes.
- Preparar un paquete de traspaso compacto con objetivo, decisiones, archivos relevantes, cambios, resultados y siguiente paso.
- Pedir confirmación antes de enviar contexto de una tarea corporativa a un perfil personal.
- Mostrar una vista previa del contexto que se enviará y permitir aprobar o cancelar.
- Registrar asignaciones, aprobaciones, resultados y fallos.
- Ejecutar cada tarea que modifica código en un worktree y rama propios, con hook de preparación opcional y limpieza automática.
- Pedir revisión cruzada de un resultado a otro perfil, en modo solo lectura.
- Mostrar el consumo (coste/tokens) que reporte cada CLI por ejecución y perfil, marcado como estimación.
- Crear tareas a partir de un brief con objetivo, criterios de aceptación y restricciones.
- Mantener a la persona como aprobadora de cambios antes de integrarlos al proyecto principal.

### Fuera de alcance por ahora

- Fusionar, transferir o aumentar las cuotas de distintos proveedores.
- Leer o migrar automáticamente historiales privados entre servicios sin una API o autorización explícita.
- Automatizar acciones en aplicaciones corporativas sin exponer qué cuenta y permisos se usarán.
- Dar por hecho que una suscripción web incluye acceso programático a API o CLI.
- Ejecutar cambios de código concurrentes en el mismo árbol de trabajo sin aislamiento.
- Ejecución paralela, UI web, lanzador SSH, notificaciones de escritorio, varios repositorios por proyecto, adaptador ACP y Perplexity (incremento 2 o posterior).
- Modificar los CLIs de los proveedores, ofrecer su inicio de sesión dentro del producto o manejar sus tokens.
- Elegir proveedor o cuenta de forma completamente autónoma sin informar al usuario.

## 7. Requisitos funcionales

### FR-001 — Perfiles de cuenta

El sistema debe permitir al usuario habilitar las aplicaciones que utiliza y configurar cero o más perfiles por aplicación. Cada perfil debe tener una identidad visible y distinguir, como mínimo, aplicación/proveedor, cuenta, método de autenticación y capacidades disponibles. El modelo no debe depender de una lista ni una cantidad fija de proveedores.

### FR-002 — Selección de perfil por tarea

Al crear o asignar una tarea, el usuario debe poder elegir un perfil elegible que la ejecutará. El sistema puede recomendar perfiles elegibles y debe explicar la recomendación; cualquier cambio de perfil requiere confirmación explícita.

### FR-011 — Elegibilidad de perfiles

Antes de recomendar o asignar un perfil, el sistema debe comprobar que la política aplicable permite enviarle la clasificación de datos de la tarea y que el perfil dispone de las capacidades, herramientas y acceso requeridos. Los perfiles no elegibles deben excluirse y la razón debe ser visible. Una política organizacional obligatoria no puede anularse mediante aprobación del usuario.

### FR-012 — Criterios de recomendación

Solo entre perfiles elegibles, el sistema puede ordenar o recomendar opciones considerando capacidad suficiente, disponibilidad, consumo o costo conocido, historial de resultados por tipo de tarea y preferencia del usuario. Debe indicar qué señales conocidas sustentan la recomendación y no presentar como conocidos datos que el proveedor no expone. **El margen de cuota no puede ser el factor que diferencie entre perfiles de la misma aplicación para el mismo tipo de trabajo**, porque Hilvan no se usa para rotar cuentas y esquivar límites. El resultado de cada tarea (aceptada, rechazada, bloqueada, coste, duración) se registra para mejorar las recomendaciones.

### FR-013 — Destino de ejecución del perfil

Cada perfil debe asociarse a un destino de ejecución identificable y verificable. El destino debe distinguir entornos locales diferentes aunque usen la misma aplicación, como Windows nativo y una distribución WSL específica, y debe declarar como mínimo el tipo de entorno, el adaptador/ejecutable, el directorio de trabajo y el modo de invocación. La selección del perfil debe resolver un único destino antes de enviar instrucciones.

### FR-014 — Entrega y resultado de instrucciones

El adaptador del perfil debe entregar la instrucción y el paquete de contexto aprobado al proceso asociado, y devolver al orquestador el estado de inicio/terminación, salida disponible y código de finalización. Debe permitir solicitar cancelación y comunicar si el resultado de la cancelación es incierto. Los argumentos y rutas deben transmitirse sin construir comandos concatenados de forma insegura. La ejecución interactiva o con intervención del usuario debe identificarse como tal y no tratarse como una ejecución desatendida.

### FR-003 — Paquete de continuidad

El sistema debe poder crear un resumen de traspaso basado en el estado conocido del proyecto. Debe identificar hechos observados, decisiones, supuestos, archivos relevantes y próximos pasos, y distinguir claramente la información no verificada.

### FR-004 — Aprobación de cruce de cuentas

Cuando una tarea esté marcada como corporativa y se seleccione un perfil personal, el sistema debe mostrar una vista previa del material que recibirá ese perfil y requerir autorización explícita para esa tarea antes de enviarlo.

### FR-005 — Cancelación y límites

El usuario debe poder cancelar el envío del contexto después de inspeccionar la vista previa. La cancelación no debe enviar el paquete al perfil seleccionado.

### FR-006 — Herramientas por perfil

El sistema debe asociar conectores y herramientas con los perfiles que tengan autorización para usarlos. No debe asumir que una herramienta conectada en una cuenta está disponible en las otras.

### FR-007 — Coordinación de agentes

El usuario debe poder crear tareas relacionadas, asignar tareas independientes a distintos perfiles y revisar sus resultados dentro del mismo proyecto.

### FR-008 — Investigación con fuentes (posterior al MVP, D-004)

Si se habilita Perplexity, la integración debe devolver hallazgos con URL de fuente y fecha de consulta cuando estén disponibles. La salida debe poder adjuntarse a una tarea de otro perfil como contexto revisable.

### FR-009 — Trazabilidad

El sistema debe conservar un registro visible de asignaciones, perfil utilizado, aprobaciones de cruce, referencias de contexto enviado, resultado y estado final de cada tarea.

### FR-010 — Cuotas y errores

El sistema debe comunicar errores de autenticación, indisponibilidad o límites de uso sin reintentar automáticamente con otra cuenta cuando ese cambio pueda transferir datos a un perfil distinto. Debe permitir al usuario escoger qué hacer a continuación.

### FR-015 — Revisión cruzada

El usuario debe poder pedir que un perfil distinto revise el resultado o diff de una tarea en modo solo lectura. La revisión se adjunta a la tarea y está sujeta a las mismas reglas de elegibilidad y aprobación de contexto que cualquier envío.

### FR-016 — Consumo visible

El sistema debe mostrar por ejecución y por perfil el coste o los tokens que el CLI reporte, indicando que son estimaciones del cliente y no saldos del proveedor.

### FR-017 — Estados bloqueado e incierto

Una ejecución que termina por límite de uso, autenticación o facturación pasa a `blocked` con la causa reportada. Una ejecución cuyo proceso termina sin resultado final pasa a `uncertain`. En ningún caso se reintenta ni se cambia de perfil automáticamente; el usuario decide.

### FR-018 — Aislamiento por worktree

Cada tarea que puede modificar archivos se ejecuta en un git worktree y rama propios. Debe admitir un hook de preparación configurable y eliminar el worktree al aceptar o descartar la tarea.

### FR-019 — Brief de tarea

Una tarea se define con un brief estructurado: objetivo, contexto, criterios de aceptación, restricciones y clasificación. El brief forma parte del paquete que recibe el perfil.

### FR-020 — Decisiones locales tipadas (Criterio)

El sistema debe responder a preguntas acotadas (elección, puntuación 2–10, sí/no) con un valor, una confianza y las señales que la sustentan, usando solo información local. Cada decisión se clasifica como automática, a confirmar o a escalar según umbrales configurables. Las decisiones que envían datos a otro perfil o cambian de perfil siempre requieren confirmación. El sistema aprende de los resultados y correcciones del usuario y muestra su calibración.

## 8. Requisitos no funcionales

- **NFR-001 — Secretos:** credenciales y tokens se guardan en almacenamiento seguro del sistema operativo o mecanismo equivalente; no se escriben en registros ni archivos de proyecto.
- **NFR-002 — Minimización:** cada agente recibe solo el contexto necesario para su tarea.
- **NFR-003 — Privacidad por defecto:** el envío a otro perfil requiere una acción visible; no se copian historiales completos por defecto.
- **NFR-004 — Recuperabilidad:** el estado de las tareas y sus artefactos debe sobrevivir al cierre y reinicio de la aplicación.
- **NFR-005 — Proveedores sustituibles:** integrar un proveedor nuevo no debe requerir rediseñar el modelo de tareas o proyectos.
- **NFR-006 — Evidencia:** resultados de investigación deben mantener referencias a sus fuentes; los cambios de código deben poder revisarse como diff.
- **NFR-007 — Transparencia de gasto:** toda integración programática debe mostrar si consume cuota de suscripción, créditos API o una modalidad de facturación distinta, siempre que el proveedor exponga esa información.
- **NFR-008 — Aislamiento de ejecución:** cada invocación debe usar explícitamente el entorno y contexto de autenticación configurados para el perfil; el orquestador no debe copiar credenciales entre perfiles ni cambiar la identidad activa del proveedor para enrutar una tarea.
- **NFR-009 — Rutas y entornos:** el sistema debe validar el destino y el directorio de trabajo antes de invocar el agente, y debe mostrar qué entorno recibirá la tarea.
- **NFR-010 — Cumplimiento de términos:** el producto ejecuta los CLIs oficiales sin modificar, no ofrece el inicio de sesión del proveedor dentro del producto, no recoge ni almacena credenciales o tokens de sesión, y no intermedia ni revende uso.
- **NFR-011 — Configuración explícita del agente:** las ejecuciones no deben cargar servidores MCP ni hooks no declarados para el perfil (p. ej. usar configuración MCP estricta), para evitar que un repositorio ajeno amplíe las herramientas de un perfil.
- **NFR-012 — Multiplataforma:** las pruebas automáticas se ejecutan en Linux y Windows.
- **NFR-013 — Ligereza:** Hilvan funciona en equipos modestos sin modelos locales ni servicios externos propios. Cada decisión de Criterio tarda menos de 5 ms. El arranque del CLI y la memoria tienen presupuestos medidos en las pruebas.

## 9. Criterios de aceptación del MVP

- **AC-001:** el usuario puede habilitar una aplicación y registrar dos perfiles distintos para ella; puede distinguirlos en la lista de perfiles.
- **AC-002:** una tarea puede asignarse a cualquiera de los perfiles elegibles sin reemplazar ni compartir sus credenciales.
- **AC-003:** una tarea corporativa asignada a un perfil personal no se envía hasta que el usuario revise y apruebe la vista previa.
- **AC-004:** si el usuario cancela la aprobación, el perfil personal no recibe el paquete de contexto.
- **AC-005:** el paquete de continuidad permite a un segundo perfil entender el objetivo, el estado, las decisiones y el siguiente paso sin depender del historial completo del primero.
- **AC-006:** una herramienta conectada solo al perfil corporativo no aparece como disponible en el perfil personal.
- **AC-007:** cuando una tarea falla por límite de uso, el sistema solicita una decisión y no cambia de perfil silenciosamente.
- **AC-008:** el historial de tareas indica qué perfil actuó, qué contexto se aprobó y cuál fue el resultado.
- **AC-009 (posterior al MVP, D-004):** si Perplexity está habilitado, una investigación puede adjuntarse a otra tarea conservando enlaces de fuentes.
- **AC-010:** la persona puede revisar los cambios producidos antes de aceptarlos en el proyecto principal.
- **AC-011:** el usuario puede tener cero, uno o varios perfiles por aplicación, y el flujo principal no depende de nombres ni cantidades de proveedores codificados como obligatorios.
- **AC-012:** un perfil sin autorización para la clasificación de datos o sin una herramienta requerida queda excluido de las recomendaciones, con una explicación visible.
- **AC-013:** entre perfiles elegibles, una recomendación explica los factores observables considerados y solicita confirmación antes de reasignar una tarea.
- **AC-014:** una restricción organizacional marcada como obligatoria bloquea el perfil o transferencia incluso si el usuario intenta aprobarla.
- **AC-015:** se pueden configurar dos perfiles de una misma aplicación apuntando a entornos distintos en un mismo PC (por ejemplo Windows nativo y una distribución WSL), y el usuario puede identificar el destino asociado a cada perfil.
- **AC-016:** al asignar una tarea, el orquestador inicia el adaptador/destino del perfil seleccionado, entrega la instrucción y el contexto aprobado, y registra el estado y resultado devueltos; no escribe en una terminal elegida por heurística.
- **AC-017:** una tarea asignada a un perfil no usa por accidente el ejecutable, directorio de trabajo o autenticación configurados para otro perfil.
- **AC-018:** el usuario puede pedir a un segundo perfil una revisión de solo lectura de una tarea y la revisión queda adjunta a la tarea.
- **AC-019:** tras cada ejecución, la tarea muestra el coste o tokens reportados por el CLI con la etiqueta "estimado", o "no disponible" si el CLI no lo reporta.
- **AC-020:** si se interrumpe el proceso del agente sin resultado final, la tarea queda en `uncertain` y no se reintenta automáticamente.
- **AC-021:** una tarea que modifica código trabaja en su propio worktree; al aceptarla se fusiona su rama y al aceptarla o descartarla se elimina el worktree.
- **AC-022:** con red deshabilitada, Criterio sugiere tipo de tarea, complejidad y riesgo con confianza y explicación, en menos de 5 ms por decisión. Una corrección del usuario cambia las sugerencias posteriores para entradas similares.

## 10. Riesgos y supuestos

- **R-001 — Políticas de la organización:** el usuario puede autorizar un cruce desde el producto, pero eso no anula políticas o acuerdos corporativos. Está pendiente definir cómo representar restricciones obligatorias del administrador.
- **R-002 — Diferencias entre interfaces:** una suscripción de una aplicación puede no estar disponible en su CLI, API o integración programática.
- **R-003 — Contexto incompleto:** un resumen puede omitir decisiones implícitas; debe ser revisable y no presentarse como transcripción perfecta.
- **R-004 — Presupuesto de Perplexity (confirmado 2026-10-06):** el acceso de API puede tener facturación distinta a la suscripción web; el MVP no debe asumir que Pro/Max cubre llamadas API.
- **R-005 — Conflictos de edición:** agentes concurrentes pueden modificar los mismos archivos; falta definir el mecanismo de aislamiento.
- **A-001:** los usuarios tienen autorización para iniciar sesión en cada cuenta que configuren.
- **A-002:** los proveedores ofrecen una vía permitida para invocar o usar sus agentes desde la plataforma, mediante CLI, SDK, API u otro mecanismo documentado.
- **R-006 — Términos de proveedores:** los proveedores pueden endurecer las condiciones de uso programático con suscripción. Mitigación: NFR-010, permitir perfiles con API key y revisar los términos antes de cada versión.
- **R-007 — Nombre:** "Agent Orchestrator" ya está en uso por otros productos. Mitigación: Q-011 antes de publicar.
- **R-008 — Cambios en los CLIs:** el formato de eventos de `claude -p` y `codex exec` puede cambiar. Mitigación: adaptadores con fixtures de eventos reales y detección de capacidades.

## 11. Preguntas resueltas y pendientes

| Pregunta | Resolución |
|---|---|
| Q-001 Plataforma | D-014 |
| Q-002 Sistemas operativos | D-014 |
| Q-003 Integraciones iniciales | D-015, D-004 |
| Q-004 Superficie | D-014 |
| Q-005 Trabajo paralelo | D-020 |
| Q-006 Modelo de uso | D-021 |
| Q-007 Clasificación | D-022 |
| Q-008 Aprobación | D-023 |
| Q-009 Adaptadores | D-016 |
| Q-010 Entornos | D-017 |
| Q-011 Nombre público | D-025 |

Pendientes, no bloqueantes para el MVP:

- **Q-013 — Lenguaje de implementación (2026-10-07):** el spike 003 muestra que Go cumple mucho mejor NFR-013 (arranque 1 ms frente a ~145 ms, 15 MB frente a 60–80 MB, binario sin runtime). ¿Se migra a Go antes de M2, revisando ADR-0002? Decide el usuario.
- **Q-012 — Monetización:** modelo de sostenibilidad del proyecto open source; se trabajará después del MVP.

## 12. Historial de revisión

- **1.2 (2026-10-07):** FR-020 (Criterio), NFR-013 (ligereza), AC-022; FR-012 con regla de cuotas por aplicación y registro de resultados. ADR-0011.
- **1.1 (2026-10-06):** nombre de producto Hilvan (D-025, resuelve Q-011). Sin cambios de alcance.
- **1.0 (2026-10-06):** baseline aprobado. Resuelve Q-001–Q-010 (D-013–D-024), difiere Perplexity (D-004), añade FR-015–FR-019, NFR-010–NFR-012, AC-018–AC-021, R-006–R-008. Autoriza diseño e implementación del alcance de la sección 6 según el [plan MVP](../docs/plans/001-mvp.md).
- **0.3 (2026-10-06):** destinos y adaptadores de ejecución (D-011, D-012).
- **0.2:** configuración dinámica de perfiles y elegibilidad (D-007–D-010).
- **0.1:** versión inicial.
