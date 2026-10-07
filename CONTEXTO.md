# Contexto para continuar el proyecto

Última actualización: 2026-10-06  
Estado: descubrimiento y definición de producto; todavía no hay implementación.

Este documento conserva el contexto esencial para retomar el trabajo en otra sesión. La fuente de verdad de los requisitos es `specs/001-product-requirements.md`; el análisis y plan preliminar están en `docs/estado-y-oportunidades.md`.

## Objetivo del producto

Diseñar una aplicación configurable para coordinar agentes de IA entre proveedores y entre varias cuentas de una misma aplicación. Debe reducir la reconstrucción manual de contexto, permitir asignar trabajo según capacidad y herramientas disponibles, y mantener separación y control sobre cuentas, credenciales, permisos y datos.

El caso que originó la idea: usar una cuenta Claude corporativa que tiene acceso a MCPs internos, una cuenta personal de Claude con mayor rendimiento/duración para trabajo genérico y una cuenta personal de Codex. Es solo un ejemplo: **cada usuario elige qué aplicaciones conectar y cuántas cuentas utilizar por aplicación**. No se deben codificar proveedores o cantidades fijas en el producto.

## Problema y valor buscado

- Las conversaciones y tareas viven separadas entre herramientas y cuentas.
- Cambiar de agente por especialidad, límites, disponibilidad o herramientas obliga a volver a explicar el objetivo y el estado.
- Se busca coordinar agentes y conservar continuidad verificable, no fusionar cuotas ni prometer migraciones perfectas de historiales privados.
- El enrutamiento debería usar el perfil suficiente para cada decisión y tarea, en lugar de usar siempre el modelo más potente.

## Decisiones y principios acordados

1. **Configuración por usuario:** el usuario elige las aplicaciones y el número de cuentas/perfiles por aplicación.
2. **Identidad por perfil:** cada cuenta es independiente aunque use el mismo proveedor; debe conservar autenticación, capacidades, conectores y permisos separados.
3. **Selección por tarea:** asignación manual inicial; el sistema puede recomendar perfiles explicando por qué. No cambiar de identidad silenciosamente.
4. **Elegibilidad antes de optimización:** primero verificar permisos, clasificación de datos y herramientas requeridas; solo entre perfiles elegibles optimizar capacidad suficiente, disponibilidad, consumo/costo conocido y preferencia.
5. **Cruce personal/corporativo:** el usuario podría permitir el uso de perfil personal en una tarea corporativa, pero antes debe ver el contexto exacto y aprobarlo para esa tarea. La aprobación del usuario no anula reglas corporativas obligatorias; falta diseñar políticas administrativas.
6. **Traspaso basado en artefactos:** paquete compacto con objetivo, estado, hechos, decisiones, supuestos, archivos/diffs, resultados y próximo paso. No copiar transcript completo por defecto.
7. **Cuotas separadas:** no sumar cuotas, eludir límites ni ocultar la modalidad de facturación. Si una tarea falla por límite, solicitar decisión explícita antes de usar otro perfil.
8. **Revisión humana:** resultados y cambios deben ser inspeccionables; la persona decide sobre cambios principales.
9. **Perplexity:** candidato opcional para investigación con fuentes citadas; no es necesario para ejecutar código. Investigar acceso y coste antes de incluirlo.
10. **SDD:** mantener etapas requisitos → diseño → plan → implementación → verificación. Un borrador no autoriza implementación.

## Idea de enrutamiento derivada de la arquitectura Advisor/JEV

La imagen compartida presentaba un agente principal que planifica, subagentes especialistas (worker/explorer/researcher), un asesor que revisa en momentos importantes y una capa ligera de decisión para opciones acotadas (p. ej. herramienta/archivo/reintentar/detener), con escalamiento ante ambigüedad.

Aplicación al producto:

