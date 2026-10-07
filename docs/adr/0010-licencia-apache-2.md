# ADR-0010 — Licencia Apache-2.0

- **Estado:** Accepted
- **Fecha:** 2026-10-06
- **Decisores:** Fernando Giraldo (aprobación), Claude Code (propuesta como arquitecto)
- **Relacionado:** D-024, Q-012

## Contexto

El proyecto será open source, para que cualquiera lo use y evolucione, y más adelante se evaluará cómo monetizarlo.

## Opciones consideradas

1. **MIT** — máxima simplicidad; sin concesión explícita de patentes.
2. **Apache-2.0** — permisiva, con concesión de patentes y aviso de cambios; habitual en herramientas de desarrollo.
3. **AGPL-3.0** — obliga a publicar cambios en servicios en red; protege frente a SaaS cerrados, pero frena la adopción empresarial.

## Decisión

**Apache-2.0.** Facilita la adopción por empresas (relevante, porque el caso de uso incluye cuentas corporativas) y es compatible con un modelo open-core posterior (Q-012). Las contribuciones se aceptan bajo la misma licencia (inbound = outbound) con *Developer Certificate of Origin* (`Signed-off-by`).

## Consecuencias

- Positivas: adopción sin fricción; protección de patentes para contribuyentes y usuarios.
- Negativas: terceros pueden ofrecer servicios cerrados basados en el proyecto; la monetización deberá basarse en servicios, funciones de equipo o soporte, no en restringir el núcleo.
