# Spike 002 — Invocación no interactiva de Claude Code y Codex

- **Estado:** Done
- **Fecha inicio / fin:** 2026-10-06 / 2026-10-06
- **Límite de tiempo:** 1 h
- **Relacionado:** Q-009, FR-014, FR-016, FR-017, AC-016, AC-019, AC-020, ADR-0004

## Pregunta

¿Pueden `claude -p` y `codex exec` lanzarse como procesos hijos con salida JSON por líneas, consumo reportado, cancelación y código de salida aprovechables por un adaptador?

## Hipótesis

Sí, según la documentación oficial (ver [investigación](../../docs/research/2026-10-06-foros-y-herramientas.md#3-ejecución-no-interactiva)).

## Entorno

Linux (Ubuntu), Claude Code 2.1.292 con login de suscripción, codex-cli 0.160.1 con login ChatGPT, directorio de trabajo temporal y prompts triviales de solo lectura.

## Procedimiento

```bash
claude -p "Responde solo con la palabra OK." --output-format stream-json --verbose \
  --strict-mcp-config --permission-prompts none --permission-mode dontAsk < /dev/null
# cancelación: misma orden con un prompt largo y kill -INT tras 6 s
codex exec --json --sandbox read-only --skip-git-repo-check --ephemeral "Responde solo con la palabra OK." < /dev/null
CLAUDE_CONFIG_DIR=<dir vacío> claude auth status     # aislamiento
CODEX_HOME=<dir vacío> codex login status
```

## Resultados

Las muestras anonimizadas están en [`fixtures/`](fixtures/) y sirven como fixtures de los tests de los adaptadores.

**Claude Code** (`claude-stream-ok.jsonl`, `claude-stream-sigint.jsonl`):

- Secuencia: `system/init` → `assistant`* → `rate_limit_event` → `result`.
- `system/init` informa `model`, `mcp_servers` (vacío con `--strict-mcp-config`), `apiKeySource` (`none` con suscripción), `permissionMode`, `claude_code_version`, `cwd` y `capabilities` (p. ej. `interrupt_receipt_v1`).
- **`rate_limit_event.rate_limit_info`** expone `status`, `rateLimitType` y `unifiedWindows.five_hour|seven_day.utilization` (0–1) con `resetsAt`. **Es un dato oficial de uso de cuota por perfil** que sirve para la recomendación (FR-012) y para mostrar el consumo (FR-016).
- `result`: `subtype` (`success` | `error_during_execution`), `is_error`, `result`, `session_id`, `total_cost_usd` (estimado en el cliente), `usage`, `num_turns`, `duration_ms`, `permission_denials`.
- **SIGINT:** el proceso emite un `result` con `subtype: error_during_execution`, `is_error: true`, y **sale con código 0**. El adaptador debe basarse en el evento `result` y no solo en el código de salida.
- Sin stdin redirigido, avisa `no stdin data received in 3s`. El adaptador debe cerrar stdin o enviar el prompt por stdin.
- `claude auth status` (JSON por defecto) devuelve `loggedIn`, `authMethod` (`claude.ai` | `none` | …), `email`, `orgId`, `orgName` y `configDirectory`. **Sirve para verificar la identidad del perfil** (AC-017).
- Con `CLAUDE_CONFIG_DIR` apuntando a un directorio vacío, `loggedIn: false`. En Linux el aislamiento por directorio funciona.

**Codex** (`codex-exec-ok.jsonl`):

- Secuencia: `thread.started{thread_id}` → `turn.started` → `item.completed{item: agent_message}` → `turn.completed{usage}`.
- `usage` solo informa tokens (`input_tokens`, `cached_input_tokens`, `output_tokens`, `reasoning_output_tokens`); **no hay coste ni porcentaje de límite** en los eventos.
- Sin stdin redirigido muestra `Reading additional input from stdin...`; hay que cerrar stdin.
- `codex login status` responde en texto (`Logged in using ChatGPT` / `Not logged in`), sin identificar la cuenta.
- **`CODEX_HOME` debe existir**: si no, falla con `CODEX_HOME points to … but that path does not exist`. Además, Codex se niega a crear binarios auxiliares bajo `/tmp`, así que los directorios de perfil deben estar fuera de `/tmp`.

## Conclusión

Hipótesis confirmada, con confianza alta para Linux. Ambos CLIs se pueden controlar como procesos hijos con JSONL. Claude informa además el uso de sus ventanas de límite, y Codex solo tokens.

## Impacto

- [Contrato de adaptador](../../docs/design/contrato-adaptador.md): estado final basado en el evento `result`/`turn.*`, cierre de stdin, evento normalizado `usage` y evento `limits` (solo Claude).
- `ao profile check`: Claude usa `claude auth status`; Codex usa `codex login status`, que solo indica si hay sesión (la cuenta concreta no es verificable → advertencia visible).
- El adaptador de Codex crea el directorio de `CODEX_HOME` del perfil si no existe.
- Pendiente para Windows/WSL: repetir en el equipo de validación (spike 001).
