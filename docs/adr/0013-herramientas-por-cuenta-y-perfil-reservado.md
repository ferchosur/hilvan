# ADR-0013 — Herramientas de la cuenta, perfil reservado e inferencia de herramientas

- **Estado:** Accepted
- **Fecha:** 2026-10-07
- **Decisores:** Fernando Giraldo (requisito: "todo excepto lo que necesite las herramientas corporativas lo hace la cuenta personal"), Claude Code (diseño)
- **Relacionado:** FR-006, FR-011, FR-012, FR-021, NFR-011, SR-011, SR-012, ADR-0006, ADR-0011
- **Modifica:** ADR-0004 y el contrato de adaptador (argumentos de Claude Code)

## Contexto

La cuenta corporativa del usuario tiene MCP y conectores internos que la cuenta personal no tiene. El usuario quiere que la corporativa se use **solo** cuando una tarea necesita esas herramientas.

Problemas del diseño anterior:

1. **`--strict-mcp-config` apagaba los MCP propios de la cuenta**, incluidos los conectores gestionados por la organización. Eso rompe justo el caso corporativo.
2. **Prueba empírica (2026-10-07, Claude Code 2.1.292)** con un repositorio que tenía un `.mcp.json` y un hook `SessionStart` hostiles:

   | Lanzamiento | MCP del repositorio | Hook del repositorio | MCP de la cuenta |
   |---|---|---|---|
   | por defecto | se ejecuta | se ejecuta | sí |
   | `--strict-mcp-config` | bloqueado | **se ejecuta** | **no** |
   | `--setting-sources user` | bloqueado | bloqueado | **sí** |

3. La preferencia numérica del ranking (15 %) no garantiza la regla del usuario.
4. Declarar `--requires` a mano es propenso a olvidos.

## Decisión

1. **Claude Code se lanza siempre con `--setting-sources user`.** No se cargan ajustes, hooks ni `.mcp.json` del repositorio, y la cuenta conserva sus propias herramientas. Modo MCP por perfil (`tools.mcp`):
   - `account` (por defecto): MCP de la cuenta y de la organización;
   - `profile`: `--strict-mcp-config --mcp-config <archivo>`;
   - `none`: `--strict-mcp-config`.
2. **`routing: reserved`.** Un perfil reservado solo entra en las recomendaciones si la tarea requiere alguna herramienta que declara en `provides`. Si no, aparece excluido con el motivo `routing`. Si no queda ningún otro perfil elegible, se mantiene como último recurso, marcado así. Usarlo a mano para otra tarea muestra un aviso, pero no se bloquea: la decisión es del usuario.
3. **Criterio infiere las herramientas requeridas** (pregunta `requiredTools`):
   - catálogo formado por las palabras del id, alias predefinidos en español e inglés para herramientas comunes (Jira, Confluence, Slack, Teams, GitHub, GitLab, Outlook, Salesforce, ServiceNow…) y los alias propios del perfil (`--alias jira=ticket,incidencia`);
   - un modelo binario por herramienta que aprende de las confirmaciones (las correcciones cuentan el doble);
   - como cambia qué cuenta recibe los datos, **siempre se confirma**: con una selección múltiple en terminal interactiva o con `hilvan task requires`;
   - `task recommend` avisa si el brief parece necesitar una herramienta no declarada.
4. **Descubrimiento:** `hilvan profile tools <id> [--apply]` usa `claude mcp list` / `codex mcp list --json`, ejecutados desde la carpeta personal (fuera del repositorio). Compara lo encontrado con `provides`. Nunca guarda los destinos (URL o comandos), que pueden contener tokens.

## Consecuencias

- Positivas: la regla del usuario es determinista y explicable. Cierra el riesgo residual T10 (hooks del repositorio) en lugar de solo avisar, así que se retira la confianza por hash de SR-012. La cuenta corporativa conserva sus conectores.
- Negativas: los ajustes legítimos del proyecto (`.claude/settings.json` del repositorio) no se aplican en las ejecuciones de Hilvan. Se documenta y, si hace falta, se podrá añadir una opción explícita por proyecto con confirmación.
- Pendiente: verificar en el equipo de validación que los MCP gestionados por la organización aparecen con `--setting-sources user` (spike 001).
