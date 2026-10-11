# Hilvan

**Hilvan** une agentes de IA y cuentas (Claude Code, Codex…) en un mismo flujo de trabajo: cada cuenta es una identidad aislada, el contexto pasa de un agente a otro en paquetes revisables, y tú decides qué se envía y a quién.

> El nombre viene de *hilvanar*: unir piezas con puntadas para que el trabajo continúe. Comando: `hilvan` (alias `hv`).

**Estado:** MVP completo (M0–M5), CI en Linux y Windows; 23 de 25 criterios de aceptación verificados, AC-015 pendiente de verificación en el equipo de validación y AC-009 diferido (ver [verificación](docs/plans/verificacion-mvp.md)). Especificación 001 v1.4. Licencia [Apache-2.0](LICENSE). English: [README.en.md](README.en.md).

## Empezar aquí

1. [CONTEXTO.md](CONTEXTO.md): estado actual, decisiones y próximos pasos (memoria del proyecto).
2. [specs/001-product-requirements.md](specs/001-product-requirements.md): requisitos de producto (fuente de verdad).
3. [docs/glosario.md](docs/glosario.md): términos del dominio.
4. [CONTRIBUTING.md](CONTRIBUTING.md): flujo de trabajo, git y convenciones.
5. [docs/design/arquitectura.md](docs/design/arquitectura.md) y [docs/plans/001-mvp.md](docs/plans/001-mvp.md): cómo se construye y en qué orden.

Si trabaja con un agente de IA, las reglas están en [AGENTS.md](AGENTS.md) (Claude Code las carga mediante [CLAUDE.md](CLAUDE.md)).

## Uso rápido (en desarrollo)

```bash
# Requiere Go (la versión de go.mod) y git. Genera un único binario, sin runtime.
go build -trimpath -ldflags "-s -w" -o ~/.local/bin/hilvan ./cmd/hilvan
ln -sf ~/.local/bin/hilvan ~/.local/bin/hv   # alias corto (en Windows: copie hilvan.exe como hv.exe)
hilvan doctor                           # comprueba git, Claude Code, Codex, WSL y los perfiles

# Un perfil por cuenta y entorno
hilvan profile add --id claude-personal --app claude-code --accepts public,personal --use-default-login
hilvan profile add --id codex-personal  --app codex       --accepts public,personal --use-default-login
hilvan profile add --id claude-work     --app claude-code --accepts corporate,confidential --tags corporate
hilvan profile login claude-work        # login oficial en ~/.hilvan-profiles/claude-work
hilvan profile check                    # verifica ejecutable, carpeta y cuenta activa de cada perfil
```

Hilvan nunca lee ni guarda credenciales: cada perfil usa su propia carpeta de configuración del CLI oficial.

## Método de trabajo: SDD

Usaremos desarrollo guiado por especificaciones (Spec-Driven Development):

1. **Requisitos** (`specs/`): problema, usuarios, alcance, reglas de negocio y criterios de aceptación.
2. **Diseño** (`docs/design/`, `docs/adr/`): arquitectura e interfaces que satisfacen los requisitos aprobados.
3. **Plan** (`docs/plans/`): unidades implementables y verificables.
4. **Implementación** (`cmd/`, `internal/`): desarrollo en Go contra la especificación aprobada.
5. **Verificación**: comprobar cada criterio de aceptación y registrar resultados y desviaciones.

Una especificación en estado `Draft` expresa hipótesis y decisiones pendientes. No autoriza por sí sola a implementar. Antes de implementar una capacidad, sus requisitos y criterios de aceptación deben estar acordados. Los **spikes** (`spikes/`) reducen riesgo técnico antes del diseño y no son código de producto.

```mermaid
flowchart LR
    R[Requisitos<br/>specs/] -->|Approved| D[Diseño<br/>docs/design + ADR]
    S[Spikes<br/>spikes/] -.evidencia.-> D
    D --> P[Plan<br/>docs/plans/]
    P --> I[Implementación<br/>cmd/ + internal/]
    I --> V[Verificación<br/>AC-xxx]
    V -.desviaciones.-> R
```

## Estructura

```
hilvan/
├── AGENTS.md            Reglas para agentes de IA (compartidas)
├── CLAUDE.md            Entrada de Claude Code (importa AGENTS.md y CONTEXTO.md)
├── CONTEXTO.md          Memoria viva: estado, decisiones, próximos pasos
├── CHANGELOG.md         Cambios notables
├── CONTRIBUTING.md      Flujo de trabajo y convenciones
├── specs/               Especificaciones SDD numeradas + plantilla
├── docs/
│   ├── glosario.md      Terminología del dominio
│   ├── adr/             Decisiones de arquitectura (ADR)
│   ├── design/          Diseño técnico
│   ├── plans/           Planes de implementación
│   ├── research/        Investigación y benchmarking
│   └── sessions/        Bitácora de sesiones de trabajo
├── spikes/              Pruebas de viabilidad desechables
├── cmd/hilvan/          Punto de entrada del binario `hilvan`
├── internal/            Código Go por paquetes, con sus tests y `testdata/`
├── scripts/check/       Comprobación antes de cada commit (`go run ./scripts/check`)
└── .claude/             Configuración y skills compartidos de Claude Code
```

## Documentos

| Documento | Estado |
|---|---|
| [001 — Requisitos de producto](specs/001-product-requirements.md) | Approved v1.4 |
| [Estado del proyecto y oportunidades](docs/research/estado-y-oportunidades.md) | Informe 2026-10-04 |
| [ADR-0001 a 0014](docs/adr/README.md) | Accepted (0002 sustituido por 0014; 0012 Proposed) |
| [Diseño del MVP](docs/design/README.md) | Accepted |
| [Plan 001 — MVP](docs/plans/001-mvp.md) | Accepted · completo |
| [Plan 002 — Migración a Go](docs/plans/002-migracion-go.md) | Accepted · completo |
| [Verificación del MVP](docs/plans/verificacion-mvp.md) | 23/25 AC |
| [Investigación de foros y herramientas](docs/research/2026-10-06-foros-y-herramientas.md) | 2026-10-06 |
