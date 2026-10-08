# Verificación del MVP — criterios de aceptación

- **Fecha:** 2026-10-08, con la implementación en Go ([plan 002](002-migracion-go.md), G5)
- **Base:** [spec 001 v1.4](../../specs/001-product-requirements.md) §9 y [plan 001](001-mvp.md)
- **Evidencia automática:**
  - 109 tests de Go (`go run ./scripts/check`), en CI en ubuntu y windows;
  - prueba diferencial con la CLI TypeScript: 61/61 pasos idénticos (`scripts/parity/diff-cli.ts`, job `parity`);
  - casos dorados del código TS: 2.507 operaciones de Criterio, parsers, auditoría y `task.json`.
- **Evidencia e2e real (2026-10-08, Linux):** binario de Go con Claude Code 2.1.293 y codex-cli 0.160.1, usando los logins por defecto del usuario.

| AC | Criterio (resumen) | Estado | Evidencia (Go) |
|---|---|---|---|
| AC-001 | Dos perfiles de la misma aplicación, distinguibles | ✔ | `core.TestProfileRegistry`; e2e: `profile add`/`list`/`check` con dos cuentas |
| AC-002 | Asignar a cualquier perfil elegible sin compartir credenciales | ✔ | Entorno por perfil (`adapters.TestCommandsMatchTypeScript`, `launchers.TestNative`); `core.TestHandoffContinuesBlockedTask` |
| AC-003 | Tarea corporativa → perfil personal: no se envía sin vista previa y aprobación | ✔ | `core.TestApprovalShowsBriefAndDeclineSendsNothing`, `core.TestApprovedRunGoesAhead` |
| AC-004 | Cancelar la aprobación no envía el paquete | ✔ | `core.TestApprovalShowsBriefAndDeclineSendsNothing`, `core.TestCancelledHandoffSendsNothing`; prueba diferencial (`handoff` sin terminal) |
| AC-005 | El paquete de continuidad permite continuar sin el historial | ✔ | `core.TestHandoffContinuesBlockedTask`; e2e: Codex → paquete aprobado → Claude continúa y cita los hechos del paquete |
| AC-006 | Una herramienta del perfil corporativo no aparece en el personal | ✔ | `policy.TestToolOnlyOnCorporateProfile`; argumentos MCP por perfil (`adapters.TestCommandsMatchTypeScript`) |
| AC-007 | Ante un límite se pide decisión, sin cambio silencioso de perfil | ✔ | `core.TestBlocksAndRefusesSilentSwitch`; prueba diferencial |
| AC-008 | El historial muestra perfil, contexto aprobado y resultado | ✔ | `task show`; cadena de hash compatible con TS (`store.TestVerifiesTypeScriptChain`, `store.TestDetectsTampering`); e2e: `audit verify` |
| AC-009 | Investigación con Perplexity y fuentes | ⏸ diferido | D-004 (sin créditos de API) |
| AC-010 | Revisar los cambios antes de aceptarlos | ✔ | `core.TestAcceptRequiresReview`; e2e: Claude escribe → `review` → `accept` |
| AC-011 | Cero, uno o varios perfiles por aplicación sin nombres fijos | ✔ | `domain.TestProfilesFile`, `core.TestProfileRegistry` |
| AC-012 | Perfil sin autorización o sin herramienta: excluido con motivo | ✔ | `policy.TestClassificationExcludes`; `task recommend` en la prueba diferencial y en e2e |
| AC-013 | Recomendación explicada y confirmación antes de reasignar | ✔ | `core.TestReservedRouting`, `criterio.TestRanking`; e2e: `task recommend` explica sin ejecutar |
| AC-014 | Una restricción obligatoria bloquea aunque el usuario apruebe | ✔ | `core.TestEnforcedDenyCannotBeApproved`, `core.TestEnforcedRuleBlocksHandoff`, `policy.TestEnforcedOnlyInManagedLayer` |
| AC-015 | Dos perfiles de la misma app en entornos distintos del mismo PC | ◐ | Lanzadores y traducción de rutas (`launchers.*`), `.cmd` real en la CI de Windows, descubrimiento de WSL (`core.TestWSLDiscovery`). Spike 001: aislamiento sin login verificado en WSL, Windows y Windows → WSL, y worktrees relativos utilizables desde WSL. **Falta la verificación con sesión iniciada en el equipo de validación.** |
| AC-016 | El orquestador lanza el destino, entrega instrucción y contexto, y registra el resultado | ✔ | `adapters.TestRunner*`, `core.TestWriteTaskRunsInWorktree`; e2e con Claude y Codex |
| AC-017 | Una tarea no usa por error el ejecutable, carpeta o autenticación de otro perfil | ✔ | Choque de carpetas (`core.TestProfileRegistry`), claves heredadas eliminadas (casos dorados de `profileEnv`), sesiones solo del mismo perfil (`RunTask`) |
| AC-018 | Revisión de solo lectura por un segundo perfil, adjunta a la tarea | ✔ | `core.TestCrossReview`; e2e: revisión cruzada real de Codex (`VERDICT: approve`) adjunta a la tarea |
| AC-019 | Coste/tokens "estimados" o "no disponibles" por ejecución | ✔ | `core.TestWriteTaskRunsInWorktree`, `core.TestUsageByProfile`; e2e: Claude con coste, Codex solo tokens |
| AC-020 | Proceso interrumpido → `uncertain`, sin reintento | ✔ | `adapters.TestRunnerCrashIsUncertain`, `core.TestRecoversInterruptedRun` |
| AC-021 | Worktree propio; aceptar fusiona y limpia | ✔ | `core.TestWriteTaskRunsInWorktree` (enlace del worktree relativo), `core.TestAcceptMergesAndTeaches`; e2e: fusión `--no-ff` y worktree eliminado |
| AC-022 | Criterio sin red, menos de 5 ms y que aprende de correcciones | ✔ | `criterio.TestPerformance`, `criterio.TestLearnsFromCorrections`, `criterio.TestGoldenEngine` |
| AC-023 | Personal por defecto, corporativo reservado | ✔ | `core.TestReservedRouting` |
| AC-024 | Criterio sugiere y aprende las herramientas requeridas | ✔ | `core.TestRequiredToolsCatalogAndLearning`, `criterio.TestGoldenEngine` |
| AC-025 | Ni hooks ni MCP del repositorio; sí los de la cuenta | ✔ | Argumentos idénticos a TS (`--setting-sources user`, casos dorados). **E2e 2026-10-08:** en un repo con un hook `SessionStart` y un `.mcp.json` hostiles, la ejecución vía Hilvan no disparó el hook y arrancó sin servidores MCP; el control con `claude -p` directo sí disparó el hook |

