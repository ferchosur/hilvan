# Estado del proyecto y oportunidades

- **Fecha:** 2026-10-04
- **Estado:** Informe de descubrimiento; no aprobado como especificación de implementación
- **Nombre de trabajo:** Orquestador de agentes de IA (`agent-orchestrator`)
- **Especificación vigente:** [001 — Requisitos de producto](../specs/001-product-requirements.md), borrador v0.3

## 1. Resumen ejecutivo

El proyecto está en etapa de definición. Existe una especificación inicial de producto, pero no hay todavía arquitectura técnica aprobada, plan de implementación ni código del producto. El trabajo se guía por SDD: requisitos, diseño, plan, implementación y verificación. El estado `Draft` de la especificación significa que sus criterios aún deben revisarse antes de iniciar el desarrollo.

La idea central es una aplicación configurable para coordinar agentes y cuentas de IA en un proyecto. Cada usuario decide qué aplicaciones integra y cuántas cuentas configura en cada una. El sistema puede repartir y enrutar tareas según capacidad, disponibilidad y herramientas, y facilitar continuidad entre sesiones mediante artefactos revisables.

La oportunidad más clara frente a las herramientas revisadas es combinar tres elementos:

1. **Identidad por cuenta:** las cuentas del mismo proveedor son perfiles diferentes, con credenciales, conectores y permisos independientes.
2. **Enrutamiento sujeto a elegibilidad:** primero se determina qué perfiles pueden recibir los datos y realizar la tarea; entre esos perfiles se recomienda el que mejor equilibre capacidad, disponibilidad, consumo y costo.
3. **Transferencia de contexto bajo control:** antes de pasar información entre perfiles, mostrar qué se enviará y permitir aprobar, editar o cancelar.

Esta combinación es una hipótesis de diferenciación; todavía falta validar demanda, viabilidad de los mecanismos de autenticación y límites de cada proveedor.

## 2. Estado actual

### Artefactos existentes

- `README.md`: describe el método SDD y aclara que un borrador no autoriza implementar.
- `specs/001-product-requirements.md`: especificación funcional v0.1 con problema, usuario, principios, decisiones, alcance candidato, requisitos, criterios de aceptación, riesgos y preguntas pendientes.
- Este informe: síntesis de lo conversado y del análisis exploratorio de Orcha, `codex-plugin-cc` y la arquitectura Advisor/JEV compartida en una imagen.

### Aún no existe

- Requisitos aprobados como baseline.
- Diseño de componentes, modelo de datos y límites de confianza.
- Decisión de plataforma y experiencia principal.
- Plan de entregas priorizado.
- Prototipo que demuestre autenticación aislada para varias cuentas.
- Código del producto o integración instalada en este equipo.

No se ejecutaron pruebas ni se conectaron cuentas de proveedores durante este trabajo.

## 3. Qué tenemos definido

Las decisiones y requisitos que ya aparecen en la especificación son:

- El producto coordina agentes de distintos proveedores y puede registrar varias cuentas del mismo proveedor.
- Cada cuenta se modela como un perfil seleccionable por tarea, no como una cuota compartida.
- Perfiles distintos conservan sus credenciales, capacidades y conectores propios.
- Si se quiere pasar contexto de una tarea corporativa a un perfil personal, el usuario debe poder inspeccionar el paquete y autorizar ese envío para la tarea concreta.
- La autorización del usuario dentro del producto no reemplaza políticas corporativas que puedan prohibir la transferencia.
- La continuidad se apoya en resúmenes, decisiones, archivos y resultados observables. No se presupone que se puedan migrar historiales privados completos entre servicios.
- Un límite, error o cambio de perfil no debe producir una transferencia silenciosa a otra identidad.
- Perplexity es candidato opcional para investigación con fuentes; no es un requisito de ejecución de código.
- La persona mantiene revisión final de las entregas y cambios.
- Se usará SDD y no se implementará una capacidad sin requisitos y criterios de aceptación acordados.

### Configuración de aplicaciones y cuentas

La conversación posterior a la versión 0.1 precisa que la configuración debe ser elección de cada usuario:

