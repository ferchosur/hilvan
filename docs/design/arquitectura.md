# Arquitectura

- **Estado:** Accepted (baseline del MVP; actualizado a Go el 2026-10-08, plan 002)
- **Fecha:** 2026-10-06
- **Relacionado:** [spec 001 v1.4](../../specs/001-product-requirements.md), ADR-0003 a ADR-0014

## 1. Vista general

Un único binario de Go (`hilvan`, ADR-0014) coordina perfiles que viven en distintos entornos. Lanza los CLIs oficiales de los agentes como procesos hijos y guarda todo el estado en archivos.

```mermaid
flowchart TB
    U([Persona]) --> CLI[cli<br/>comandos hilvan]
    CLI --> ORC[core / orchestrator<br/>casos de uso]
    ORC --> POL[policy<br/>elegibilidad y aprobación]
    ORC --> HO[handoff<br/>paquete de continuidad]
    ORC --> GIT[gitops<br/>worktrees, diff, merge]
    ORC --> ST[(store<br/>.hilvan/ + config de usuario)]
    ORC --> AD[adapters<br/>claude-code · codex]
    AD --> LA[launchers<br/>native · wsl · windows · ssh*]
    LA -->|spawn, stdio JSONL| P1[[claude -p<br/>CLAUDE_CONFIG_DIR=perfil A]]
    LA --> P2[[codex exec<br/>CODEX_HOME=perfil B]]
    CFG[config<br/>profiles.yaml, policy.yaml] --> ORC
```

`*` ssh: incremento 2.

## 2. Paquetes (`internal/`)

| Módulo | Responsabilidad | Depende de |
|---|---|---|
| `domain/` | Tipos y validación (mismas reglas que tenían los esquemas zod): Profile, Destination, Task, Run, Policy. Máquina de estados de la tarea. Sin E/S. | — |
| `security/`, `jsfmt/` | Saneado de terminal, secretos, E/S segura y argumentos; formato de números, JSON canónico y redondeo de JavaScript, para que los datos guardados por la versión TS sigan siendo válidos. | — |
| `criterio/` | Motor de decisión local (ADR-0011). | domain, jsfmt |
| `process/` | Ejecución sin shell (con `.cmd` de Windows protegido), divisor JSONL con límites y salida UTF-16 de `wsl.exe`. | security |
| `config/` | Rutas de configuración por sistema operativo; carga y validación de `profiles.yaml` y `policy.yaml` (usuario y gestionada). | domain |
| `store/` | Interfaz `Store` e implementación con archivos: escritura atómica, `events.jsonl` de solo añadido. | domain |
| `launchers/` | Funciones puras `(Destination, Command) → Invocation` (ejecutable, args, env, cwd, `PATH` de WSL) y traducción de rutas Windows ↔ WSL. | domain |
| `adapters/` | `AgentAdapter` por aplicación: construye argumentos, interpreta JSONL a `AgentEvent`, cancela, reanuda y comprueba la autenticación. | domain, launchers |
| `policy/` | Evalúa la elegibilidad (clasificación, herramientas, capacidades, reglas en capas) con motivos; ordena los perfiles elegibles. | domain, config |
| `gitops/` | Worktree por tarea (con rutas relativas, usable desde Windows y WSL), diff, identidad de respaldo, fusión y limpieza. | process |
| `core/` | Casos de uso: crear tarea, asignar, ejecutar (con gancho de preparación), bloquear o marcar como incierta, traspasar (paquete con hechos de git, secretos redactados y hash), revisar, aceptar o descartar. Orquesta y emite eventos de auditoría. | todos los anteriores |
| `cli/` | Comandos con un despachador propio sin dependencias y preguntas por líneas. Solo presentación. | core |

Regla de dependencias: `domain` no importa nada del proyecto. `cli` solo habla con `core`. La futura UI web será otro cliente de `core`.

## 3. Flujos principales

### 3.1 Ejecutar una tarea

```mermaid
sequenceDiagram
    actor U as Persona
    participant C as core
    participant P as policy
    participant G as gitops
    participant A as adapter
    participant S as store
    U->>C: hilvan task run T --profile X
    C->>P: evaluate(T, X)
    P-->>C: eligible / denied(motivos) / require_approval
    alt require_approval
        C->>U: vista previa del paquete (editable)
        U-->>C: aprobar | cancelar
    end
    C->>G: worktree(T) (si escribe)
    C->>S: event run.started
    C->>A: start(invocation, prompt)
    loop eventos JSONL
        A-->>C: AgentEvent (message, tool, usage, limits…)
        C->>S: runs/<run>.jsonl
    end
    A-->>C: completed | failed | blocked | uncertain
    C->>S: event run.finished + estado tarea
    C->>U: resumen, consumo, siguiente acción
```

### 3.2 Traspaso a otro perfil

1. `hilvan handoff T --to Y`: `core` pide al perfil de origen (reanudando su sesión) un resumen con esquema JSON (`--json-schema` / `--output-schema`) y añade hechos de `gitops` (rama, commits, diff stat, archivos).
2. `handoff` detecta secretos y los marca, renderiza el Markdown, y el usuario lo edita y aprueba.
3. Se registra el hash del paquete aprobado y se crea una nueva ejecución en Y con el paquete como contexto inicial.

### 3.3 Revisión y aceptación

`hilvan review T` muestra el diff. `hilvan review T --by Z` lanza una ejecución de solo lectura en Z que adjunta su informe. `hilvan accept T` fusiona la rama y limpia el worktree. `hilvan discard T` limpia sin fusionar.

## 4. Decisiones transversales

- **Errores:** los errores de dominio son tipados (`PolicyDenied`, `ProfileUnavailable`, `AdapterProtocolError`…) y nunca provocan un cambio automático de perfil (FR-010, FR-017).
- **Concurrencia:** una ejecución activa por proyecto en el MVP, protegida con un archivo de bloqueo `.hilvan/lock` que incluye PID. Si el PID ya no existe al arrancar, la ejecución asociada pasa a `uncertain`.
- **Observabilidad:** `events.jsonl` (auditoría de negocio) y `runs/<id>.jsonl` (eventos crudos del agente). Ninguno contiene secretos (ver [amenazas](amenazas.md)).
- **Internacionalización:** los mensajes del CLI están en inglés en el código, con catálogo en español. La documentación de usuario está en español e inglés antes de publicar.

## 5. Evolución prevista

| Incremento | Cambio arquitectónico |
|---|---|
| 2 | `launchers/ssh`, ejecución paralela (N ejecuciones activas), UI web local (`hilvan ui`, servidor HTTP local + SPA) sobre `core`, notificaciones. |
| 3 | Adaptador ACP genérico, perfil de investigación (Perplexity), varios repositorios por proyecto, posible runner en Go y `Store` en SQLite. |