**NFR-013 (ligereza):** `hilvan --version` tarda 3 ms de mediana con 5,5 MB de pico de memoria (`cmd/hilvan.TestStartupAndMemoryBudget`, presupuesto de 50 ms y 40 MB). Cada decisión de Criterio tarda menos de 5 ms (`criterio.TestPerformance`).

**Resultado (2026-10-08, Go):** 23 de 25 verificados, AC-015 parcial (pendiente de la verificación con sesión en el equipo de validación) y AC-009 diferido. Es el mismo resultado que con TypeScript; además AC-025 se verificó de nuevo en real.

**Hallazgos del e2e de G5:**

- **Codex:** `codex exec` funciona aunque el servidor en segundo plano (0.161.0) y el CLI (0.160.1) tengan versiones distintas. El aviso de "incompatible feature settings" solo afecta al CLI interactivo.
- **Tareas de solo lectura con Claude:** corren con `--permission-mode dontAsk`, que impide incluso comandos de Bash de solo lectura (`ls`, `git ls-files`). El agente lo declara y Criterio rebaja su estimación de cumplimiento. Mejora candidata: permitir comandos de lectura con `--allowedTools`.

**Resultados anteriores (TypeScript):** 23 de 25 el 2026-10-07 tras FR-021; 20 de 22 antes.