- El usuario escoge qué aplicaciones/proveedores habilitar.
- Puede configurar cero, una o varias cuentas por aplicación.
- Los ejemplos Claude corporativo, Claude personal y Codex personal ilustran una configuración posible, no un catálogo obligatorio.
- El producto debe poder extender el catálogo de integraciones sin rehacer el modelo común de proyectos, tareas y perfiles.

Esta precisión se formalizó como D-007 y los requisitos FR-001/AC-011 en la versión 0.2 de `001-product-requirements.md`.

### Entorno e invocación por perfil

Una misma aplicación puede tener varias cuentas autenticadas en entornos distintos del mismo equipo, por ejemplo Claude Code nativo en Windows y Claude Code dentro de una distribución WSL. Por eso, el perfil debe apuntar a un destino de ejecución explícito: entorno/distribución, adaptador o ejecutable, directorio de trabajo y modo de invocación. El orquestador debe entregar la instrucción y el contexto aprobado a ese destino y recoger estado, salida, error y cancelación; no basta con abrir una terminal o elegir una cuenta por nombre.

La especificación v0.3 recoge esta necesidad. Quedan por decidir el contrato de adaptador inicial, si se invoca el CLI de forma no interactiva o se conecta a una sesión interactiva existente, y si un proceso del orquestador en Windows puede controlar cada entorno o se requiere un runner local.

### Enrutamiento por nivel de decisión

La imagen compartida propone una jerarquía que conviene considerar:

- agente principal que planifica y mantiene el objetivo;
- especialistas para exploración, investigación, implementación y pruebas;
- asesor invocado en decisiones de alto impacto, planes o errores persistentes;
- capa ligera de decisión para opciones acotadas, con escalamiento cuando la incertidumbre es alta.

Para nuestro producto, esa lógica debe estar subordinada a la elegibilidad de los perfiles: un modelo barato no puede ser seleccionado si no tiene autorización o herramientas necesarias. Los números de costo, forks y confianza de la imagen son ilustrativos y no se consideran benchmarks validados.

## 4. Solapamiento y reutilización conceptual

