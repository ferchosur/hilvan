# Security Policy / Política de seguridad

## Reporting a vulnerability

Please **do not open a public issue**. Report it privately through GitHub's *Report a vulnerability* (Security Advisories) on this repository, or by email to the maintainer listed in the repository profile. Include the affected version, the steps to reproduce and the expected impact.

- We acknowledge reports within 5 working days.
- We aim to release a fix for high and critical issues within 30 days, and to credit reporters who wish to be named.

## Reportar una vulnerabilidad

**No abra un issue público.** Repórtela en privado mediante *Report a vulnerability* (Security Advisories) del repositorio, o por correo al mantenedor indicado en el perfil del repositorio. Incluya la versión, los pasos para reproducirla y el impacto esperado.

## Scope / Alcance

- **En el alcance:** el CLI `hilvan` y su manejo de configuración, procesos, archivos de estado, auditoría y el motor Criterio.
- **Fuera del alcance:** vulnerabilidades de Claude Code, Codex u otros CLIs de agentes. Repórtelas a sus fabricantes.

## Design / Diseño

Hilvan never reads, stores or forwards provider credentials. It launches the official, unmodified agent CLIs with a per-profile configuration directory.

See [docs/security/analisis-de-riesgos.md](docs/security/analisis-de-riesgos.md) and [docs/security/requisitos.md](docs/security/requisitos.md).
