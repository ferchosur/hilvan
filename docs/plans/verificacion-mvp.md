# Verificación del MVP — criterios de aceptación

- **Fecha:** 2026-10-07
- **Base:** [spec 001 v1.2](../../specs/001-product-requirements.md) §9 y [plan 001](001-mvp.md)
- **Evidencia:** 155 tests automáticos (`npm run check`) y pruebas e2e reales en Linux con Claude Code 2.1.292 y codex-cli 0.160.1.

| AC | Criterio (resumen) | Estado | Evidencia |
|---|---|---|---|
| AC-001 | Dos perfiles de la misma aplicación, distinguibles | ✔ | `tests/profiles.test.ts` (*supports two profiles*); `hilvan profile list` real |
| AC-002 | Asignar a cualquier perfil elegible sin compartir credenciales | ✔ | Entorno por perfil (`tests/adapters.test.ts`, `launchers.test.ts`); `runTask` con perfiles distintos (`handoff.test.ts`) |
| AC-003 | Tarea corporativa → perfil personal: no se envía sin vista previa y aprobación | ✔ | `tests/policy.test.ts` (*shows the exact brief…*); `hilvan task run --approve` y prompt interactivo |
| AC-004 | Cancelar la aprobación no envía el paquete | ✔ | `policy.test.ts` (rechazo), `handoff.test.ts` (*cancelling sends nothing*) |
| AC-005 | El paquete de continuidad permite continuar sin el historial | ✔ | `handoff.test.ts` (tarea bloqueada → paquete → ejecución en otro perfil) |
| AC-006 | Una herramienta del perfil corporativo no aparece en el personal | ✔ | `policy.test.ts` (*a tool connected only to the corporate profile…*); `--strict-mcp-config` por perfil |
| AC-007 | Ante un límite se pide decisión, sin cambio silencioso de perfil | ✔ | `tasks.test.ts` (*blocks on a usage limit and refuses to switch…*) |
| AC-008 | El historial muestra perfil, contexto aprobado y resultado | ✔ | `task show`, auditoría con cadena de hash (`security.test.ts`, `tasks.test.ts`) |
| AC-009 | Investigación con Perplexity y fuentes | ⏸ diferido | D-004 (sin créditos de API) |
| AC-010 | Revisar los cambios antes de aceptarlos | ✔ | `review.test.ts` (*requires the current diff to be reviewed…*); e2e real: tarea de escritura con Claude → `review` → `accept` |
| AC-011 | Cero, uno o varios perfiles por aplicación sin nombres fijos | ✔ | `profiles.test.ts`, `domain.test.ts` |
| AC-012 | Perfil sin autorización o sin herramienta: excluido con motivo | ✔ | `policy.test.ts`; `hilvan task recommend` real |
| AC-013 | Recomendación explicada y confirmación antes de reasignar | ✔ | `policy.test.ts` (*ranks only eligible…*); `criterio.test.ts` (ranking) |
| AC-014 | Una restricción obligatoria bloquea aunque el usuario apruebe | ✔ | `policy.test.ts` y `handoff.test.ts` (*enforced…*) |
| AC-015 | Dos perfiles de la misma app en entornos distintos del mismo PC | ◐ | Lanzadores `wsl`/`windows` con tests unitarios; **falta validarlo en Windows (spike 001)** |
| AC-016 | El orquestador lanza el destino, entrega instrucción y contexto, y registra el resultado | ✔ | `adapters-run.test.ts`, `tasks.test.ts`; e2e real con Claude y Codex |
| AC-017 | Una tarea no usa por error el ejecutable, carpeta o autenticación de otro perfil | ✔ | Choque de carpetas (`profiles.test.ts`), claves heredadas eliminadas (`adapters.test.ts`), sesiones solo del mismo perfil (`handoff.test.ts`) |
| AC-018 | Revisión de solo lectura por un segundo perfil, adjunta a la tarea | ✔ | `review.test.ts`; e2e real: Claude rechazó la respuesta alucinada de Codex |
| AC-019 | Coste/tokens "estimados" o "no disponibles" por ejecución | ✔ | `tasks.test.ts` (uso y límites); e2e real (Claude con coste, Codex solo tokens) |
| AC-020 | Proceso interrumpido → `uncertain`, sin reintento | ✔ | `adapters-run.test.ts` (*crash*), `tasks.test.ts` (*recovery*) |
| AC-021 | Worktree propio; aceptar fusiona y limpia | ✔ | `tasks.test.ts`, `review.test.ts`; e2e real: `HELLO.md` creado por Claude en `hilvan/<tarea>`, fusionado con `--no-ff`, worktree eliminado |
| AC-022 | Criterio sin red, menos de 5 ms y que aprende de correcciones | ✔ | `criterio.test.ts` (rendimiento y aprendizaje) |

**Resultado:** 20 de 22 verificados, 1 parcial (AC-015, pendiente de Windows) y 1 diferido (AC-009).