| Referencia | Qué ya resuelve | Qué podemos reutilizar como patrón | Qué no cubre por sí sola |
|---|---|---|---|
| [Orcha](https://github.com/open-orcha/orcha) | Espacios de trabajo, tareas y dependencias, asignación de agentes, mensajería, continuidad, supervisión humana, dashboard y métricas. Soporta entornos Claude y Codex con diferencias entre los runtimes. | Estado persistente de tareas; tareas como unidad de coordinación; asignación explícita; solicitudes entre agentes; memoria de continuidad; estados de revisión; control humano de finalización; métricas por ejecución. | La experiencia central no es un registro configurable de varias cuentas por proveedor con permisos/conectores por perfil y aprobación de transferencia de datos entre identidades. El repo mantiene una lista de brechas conocidas; una reutilización de código exigiría revisar por separado la licencia, seguridad y alcance técnico. |
| [OpenAI `codex-plugin-cc`](https://github.com/openai/codex-plugin-cc) | Desde Claude Code, delega a Codex, hace revisiones normales/adversariales, inicia trabajos de fondo, muestra estado/resultados, cancela tareas e importa una sesión Claude para continuarla en Codex. | Comandos de delegación fáciles de descubrir; ejecución de fondo; consultar estado y cancelar; revisión adversarial; retorno explícito de resultados; continuidad entre herramientas. | Es un puente de Claude Code hacia la autenticación/configuración local de Codex. No administra varias cuentas ni enruta entre perfiles. La transferencia de sesión no ofrece por sí sola nuestro flujo de minimización, vista previa y aprobación de contexto corporativo-personal. |
| Patrón Advisor/JEV de la imagen | Especialización por rol; escalamiento para revisión; decisión ligera para opciones pequeñas; una barrera de revisión antes de finalizar. | Enrutamiento escalonado; política de cuándo consultar un asesor; revisión separada de ejecución; registro de razón de escalamiento; permitir un modo sin asesor para tareas rutinarias. | No resuelve identidades, perfiles, autenticación, herramientas por cuenta, privacidad entre cuentas o contexto portable. Las cifras de la imagen no son evidencia independiente de calidad/costo. |

### Reutilizar capacidad no implica copiar implementación

Orcha y `codex-plugin-cc` pueden informar el diseño de flujos y operaciones. Antes de reutilizar código habría que evaluar su licencia y avisos, dependencias, acoplamiento con el runtime, modelo de seguridad, compatibilidad con SDD y costo de mantenimiento. El punto de partida recomendado es reutilizar patrones y protocolos soportados por proveedores; adoptar código requiere una decisión técnica posterior.

## 5. Diferenciación propuesta

La diferenciación no debería ser “tenemos múltiples agentes” o “un agente puede llamar a otro”; ambas ideas aparecen en las referencias analizadas. La propuesta de valor a validar es:

### 5.1 Orquestación configurable por usuario

Cada usuario habilita las aplicaciones que realmente usa y agrega la cantidad de cuentas que quiera en cada una. La interfaz no debe asumir Claude, Codex, Perplexity ni una combinación fija.

### 5.2 El perfil representa identidad, no solo modelo

Un perfil incluye proveedor/aplicación, identidad visible, mecanismo de autenticación, runtime o interfaz utilizada, conectores habilitados, capacidades, límites conocidos y clasificación personal/corporativa. Dos perfiles del mismo proveedor no comparten permisos por defecto.

### 5.3 Política antes que optimización

Orden propuesto de decisión:

1. Requisitos funcionales de la tarea: herramientas, runtime, acceso al repo o investigación.
2. Límites de privacidad y política organizacional.
3. Perfiles realmente elegibles.
4. Recomendación entre los elegibles, considerando capacidad suficiente, disponibilidad, costo/cuota expuesta y preferencia del usuario.
5. Aprobación humana cuando la asignación o transferencia cruce una frontera sensible.

No se combinan cuotas ni se promete evitar límites del proveedor. El sistema recomienda una opción dentro de las cuentas configuradas y de sus términos de uso.

### 5.4 Traspaso de contexto con minimización verificable

Crear paquetes compactos con propósito, estado, decisiones, hechos, supuestos, archivos/diffs, resultados y siguiente paso. Mostrar el contenido concreto antes de enviarlo a otro perfil; soportar edición y cancelación; guardar constancia de lo aprobado.

### 5.5 Enrutamiento explicable y escalamiento

La recomendación debe explicar por qué se propone un perfil (p. ej. “tiene acceso al MCP X”, “no requiere escritura”, “perfil más económico disponible”). Un router liviano puede decidir operaciones acotadas; ambigüedad, errores repetidos, cambios destructivos o consecuencias altas escalan a un asesor o a la persona.

### 5.6 Investigación como rol, no proveedor obligatorio

Perplexity podría ser un perfil de investigación con salida enlazada y revisable. La abstracción útil es la capacidad de investigación con fuentes; el usuario debe poder usar otro proveedor para ese rol si lo prefiere.

## 6. Decisiones pendientes

Las preguntas Q-001–Q-008 de la especificación siguen abiertas. Además, los nuevos hallazgos sugieren desglosarlas de esta forma:

### Producto y experiencia

- ¿Aplicación local de escritorio, web local, terminal o combinación?
- ¿Cuál es la primera plataforma y sistemas operativos soportados?
- ¿El usuario crea tareas en la aplicación o trabaja principalmente desde el chat/CLI de su agente habitual?
- ¿La configuración de integraciones pertenece a un usuario/dispositivo, a un proyecto o a ambos?

### Alcance del primer incremento

- ¿Primer flujo vertical: traspaso manual entre dos cuentas Claude, o delegación Claude→Codex?
- ¿Qué integración se usa para la primera prueba real y cuál es solo una opción futura?
- ¿Perplexity entra después del primer flujo de continuidad?
- ¿MVP secuencial con entrega revisable o también ejecución paralela aislada en worktrees?

### Cuenta, autenticación y compatibilidad

- ¿Qué mecanismos soportados permiten tener sesiones independientes por cuenta Claude y Codex sin pisar credenciales, incluso cuando corren en Windows nativo y WSL?
- ¿El perfil debe lanzar un proceso CLI, conectarse a un agente/runner persistente o ambas cosas? ¿Cómo se controlan entrada, salida, streaming y cancelación?
- ¿Se integra invocando CLIs instalados, SDK/API, app servers o conectores MCP? Cada mecanismo puede tener acceso, facturación y límites distintos.
- ¿Cómo rotar/revocar credenciales, detectar cuál cuenta está autenticada y mostrar claramente qué sesión se utilizará?
- ¿Qué proveedor expone de forma confiable el uso restante, cuota o razón de rate limit? ¿Qué hacemos cuando no expone esos datos?
- ¿Cómo se verifica la compatibilidad con términos de cada suscripción y política de uso? No debe diseñarse un mecanismo que eluda límites.

### Datos, permisos y gobierno

- ¿Cómo clasificar tareas y artefactos: personal, corporativo, público, sensible u opciones configurables?
- ¿Puede un administrador imponer una prohibición de salida que el usuario final no pueda anular?
- ¿La vista previa muestra el paquete completo y sus adjuntos, o permite aprobación por secciones?
- ¿Qué evidencia queda en el registro y por cuánto tiempo? ¿Qué contenido nunca debe almacenarse?
- ¿Cómo se evita que un proveedor lea por defecto archivos irrelevantes del repo al recibir un paquete de tarea?

### Enrutamiento y calidad

- ¿El MVP recomienda perfiles o los selecciona automáticamente entre los elegibles?
- ¿Qué señales observables alimentan el router: herramientas disponibles, etiqueta de tarea, fallos, latencia, costo, cuota reportada, preferencias?
- ¿Cómo medir éxito, calidad de entrega, tiempo y consumo sin asumir que una puntuación de confianza del modelo es correcta?
- ¿Cuándo escalar a asesor, cuándo reintentar con el mismo perfil y cuándo detenerse?
- ¿Qué clases de acciones exigen siempre confirmación humana, independientemente del perfil?

## 7. Qué falta investigar

### Investigación prioritaria para reducir riesgo técnico

1. **Autenticación aislada por perfil:** prueba de concepto con dos cuentas Claude y Codex, verificando que invocar un perfil no cambie ni reutilice accidentalmente la identidad del otro. No copiar tokens manualmente entre almacenes; comprobar opciones documentadas por CLI/SDK.
2. **Superficie de ejecución soportada:** comparar CLI, app server, SDK y API para cada herramienta objetivo: invocación, sesiones, streaming, cancelación, trabajo de fondo, archivos, MCP, autenticación y facturación.
3. **Acceso a herramientas MCP por perfil:** comprobar cómo exponer el MCP corporativo solo al perfil corporativo y evitar que una tarea derivada lo herede al ejecutarse en un perfil personal.
4. **Importación de conversaciones vs paquete curado:** revisar límites y metadatos del traspaso de `codex-plugin-cc`; comparar riesgo de transferir transcript completo con un paquete mínimo generado por nuestra aplicación.
5. **Aislamiento de archivos:** estudiar worktrees/directorios por tarea, permisos de escritura, diffs y conflictos; valorar modo read-only para exploración/revisión.
6. **Datos de cuota/costo:** documentar qué cuenta cada método de autenticación y qué telemetría ofrece cada proveedor. No inferir cuota restante a partir de tokens si no está expuesta oficialmente.

### Investigación de producto

7. Entrevistar a usuarios que mantengan cuentas personal/corporativa de uno o varios proveedores; validar frecuencia, impacto y disposición a instalar un orquestador.
8. Mapear alternativas cercanas: Orcha, `codex-plugin-cc`, Agent Switchboard y otras integraciones de agentes/CLI; comparar capacidades verificadas, no solo páginas de presentación.
9. Probar con tareas representativas el valor real del contexto portable: qué información necesita el agente receptor, qué se puede omitir y qué errores ocurren por resúmenes incompletos.
10. Validar Perplexity como integración: modalidad de acceso, costos, cita/fecha de fuentes, límites y si hay una vía soportada desde el runtime seleccionado.

### Seguridad y operación

11. Definir modelo de amenazas local: almacenamiento de credenciales, logs, dumps, telemetría, MCPs, archivos, proceso hijo y acceso a red.
12. Definir políticas organizacionales configurables, auditoría local, retención y exportación/borrado de datos.
13. Evaluar recuperación ante caídas: reanudar trabajos, deduplicar ejecución, preservar resultados y comunicar estado incierto sin repetir acciones automáticamente.

## 8. Propuesta de plan de trabajo SDD

Este plan es una recomendación para orientar las siguientes conversaciones; todavía no es autorización para implementar.

### Fase 0 — Cerrar problema y usuario

- Convertir “varias herramientas y cuentas” en uno o dos flujos principales.
- Precisar usuario individual inicial y límites corporativos.
- Decidir si el primer valor es continuidad, ahorro/uso, MCPs por identidad o delegación paralela.
- Resultado: problema, alcance MVP y fuera de alcance aceptados.

### Fase 1 — Actualizar requisitos

- Añadir configuración dinámica de aplicaciones y cantidad de cuentas por aplicación.
- Añadir perfil como identidad/permisos/herramientas, no solo proveedor/modelo.
- Añadir requisitos de elegibilidad, recomendación de perfil, explicación y escalamiento.
- Especificar política corporativa y vista previa/aprobación del contexto.
- Ajustar aceptación para un caso configurable, no hardcodeado a Claude.
- Resultado: especificación 001 revisada y aprobada.

### Fase 2 — Spikes de viabilidad

- Probar aislamiento real de cuentas y persistencia de autenticación.
- Invocar un agente, enviar tarea acotada, recibir resultado, cancelar y retomar.
- Comprobar separación MCP personal/corporativo y contexto de repo.
- Documentar límites y facturación de cada flujo.
- Resultado: decisión de integración por proveedor con evidencia y riesgos.

### Fase 3 — Diseño técnico

- Definir modelo de datos: usuario, aplicación, perfil, capacidades, política, proyecto, tarea, paquete de contexto, aprobación, ejecución y evento de auditoría.
- Definir contrato/adaptador de proveedor y contrato del agente/runtime.
- Diseñar flujo de selección, vista previa, autorización, ejecución, resultado, revisión y traspaso.
- Diseñar almacenamiento de secretos, aislamiento y manejo de fallos.
- Resultado: arquitectura, diagramas, seguridad y decisiones registradas.

### Fase 4 — Plan y MVP vertical

- Implementar un recorrido completo con el mínimo número de perfiles/integraciones confirmado en las fases anteriores.
- Mantener asignación explícita y aprobación de transferencias sensibles.
- Diferir automatización avanzada de cuota, Perplexity y coordinación masiva hasta validar el flujo principal.
- Resultado: plan de entrega trazable a requisitos y criterios de aceptación.

### Fase 5 — Verificación y decisión de expansión

- Verificar cuentas separadas, permisos, contexto transferido, cancelación, continuidad y diff revisable.
- Evaluar recomendaciones de enrutamiento con un conjunto controlado de tareas y registrar calidad, costo y tiempo.
- Ampliar proveedores solo después de verificar autenticación, política, costo e interfaz de ejecución de cada uno.
- Resultado: evaluación del MVP y siguiente incremento decidido con evidencia.

## 9. Recomendación inmediata

El paso que más reduce riesgo es **revisar y aprobar los requisitos antes de escoger arquitectura**. En particular, convertir la decisión de configuración dinámica de cuentas y el orden “elegibilidad primero, optimización después” en requisitos explícitos. Después, hacer un spike pequeño de autenticación y herramientas por perfil para saber qué integración real es posible antes de prometer el enrutamiento automático.

## 10. Referencias

- [Especificación 001 del proyecto](../specs/001-product-requirements.md)
- [Orcha — repositorio](https://github.com/open-orcha/orcha)
- [Orcha — hallazgos y roadmap](https://github.com/open-orcha/orcha/blob/main/docs/orcha-roadmap-and-findings.md)
- [Orcha — guía BYOC y seguridad](https://github.com/open-orcha/orcha/blob/main/docs/byoc-guide.md)
- [`codex-plugin-cc` — repositorio](https://github.com/openai/codex-plugin-cc)
- [`codex-plugin-cc` — README y flujos](https://github.com/openai/codex-plugin-cc/blob/main/README.md)
- [MCP en Codex](https://developers.openai.com/learn/docs-mcp)
- [Guía de selección de modelos OpenAI](https://developers.openai.com/api/docs/guides/latest-model?model=gpt-6-astra)
