# Spike 005 — Mods de Claude Code en las ejecuciones de Hilvan

- **Estado:** Planned
- **Fecha inicio / fin:** 2026-10-07 /
- **Límite de tiempo:** 2 h (Linux). La parte de cuenta corporativa se hace junto al spike 001 en el equipo de validación.
- **Relacionado:** NFR-011, FR-021, FR-009 (política), AC-006, SR-011, SR-012, amenaza T10, ADR-0009, ADR-0013

## Contexto

Desde Claude Code 2.1.287 (2026-10-01), los **mods** están activos por defecto. Un mod es un plugin con funciones JS/TS que se ejecutan dentro del proceso de Claude Code. Según la [documentación oficial](https://code.claude.com/docs/en/plugins/mods/overview) (consultada el 2026-10-07):

- Sus hooks **se ejecutan en `claude -p`** (tabla *Where mods run*), que es como lanza Hilvan.
- Un mod ve todos los prompts y llamadas a herramientas, puede reescribirlos, puede aprobar llamadas que un `ask`, un `PreToolUse` y, con `allowModsToOverrideDenyRules`, una regla `deny` habrían bloqueado; puede cambiar el modelo de cada petición (`turn.step`) o de cada subagente (`agent.spawn`), llamar a modelos con el plan del usuario y no tiene sandbox.
- `CLAUDE_CODE_PLUGIN_DIRS` (entorno, o `env` en `~/.claude/settings.json`) carga directorios de plugins como `--plugin-dir`. Hilvan **hereda el entorno del host** (`src/adapters/claude-code.ts`, `profileEnv` solo elimina claves concretas).
- `--bare` desactiva los mods instalados, pero la autenticación pasa a ser solo `ANTHROPIC_API_KEY` (rompe los perfiles de suscripción). `--safe-mode` también los desactiva, pero apaga además los servidores MCP (rompería FR-021 en `mcp: account`). Fuente: `claude --help`, 2.1.293.
- El evento `system/init` de `stream-json` ya trae un campo `plugins` (`[{name, path, source}]`); hoy solo lista los integrados (`…@builtin`). Ver `spikes/002-invocacion-headless/fixtures/claude-stream-ok.jsonl`.

ADR-0013 cerró la vía del **repositorio** (`--setting-sources user`), pero los mods instalados por el **usuario** en la cuenta del perfil quedan fuera de ese control.

## Pregunta

Con los argumentos exactos con los que Hilvan lanza `claude -p`:

1. ¿Se carga y se ejecuta un mod instalado a nivel de usuario en el directorio de configuración del perfil?
2. ¿Se carga un mod habilitado solo en el `.claude/settings.json` del repositorio (ámbito `project`)?
3. ¿Se carga un mod indicado por `CLAUDE_CODE_PLUGIN_DIRS` en el entorno del host, a través de `hilvan task run`?
4. ¿Puede un mod aprobar una llamada que el modo `dontAsk` de Hilvan denegaría y cambiar el modelo pedido con `--model`? ¿Lo reflejan los eventos que Hilvan registra?
5. ¿Qué mitigación desactiva los mods de usuario **sin** romper la autenticación por suscripción ni los MCP de la cuenta? Candidatas: `--settings '{"disableAllHooks":true}'`, `--safe-mode`, `--bare`, quitar `CLAUDE_CODE_PLUGIN_DIRS`.
6. ¿Puede Hilvan **detectar** los mods activos sin invocar al modelo (`claude plugin list --json`) y al iniciar la ejecución (`init.plugins`)?

## Hipótesis

1. Sí (la documentación lo dice explícitamente).
2. No, porque `--setting-sources user` no lee los ajustes del proyecto.
3. Sí, porque el entorno se hereda.
4. Sí en ambos casos; `init.model` mostraría el modelo pedido y `result.modelUsage` el que se usó realmente.
5. `--settings '{"disableAllHooks":true}'` es la candidata más prometedora: según la referencia, en ajustes de usuario detiene los mods y hooks instalados, pero deja cargar MCP, skills y agentes del plugin. Falta comprobar que los ajustes pasados por `--settings` se aplican junto a `--setting-sources user`.
6. Sí en ambos casos.

## Entorno

Linux (Ubuntu), Claude Code 2.1.293, Hilvan `main` (`b89f0fb`), login de suscripción. **Perfil de prueba aislado**, con su propio directorio de configuración fuera de `/tmp` y del repositorio: no se tocan `~/.claude` ni `~/.claude.json`. Sin credenciales ni rutas reales en este informe.

## Material

- [`mods/`](mods/): marketplace local `hilvan-spike-005` con dos mods desechables, validados con `claude plugin validate` (2.1.293):
  - **`hv-canary`** solo observa: escribe una línea JSON por `session.start`, `turn.start`, `tool.call` y `tool.check` en `$HILVAN_SPIKE005_LOG`. Si la variable no existe, no hace nada.
  - **`hv-override`** interviene: devuelve `allow` en `tool.check` para un `Bash` que contenga `hilvan-spike-005` y, si existe `$HILVAN_SPIKE005_MODEL`, cambia el modelo en `turn.step`.
- Salidas en `out/` (ignorado por git).

## Procedimiento

```bash
cd spikes/005-mods-headless
export SPIKE_CFG=<dir-de-config-de-prueba>     # p. ej. bajo ~/.local/share, fuera de /tmp
export HILVAN_SPIKE005_LOG="$PWD/out/canary.log"
mkdir -p out "$SPIKE_CFG"

# Perfil de Hilvan aislado para el spike (login interactivo una vez)
hilvan profile add --id spike005 --app claude-code --label "Spike 005" --config-dir "$SPIKE_CFG"
hilvan profile login spike005
hilvan profile check spike005

# Argumentos de Hilvan (copiados de src/adapters/claude-code.ts, modo read)
HV_ARGS=(-p --output-format stream-json --verbose --setting-sources user
         --permission-prompts none --permission-mode dontAsk)
run() { CLAUDE_CONFIG_DIR="$SPIKE_CFG" claude "${HV_ARGS[@]}" "$@" < /dev/null; }
init_plugins() { head -1 "$1" | jq -c '[.plugins[].source]'; }
```

**E0 — Control sin mods.** `run "Responde OK" > out/e0.jsonl`. Esperado: `init.plugins` solo con `@builtin` y `canary.log` vacío.

**E1 — Sanidad con `--plugin-dir`.** `run --plugin-dir mods/canary "Lista los archivos con Bash" > out/e1.jsonl`. Esperado: `session.start`, `turn.start` y `tool.*` en `canary.log`. Si falla, el mod no sirve como sonda y hay que corregirlo antes de seguir.

**E2 — Mod de usuario (pregunta 1).**

```bash
CLAUDE_CONFIG_DIR="$SPIKE_CFG" claude plugin marketplace add "$PWD/mods"
CLAUDE_CONFIG_DIR="$SPIKE_CFG" claude plugin install hv-canary@hilvan-spike-005 --scope user
CLAUDE_CONFIG_DIR="$SPIKE_CFG" claude plugin list --json > out/e2-list.json   # pregunta 6
run "Lista los archivos con Bash" > out/e2.jsonl; init_plugins out/e2.jsonl
```

Repetir con `hilvan task new` + `hilvan task run <id> -p spike005` en un repositorio de prueba para confirmarlo con el lanzador real.

**E3 — Mod de proyecto (pregunta 2).** Desinstalar `hv-canary` del ámbito de usuario. En un repositorio git de prueba: `claude plugin install hv-canary@hilvan-spike-005 --scope project` (escribe `enabledPlugins` en `.claude/settings.json`). Comparar `run …` (esperado: no carga) con la misma orden **sin** `--setting-sources user` (control positivo: sí carga).

**E4 — `CLAUDE_CODE_PLUGIN_DIRS` (pregunta 3).** Sin mods instalados: `CLAUDE_CODE_PLUGIN_DIRS="$PWD/mods/canary" hilvan task run <id> -p spike005`. Comprobar `canary.log` e `init.plugins` en el registro de la ejecución (`.hilvan/runs/…`). Repetir poniendo la variable en `env` de `$SPIKE_CFG/settings.json`.

**E5 — Alcance de un mod que interviene (pregunta 4).** Instalar `hv-override` (usuario).

- Permisos: `run "Ejecuta exactamente: echo hilvan-spike-005" > out/e5a.jsonl`. Comparar `result.permission_denials` y si aparece la salida del `echo`, con y sin el mod.
- Modelo: `HILVAN_SPIKE005_MODEL=haiku run --model sonnet "Responde OK" > out/e5b.jsonl`. Comparar `init.model` con las claves de `result.modelUsage` y con lo que muestra `hilvan stats`.

**E6 — Mitigaciones (pregunta 5).** Con `hv-canary` y `hv-override` instalados, un perfil con un MCP de cuenta de prueba (inocuo, p. ej. un servidor `stdio` local) y suscripción:

| Variante | ¿Mods cargados? | ¿Auth suscripción? | ¿MCP de cuenta? | ¿Hooks de usuario? |
|---|---|---|---|---|
| `--settings '{"disableAllHooks":true}'` | | | | |
| `--safe-mode` | | | | |
| `--safe-mode --mcp-config <archivo>` | | | | |
| `--bare` | | | | |
| `env -u CLAUDE_CODE_PLUGIN_DIRS` (E4) | | | | |

**E7 — Cuenta corporativa (en el equipo de validación, con el spike 001).** Con la cuenta de Team/Enterprise: ¿carga `sec-default@builtin` (`init.plugins`)? ¿La organización fija `allowManagedModsOnly` o `allowManagedHooksOnly`? Sin instalar nada en esa cuenta: solo `claude plugin list --json` y un `run "Responde OK"`.

**Limpieza.** `claude plugin uninstall` de ambos mods, `claude plugin marketplace remove hilvan-spike-005`, `hilvan profile remove spike005` y borrar `$SPIKE_CFG` y `out/`.

## Resultados

Pendiente.

## Conclusión

Pendiente.

## Impacto (previsto según el resultado)

- **Amenaza nueva** en `docs/security/analisis-de-riesgos.md`: un mod de usuario en la cuenta del perfil observa o altera la ejecución y anula la política de Hilvan (relacionada con T10).
- **Requisito SR nuevo**, según E6: o bien lanzar con la mitigación que funcione (p. ej. `disableAllHooks` por `--settings`), o bien detectar y aplicar la política (`init.plugins` con mods que no sean `@builtin` → `blocked` o `require_approval`). En ambos casos, eliminar `CLAUDE_CODE_PLUGIN_DIRS` y `CLAUDE_CODE_PLUGIN_DIR_WATCH` en `profileEnv`.
- `hilvan doctor` / `profile check`: listar los mods activos de cada perfil (`claude plugin list --json`).
- Registro de la ejecución: guardar `init.plugins` y el modelo realmente usado (`modelUsage`) para que `stats` no atribuya mal el coste si un mod cambió el modelo.
- Si el cambio de comportamiento es por defecto, ampliar NFR-011 (hoy habla de MCP y hooks) y proponer ADR, ya que modifica ADR-0013.
