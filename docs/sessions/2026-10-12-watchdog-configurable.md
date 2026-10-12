# 2026-10-12 — Watchdog configurable, límite de revisión y medición de silencios

- **Agente/herramienta:** Claude Code (Haiku 5.5), con el usuario presente.
- **Commits en `main`:** `65bf59c` (implementación), fusionada en `767767c`.

## Objetivo

Decidir el límite del watchdog con criterio y dejar medidas para afinarlo después, a partir de la revisión de la sesión anterior.

## Hecho

- Watchdog por defecto de 10 a 20 minutos, configurable con `--idle-timeout` en `task run` y `review`. Valor provisional.
- La revisión cruzada tiene límite duro de 30 minutos (`review --timeout`); antes solo tenía el watchdog.
- Cada ejecución y cada revisión guarda `maxSilenceSeconds` en `task.json`. Una ejecución real de prueba registró 2,1 s.

## Decisiones

- Límite de revisión de 30 minutos, la mitad del de `task run`: una revisión es de solo lectura y debe tardar menos. Elección mía, pendiente de confirmar con datos.
- Los valores de timeout deben ser positivos y se validan antes de buscar la tarea.

## Qué sigue

Medir silencios en más tareas reales, incluidas las que ejecutan tests, antes de fijar el valor definitivo del watchdog. Pendiente de decisión del usuario: qué hacer con un perfil ocupado.