1. Clasificar requisitos de la tarea y herramientas necesarias.
2. Filtrar perfiles no autorizados o incapaces de cumplirla.
3. Elegir/recomendar entre perfiles elegibles con explicación.
4. Mostrar y autorizar el paquete de contexto si el flujo cruza límites de privacidad.
5. Ejecutar con especialista; escalar a asesor/persona ante decisiones de impacto, incertidumbre o errores repetidos.
6. Revisar resultado/diff antes de aceptar.

Las cifras de la imagen (forks, probabilidades, llamadas, precios) eran ilustrativas; no se consideran benchmark validado. El asesor puede ser de solo lectura inicialmente.

## Herramientas similares revisadas

### Orcha — https://github.com/open-orcha/orcha

- Coordina agentes con tareas, dependencias, mensajería, continuidad, supervisión humana, dashboard y métricas.
- Incluye runtimes Claude y Codex con diferencias de madurez; ofrece muchas funciones de coordinación que no necesitamos redescubrir.
- La oportunidad para este proyecto no es solo “varios agentes”: es gestionar perfiles de cuenta configurables, herramientas/permisos ligados a cada cuenta, enrutamiento elegible y transferencias de contexto con aprobación.
- En la sesión de análisis, el clon local quedó en `/home/kelebra/Descargas/claude_projects/orcha`, commit observado `b647cab` (2026-09-29). El README y algunos documentos tenían desfase frente al código.
- El informe documenta limitaciones y brechas reconocidas por Orcha; no asumir que siguen sin resolver sin revisar la versión actual.

### OpenAI `codex-plugin-cc` — https://github.com/openai/codex-plugin-cc

- Desde Claude Code delega trabajo/revisión a Codex, ejecuta trabajos en segundo plano, consulta estado/resultados, permite cancelar, y ofrece revisión adversarial.
- `/codex:transfer` importa una sesión de Claude a Codex para continuarla.
- Reutiliza la instalación, la autenticación y la configuración local de Codex; es un puente Claude Code → Codex, no un gestor de cuentas múltiples ni un router general.
- La transferencia de sesión hace más evidente la necesidad de poder inspeccionar/minimizar el contexto antes de enviarlo entre identidades.
- Patrones reutilizables: comandos claros, estado de trabajo, continuidad, delegación background, cancelación y revisión externa.

### Perplexity

- Se ha considerado como especialista de investigación con resultados citados, opcional.
- Pendiente verificar modalidad de acceso programático, precios, límites, citas y si una suscripción web cubre el uso requerido. No asumirlo.

## Diferenciación propuesta (hipótesis a validar)

- Router agnóstico que permite al usuario habilitar proveedores y cuentas con distintas vías de invocación.
- Modelo de identidad/perfil por cuenta, no solo por marca o modelo.
- Políticas y MCPs asociados al perfil concreto.
- Enrutamiento que filtra por permisos/privacidad antes de comparar costo y capacidad.
- Paquete portable, minimizado y visible para continuidad entre cuentas.
- Aprobación editable/cancelable para cruces personales/corporativos, y auditoría de qué perfil recibió qué contexto.
- Escalamiento por impacto/incertidumbre, combinado con selección de cuenta/proveedor.

No afirmar que estos elementos son únicos en todo el mercado hasta completar benchmarking actualizado.

## Documentos y estado

- `README.md`: método SDD y lista de documentos.
- `specs/001-product-requirements.md`: versión 0.3, Draft (2026-10-06); incorpora configuración dinámica de aplicaciones/perfiles, elegibilidad antes de optimización, recomendaciones explicables, restricciones obligatorias y destino/adaptador de ejecución por perfil. Conserva Q-001–Q-010.
- `docs/estado-y-oportunidades.md`: informe detallado de estado, reutilización, diferenciación, investigación y fases de trabajo (2026-10-04).
- La configuración dinámica de aplicaciones/perfiles y la elegibilidad antes de optimizar ya están incorporadas en la versión 0.2 de la especificación. Sigue en Draft: aún falta revisar y aprobar el baseline.
- La versión 0.3 incorpora la necesidad de ejecutar dos perfiles de la misma aplicación en entornos distintos del mismo PC (p. ej. Windows nativo y una distro WSL). El perfil debe resolver a un destino/adaptador explícito; las preguntas sobre proceso no interactivo vs. puente de sesión y ubicación del runner siguen abiertas.
- No hay diseño técnico aprobado ni código del producto.
- No se instalaron/configuraron cuentas de Claude, Codex o Perplexity como parte del proyecto.

