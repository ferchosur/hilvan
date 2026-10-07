# Modelo de amenazas

- **Estado:** Accepted (baseline del MVP)
- **Fecha:** 2026-10-06
- **Método:** STRIDE ligero sobre los límites de confianza del MVP local.
- **Relacionado:** NFR-001, NFR-002, NFR-008, NFR-010, NFR-011, ADR-0006, ADR-0009
- **Ampliado por:** [análisis de riesgos (2026-10-07)](../security/analisis-de-riesgos.md) y [requisitos de seguridad SR-001…SR-031](../security/requisitos.md). Este documento resume el diseño; el análisis detallado y el estado de los controles están allí.

## 1. Activos y límites de confianza

**Activos:**

- Credenciales de los proveedores, que están en el directorio de configuración de cada perfil y que **Hilvan no lee**.
- Datos corporativos dentro de tareas y paquetes.
- Código del repositorio.
- El registro de auditoría.

**Límites de confianza:**

1. La persona y Hilvan.
2. Hilvan y cada proceso de agente (el agente es un actor semiconfiable: obedece a un modelo y al contenido del repositorio).
3. Un perfil y otro perfil (separación de identidad y de clasificación).
4. El repositorio y la configuración de Hilvan (un repositorio clonado puede ser hostil).

## 2. Amenazas y controles

| # | Amenaza (STRIDE) | Escenario | Control en el MVP |
|---|---|---|---|
| T1 | Divulgación de información | Contexto corporativo enviado a un perfil personal | Elegibilidad antes de recomendar; reglas `deny` gestionadas y `enforced`; vista previa y aprobación obligatorias; el hash de lo enviado queda en la auditoría (AC-003, AC-014). |
| T2 | Divulgación de información | Secretos (tokens, claves) dentro del paquete de traspaso o de los logs | Detector de secretos (patrones de claves conocidas y alta entropía) antes de la vista previa, con las coincidencias marcadas; `events.jsonl` guarda solo hashes y rutas; `.hilvan/` en `.gitignore`. |
| T3 | Suplantación | Una tarea se ejecuta con la cuenta equivocada | `CLAUDE_CONFIG_DIR`/`CODEX_HOME` explícitos por ejecución; `profile check` verifica la cuenta (Claude) antes de la primera ejecución y tras cambios de configuración; el destino se muestra en cada confirmación (AC-017). |
| T4 | Elevación de privilegios | El repositorio trae `.mcp.json` o hooks que `claude -p` ejecuta sin pedir confianza | `--strict-mcp-config` siempre y solo el MCP declarado en el perfil (NFR-011). Riesgo residual: los hooks de `.claude/settings.json` del repositorio siguen activos sin `--bare` → se avisa la primera vez que se usa un repositorio con hooks y se documenta. |
| T5 | Manipulación | Inyección de argumentos o comandos mediante rutas, títulos o prompts | `spawn` con array de argumentos y `shell:false`; el prompt va por stdin, nunca como argumento interpolado; validación de rutas y de `id` con zod. |
| T6 | Manipulación | Inyección de prompts desde contenido del repositorio o desde la salida de otro agente | Revisión humana antes de aceptar (AC-010); revisión cruzada opcional; las tareas de lectura usan sandbox `read-only` o `dontAsk`. |
| T7 | Repudio | No se puede saber qué perfil recibió qué | `events.jsonl` de solo añadido con perfil, destino, hash del paquete y resultado (FR-009). |
| T8 | Denegación de servicio / coste | Bucles que consumen cuota | Ejecución secuencial y sin reintentos automáticos entre perfiles; `limits` y `usage` visibles; los límites detienen la tarea (`blocked`). |
| T9 | Cumplimiento | Uso de credenciales de suscripción contra los términos | Solo el CLI oficial sin modificar; nunca se manejan tokens; ningún login propio (NFR-010, R-006). |
| T10 | Divulgación de información | Los datos de los perfiles quedan en `/tmp` o en directorios compartidos | Los directorios de perfil se crean con permisos `0700` fuera de `/tmp`; `doctor` avisa si los permisos son más abiertos. |

## 3. Riesgos aceptados en el MVP

- Un administrador local puede modificar la política gestionada: el MVP no es un control frente a administradores del propio equipo.
- Los hooks del repositorio en `claude -p`, mitigados solo con un aviso (T4).
- Codex no permite verificar la cuenta concreta (`verifiable: false`).
