# Contrato de adaptador

- **Estado:** Accepted (baseline del MVP)
- **Fecha:** 2026-10-06
- **Relacionado:** ADR-0004, ADR-0005, FR-014, FR-016, FR-017, [spike 002](../../spikes/002-invocacion-headless/)

## 1. Interfaz

```ts
interface AgentAdapter {
  readonly app: 'claude-code' | 'codex';
  /** Comprueba ejecutable e identidad del perfil sin leer credenciales. */
  checkAuth(profile: Profile): Promise<AuthStatus>;
  /** Comando interactivo para que la persona haga login con el flujo oficial. */
  loginCommand(profile: Profile): Command;
  /** Inicia una ejecución; devuelve un handle con flujo de eventos. */
  start(req: RunRequest): RunHandle;
}

interface RunRequest {
  profile: Profile;
  cwd: string;              // worktree o raíz del repo, en el entorno destino
  prompt: string;           // brief + paquete aprobado
  mode: 'write' | 'read';
  resumeSessionId?: string;
  outputSchema?: object;    // salida estructurada (handoff)
}

interface RunHandle {
  events: AsyncIterable<AgentEvent>;
  cancel(): Promise<void>;  // SIGINT, SIGTERM tras 10 s
  done: Promise<RunOutcome>;
}

type AuthStatus = { loggedIn: boolean; method?: string; account?: string; verifiable: boolean; detail?: string };
```

`Command = { executable, args: string[], env: Record<string,string>, cwd }`. El **lanzador** lo transforma en la invocación final (ADR-0005). Siempre se usa `spawn` con `shell: false` y stdin cerrado o con el prompt.

## 2. Eventos normalizados (`AgentEvent`)

| `type` | Campos | Origen Claude | Origen Codex |
|---|---|---|---|
| `session` | `sessionId`, `model?`, `tools?`, `mcpServers?` | `system/init` | `thread.started` |
| `message` | `role`, `text` | `assistant` (texto) | `item.completed` `agent_message` |
| `tool` | `name`, `status`, `summary` | `assistant` `tool_use` / `user` `tool_result` | `item.*` `command_execution`, `file_change`, `mcp_tool_call` |
| `retry` | `attempt`, `error` | `system/api_retry` | `error` (si es recuperable) |
| `limits` | `status`, `windows: {name, utilization, resetsAt}[]` | `rate_limit_event` | — |
| `usage` | `costUsd?`, `inputTokens`, `outputTokens`, `estimated: true` | `result.total_cost_usd`, `result.usage` | `turn.completed.usage` |
| `result` | `ok`, `text`, `sessionId`, `stopReason?`, `permissionDenials?` | `result` | `turn.completed` / `turn.failed` |
| `raw` | `line` | línea no reconocida | línea no reconocida |

Las líneas no reconocidas se conservan como `raw` y no rompen la ejecución (R-008).

## 3. Resultado (`RunOutcome`)

| Condición | `state` | `blockedReason` |
|---|---|---|
| Evento `result` con `ok` | `completed` | — |
| `result.is_error` tras una cancelación solicitada | `cancelled` | — |
| Último `retry`/`result` con `rate_limit` o `limits.status != allowed` | `blocked` | `rate_limit` |
| `authentication_failed`, `oauth_org_not_allowed`, `account_on_hold` | `blocked` | `auth` |
| `billing_error` | `blocked` | `billing` |
| Otro `result.is_error` o `turn.failed` | `failed` | — |
| Proceso terminado **sin** evento final | `uncertain` | — |

El estado final se decide por los **eventos**, no por el código de salida: Claude sale con 0 tras SIGINT (spike 002).

## 4. Construcción de argumentos

**Claude Code**

```
claude -p --output-format stream-json --verbose
       --strict-mcp-config [--mcp-config <profile.tools.mcpConfig>]
       --permission-prompts none
       --permission-mode <acceptEdits si write | dontAsk si read>
       [--model <m>] [--resume <id>] [--json-schema <json>]
env: CLAUDE_CONFIG_DIR=<configDir>, o se elimina si configDir es null (login por defecto)
     se eliminan ANTHROPIC_API_KEY/ANTHROPIC_AUTH_TOKEN heredadas en perfiles de suscripción
     (+ ANTHROPIC_API_KEY desde apiKeyEnv si auth=api-key)
stdin: prompt
```

Login: `claude auth login`. Comprobación de autenticación: `claude auth status --json` → `loggedIn`, `authMethod`, cuenta = `email` (verificable).

**Codex**

```
codex exec --json --cd <cwd>
      --sandbox <workspace-write si write | read-only si read>
      [--model <m>] [--output-schema <archivo>] -
codex exec resume <id> --json --cd … -             (reanudación; sin probar aún con la CLI real)
env: CODEX_HOME=<configDir> (el adaptador crea el directorio si falta; nunca bajo /tmp)
     (+ CODEX_API_KEY desde apiKeyEnv si auth=api-key)
stdin: prompt
```

Comprobación de autenticación: `codex login status` (texto). La cuenta concreta **no es verificable** (`verifiable: false`), y la CLI lo muestra como advertencia.

## 5. Pruebas

- Tests unitarios del parser con los fixtures reales de `spikes/002-invocacion-headless/fixtures/` (copiados a `tests/fixtures/`).
- Prueba de integración opcional (`HILVAN_E2E=1`) que lanza los CLIs reales con un prompt trivial.
- Un adaptador "fake" (script Node que emite JSONL) para probar `core` sin consumir cuota.
