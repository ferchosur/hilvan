# Cómo trabajar en este repositorio

Aplica a personas y a agentes de IA. Las reglas específicas para agentes están en [AGENTS.md](AGENTS.md).

## 1. Ciclo de vida de los documentos

### Especificaciones (`specs/`)

| Estado | Significado | Versión |
|---|---|---|
| `Draft` | Hipótesis y preguntas abiertas. No autoriza implementar. | `0.x` |
| `In Review` | Preguntas bloqueantes resueltas; pendiente de aprobación explícita. | `0.x` |
| `Approved` | Baseline. Autoriza diseño e implementación del alcance descrito. | `1.0`, `1.1`… |
| `Superseded` | Reemplazada; el encabezado enlaza a la sucesora. | sin cambio |

- Cambio menor en una spec aprobada (aclaración, error) → `1.x`. Cambio de alcance → nueva versión mayor o nueva spec.
- La aprobación se registra en el encabezado (`Aprobado por`, `Fecha de aprobación`) y en `CHANGELOG.md`, y se etiqueta en git: `spec-001-v1.0`.

### ADR (`docs/adr/`)

`Proposed` → `Accepted` | `Rejected`; más tarde `Deprecated` o `Superseded by ADR-NNNN`. Un ADR aceptado no se reescribe: se reemplaza por otro.

### Spikes (`spikes/`)

Cada spike responde a una pregunta concreta, con límite de tiempo, y termina con un informe de resultados (`README.md` del spike). El código del spike no se promueve a producto: se reescribe con diseño aprobado.

## 2. Memoria del proyecto

| Archivo | Rol | Cómo se edita |
|---|---|---|
| `CONTEXTO.md` | Foto del estado actual | Se **sobrescribe** para reflejar el presente. Fecha de última actualización arriba. |
| `docs/sessions/AAAA-MM-DD-tema.md` | Historia por sesión | **Append-only**: una entrada por sesión, no se edita después. |
| `CHANGELOG.md` | Cambios notables por versión | Sección `[Unreleased]` hasta que se cierre una versión. |
| `docs/adr/` | Por qué se eligió una arquitectura | Inmutable tras aceptar. |
| `specs/` | Qué debe hacer el producto | Versionado documental. |

Regla práctica: si algo es **estado**, va a `CONTEXTO.md`; si es **historia**, a la bitácora; si es **decisión**, a la spec (`D-xxx`) o a un ADR.

## 3. Git

### Ramas

- `main`: siempre coherente; los enlaces funcionan y los estados son correctos.
- Ramas de trabajo: `spec/001-v0.4`, `adr/0002-runner-por-entorno`, `spike/001-auth-aislada`, `feat/…`, `fix/…`.
- Fusionar con `--no-ff` o *squash* según el tamaño; borrar la rama tras fusionar.

### Commits — Conventional Commits

```
tipo(ámbito): descripción breve en imperativo

Cuerpo opcional: qué y por qué (no cómo).

Refs: Q-009, FR-013
```

| Tipo | Uso |
|---|---|
| `spec` | Cambios en especificaciones |
| `adr` | Nuevo ADR o cambio de estado |
| `docs` | Otra documentación (contexto, bitácora, research, glosario) |
| `spike` | Código o informe de pruebas de viabilidad |
| `feat` / `fix` / `refactor` / `test` | Código de producto (tras aprobación) |
| `chore` | Configuración del repositorio y herramientas |

Ámbitos sugeridos: `001`, `contexto`, `glosario`, `adr`, `claude`, `repo`.

Plantilla de mensaje: `git config commit.template .gitmessage` (ya configurado al crear el repositorio).

### Etiquetas

- `spec-NNN-vX.Y`: especificación aprobada.
- `vX.Y.Z`: versiones del producto ([SemVer](https://semver.org/lang/es/)), cuando exista código.

### Certificado de origen (DCO)

Las contribuciones se aceptan bajo Apache-2.0 y con [Developer Certificate of Origin](https://developercertificate.org/): firme cada commit con `git commit -s` (`Signed-off-by:`).

## 4. Código

- Requisitos: Node.js ≥ 22 y git.
- `npm ci` para instalar dependencias. `npm run check` ejecuta lint, typecheck y tests, y debe pasar antes de cada commit.
- `npm run dev -- <args>` ejecuta el CLI desde el código fuente; `npm run build` genera `dist/`.
- Cada cambio de comportamiento va acompañado de tests. Los parsers de los adaptadores se prueban con fixtures reales (`tests/fixtures/`), y los escenarios difíciles con el agente simulado `tests/fixtures/fake-agent.mjs`.
- Siga los [patrones de diseño](docs/design/patrones.md): núcleo puro, adaptadores intercambiables, dependencias inyectadas y carga perezosa en el CLI.
- **Seguridad:** toda funcionalidad nueva añade sus amenazas a `docs/security/analisis-de-riesgos.md` y sus requisitos a `docs/security/requisitos.md` en el mismo PR.
- **Dependencias:** justifique cada dependencia de ejecución nueva con su coste de arranque (ver el spike 003).

## 5. Seguridad

- Nunca versionar credenciales, tokens, cookies, `.env` ni directorios de perfil/autenticación de proveedores.
- En spikes, use rutas de ejemplo y variables de entorno; documente *cómo* se aisló la autenticación, no los valores.
- Datos corporativos: no copiarlos al repositorio, ni siquiera como fixtures.