## Preguntas críticas abiertas

### Producto

- Plataforma primaria: escritorio local, web local, CLI/terminal o combinación.
- Sistemas operativos de primera versión.
- Primer recorrido vertical: por ejemplo dos cuentas Claude, o Claude→Codex.
- ¿Tareas creadas en interfaz propia o desde agentes/herramientas ya utilizadas?
- ¿Solo uso individual/local inicialmente o preparar desde el inicio políticas de equipos?
- ¿Perplexity es MVP o incremento posterior?
- ¿MVP secuencial con diffs revisables o ejecución paralela aislada con worktrees?

### Proveedores, autenticación y runtime

- Qué interfaces soportadas usar por producto: CLI, app server, SDK/API o MCP.
- Demostrar aislamiento de dos identidades Claude y, si se incluye, Codex. Una configuración/model profile no prueba por sí misma que la autenticación quede separada.
- Compatibilidad de sesiones, historial, MCP, permisos, cancelación, streaming y procesos de fondo.
- Qué límites, gasto y facturación exponen los proveedores; no inferir cuota restante si no se reporta oficialmente.
- Verificar términos de uso y métodos permitidos para autenticación programática; no crear bypass de límites.

### Privacidad/seguridad

- Clasificación de datos y tarea (personal/corporativo/sensible/público) y política por proyecto.
- Si un administrador puede bloquear salidas sin opción de override por usuario.
- Granularidad de aprobación del paquete de contexto y manejo de archivos/adjuntos.
- Gestión de secretos, logs, retención, borrado, telemetría y recuperación.
- Aislamiento entre MCPs por perfil y acceso a archivos del repo.

### Router y evaluación

- Recomendación asistida inicialmente o asignación automática entre perfiles elegibles.
- Señales usadas (capacidades, herramientas, historial de éxito, disponibilidad, latencia, coste/límite conocido, preferencia).
- Criterios para elegir entre reintento, escalamiento a asesor, cambio de perfil o intervención humana.
- Benchmark propio con tareas representativas para medir calidad, coste, latencia, continuidad y filtración de datos.

## Próximos pasos recomendados

1. Revisar y resolver alcance/plataforma, primer flujo y destinos/adaptadores de ejecución (Q-001, Q-003, Q-004, Q-005, Q-009 y Q-010).
2. Revisar la versión 0.3 de `specs/001-product-requirements.md` y aprobar el baseline funcional y los criterios de aceptación del MVP.
3. Ejecutar spikes técnicos pequeños para autenticación de múltiples cuentas en Windows y WSL, aislamiento MCP, invocación/control del proceso y ejecución de agente; documentar resultados y límites por proveedor.
4. Elegir la arquitectura tras los spikes y escribir diseño técnico, amenazas, modelo de datos y plan.
5. Solo entonces implementar un MVP vertical y verificar sus criterios.

## Enlaces de referencia

- [Informe de estado y oportunidades](docs/estado-y-oportunidades.md)
- [Requisitos v0.2 (Draft)](specs/001-product-requirements.md)
- [Orcha](https://github.com/open-orcha/orcha)
- [`codex-plugin-cc`](https://github.com/openai/codex-plugin-cc)
- [Guía de MCP para Codex](https://developers.openai.com/learn/docs-mcp)
- [Selección de modelos OpenAI](https://developers.openai.com/api/docs/guides/latest-model?model=gpt-6-astra)
