# Spike 006 — Riesgos de paridad de la migración a Go

- **Estado:** Done (Linux); Windows `.cmd` pendiente para G2
- **Fecha inicio / fin:** 2026-10-07 / 2026-10-07
- **Límite de tiempo:** 2 h
- **Relacionado:** Q-013, D-026, ADR-0014, [plan 002](../../docs/plans/002-migracion-go.md) (G0), SR-004, SR-017, SR-018, FR-020

## Pregunta

¿Puede Go reproducir exactamente, sin depender de Node, las tres piezas donde una diferencia rompería datos ya guardados?

1. La serialización canónica de la cadena de auditoría (`canonical()` en `src/store/audit.ts`), que depende de `JSON.stringify`.
2. El *feature hashing* FNV-1a sobre unidades UTF-16 y la tokenización de Criterio (`src/criterio/features.ts`), de los que dependen los pesos aprendidos.
3. La lectura de los YAML escritos por la librería `yaml` de npm (`profiles.yaml`, `policy.yaml`).

## Hipótesis

Sí, con un serializador propio que imite `Number::toString` de ECMAScript, `utf16.Encode` para el hash, `golang.org/x/text/unicode/norm` para NFD y `go.yaml.in/yaml/v3` con structs tipados.

## Entorno

Linux x86-64, Go 1.27.1, Node 22.22.1, `golang.org/x/text` v0.42.0, `go.yaml.in/yaml/v3` v3.0.5, consultado el 2026-10-07.

## Procedimiento

> Desde el 2026-10-08 el código TypeScript ya no está en `main` (plan 002, G5). Para regenerar los casos, use el commit `95e1d04`.

```bash
npx tsx spikes/006-paridad-go/gen.ts       # casos dorados con el código TS real
npx tsx spikes/006-paridad-go/gen-yaml.ts  # YAML escrito por la librería de npm
cd spikes/006-paridad-go && go test -v .
```

`gen.ts` importa las funciones reales de `src/` y genera con una semilla fija:

- 40.009 números: bordes como `-0`, `1e21`, `5e-324` o `2^53`, magnitudes de 1e-40 a 1e40 y patrones de bits aleatorios;
- 2.215 valores canónicos: escapes, controles C0, `<&>`, U+2028, acentos, CJK, emoji, claves desordenadas y anidamiento;
- 2.018 textos para Criterio, con casos como `İ`, `ǅ`, `ﬁ`, el signo Kelvin y `ß`.

## Resultados

| Prueba | Casos | Discrepancias |
|---|---:|---:|
| `JSNumber` (formato de números de ECMAScript) | 40.009 | **0** |
| `Canonical` (orden de claves por unidades UTF-16, escapes de `JSON.stringify`) | 2.215 | **0** |
| `FNV1a` + `Tokenize` | 2.018 | **0** |
| YAML de npm con structs tipados | 1 documento con valores conflictivos | **0** |

Notas:

- `encoding/json` de Go **no** sirve para la cadena de hash: escapa `<`, `>`, `&`, U+2028 y U+2029, y formatea los números de otra forma. Hace falta el serializador propio (unas 100 líneas en `parity.go`).
- Las claves se ordenan por **unidades UTF-16**, como `<` en JS, no por bytes UTF-8. Cambia el orden con caracteres fuera del BMP.
- YAML: `2026-10-07` sin comillas se decodifica como `time.Time` si el destino es `any`. Con campos `string` (como serán los structs de Hilvan) se conserva el texto. **Regla para G1: nunca decodificar la configuración a `any`.**
- No se prueban cadenas con sustitutos UTF-16 sueltos. Go no puede representarlos en un `string` válido y Hilvan no los produce.

## Conclusión

Hipótesis confirmada con confianza alta: Go puede verificar las cadenas de auditoría existentes, conservar los modelos aprendidos de Criterio y leer la configuración escrita por la versión TS.

Queda pendiente el lanzamiento de *shims* `.cmd` en Windows (ADR-0014, SR-004), que se resolverá y probará en la CI de Windows en G2.

## Impacto

- G1/G2 del plan 002 pueden partir de `parity.go`, reimplementado en `internal/` y con estos casos dorados como tests.
- Dependencias confirmadas para ADR-0014: `golang.org/x/text` y `go.yaml.in/yaml/v3`.
