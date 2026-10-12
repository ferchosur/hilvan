# AgentBridge: ideas útiles y límites para Hilvan

- **Fecha de consulta:** 2026-10-12
- **Fuente:** [raysonmeng/agent-bridge](https://github.com/raysonmeng/agent-bridge), licencia MIT, última actualización del repositorio el 2026-10-11 (datos de la API de GitHub, consultados el 2026-10-12).
- **Relacionado:** AC-003, AC-018, D-003, FR-004, [propuestas pendientes](2026-10-08-propuestas-pendientes.md), AGENTS.md (reglas 4 y 6, y "código de terceros").

## Qué es

Un puente local que conecta Claude Code y Codex dentro de una misma sesión de trabajo: los mensajes de un agente llegan al otro sin copiar y pegar, y un proceso en segundo plano mantiene las dos sesiones abiertas. Según su README, la mayor parte del proyecto se hizo con los dos agentes revisándose mutuamente.

Lo que dice de sí mismo, sin verificarlo: revisión cruzada, reparto de tareas negociado entre agentes, y **relevo de cuota** para continuar una tarea cuando una de las suscripciones llega a su límite.

## Qué tomamos como ideas

Se reimplementarían a partir de la descripción, sin copiar código.

1. **Solo viaja lo que el otro agente necesita.** El puente reenvía los mensajes del agente, no sus llamadas a herramientas ni su historial completo. Es el mismo principio que el paquete de traspaso revisable de Hilvan (FR-004). Podemos reforzarlo: el resumen del traspaso debería incluir solo conclusiones.
2. **Qué hacer si el otro agente está ocupado.** Su *busy-guard* rechaza respuestas durante un turno activo. Hilvan ejecuta en secuencia, pero no ha definido qué ocurre con un perfil ocupado al lanzar una segunda tarea o una revisión cruzada. Hay que decidirlo en la especificación antes de implementarlo.
3. **Vigilancia de turnos colgados.** Un *watchdog* por inactividad evita que un evento de fin de turno perdido deje la integración bloqueada para siempre. En Hilvan encaja con los estados `uncertain` y `blocked` y con la recuperación de ejecuciones interrumpidas que ya existen.
4. **Un mensaje de "hecho" no abre una conversación nueva.** Útil para la revisión cruzada (AC-018): el resultado de una revisión debe quedar adjunto a la tarea, sin reiniciar la ejecución.
5. **Cada envío automático cuesta una llamada al modelo.** El propio README lo admite, aunque la respuesta sea "nada que añadir". Respalda la propuesta de requisito "Hilvan no aumenta el consumo" de [propuestas pendientes](2026-10-08-propuestas-pendientes.md).
6. **Un agente hace el trabajo y el otro revisa.** Ya es el modelo de Hilvan (AC-018, revisión de solo lectura). La coincidencia confirma la decisión, pero no aporta nada nuevo.

## Qué descartamos

- **Relevo de cuota y auto-reanudación entre ventanas de suscripción.** Pasar una tarea a la otra cuenta cuando una se queda sin cuota es eludir límites de proveedor. CONTEXTO.md y AGENTS.md (regla 6) lo excluyen expresamente. No se estudiará como funcionalidad.
- **Puente continuo entre agentes sin aprobación.** Cada mensaje que sale de una cuenta es contexto que sale de esa identidad. Hilvan exige vista previa y aprobación para cada envío entre cuentas distintas (AC-003, D-003). Un canal que comparte mensajes en automático, con agentes en cuentas distintas, lo incumple.
- **Copiar código.** AGENTS.md exige permiso explícito y verificación de licencia antes de copiar código de terceros. No hace falta: las ideas se reimplementan y se citan aquí.

## Efecto en el producto

No cambia el alcance del MVP ni de la especificación 001. Las ideas 2 y 3 son candidatas a una decisión sobre ejecución concurrente o con perfiles ocupados; la 1 y la 4 pueden incorporarse al diseño de la revisión cruzada; la 5 es un argumento para el requisito de consumo.

## Pendiente

- Decidir si el comportamiento con un perfil ocupado entra en la especificación (y en qué versión).
- Decidir si la revisión cruzada debe guardar un resumen de conclusiones en lugar del transcript completo.
