# ADR-0011 — Criterio: motor de decisión local tipado

- **Estado:** Accepted
- **Fecha:** 2026-10-07
- **Decisores:** Fernando Giraldo (dirección: "todo dentro de Hilvan, liviano, sin terceros"), Claude Code (diseño)
- **Relacionado:** FR-012, FR-020, NFR-013, ADR-0009, [investigación JEV](https://www.eigent.ai/es/blog/typesafe-ai-jev-system-one-models)

## Contexto

Hilvan toma muchas decisiones pequeñas y acotadas: el tipo de tarea, su complejidad, si conviene endurecer la clasificación, el riesgo de un diff, si el resultado cumple el brief y qué perfil elegible recomendar. JEV (TypeSafe) demuestra el valor de responder a "preguntas tipadas" con probabilidades calibradas y umbrales de escalamiento, pero es un servicio externo. El usuario exige que Hilvan funcione en PCs de bajos recursos, **sin modelos locales, sin terceros, sin latencia de red y sin costes ocultos**.

## Opciones consideradas

1. **API de JEV**: envía datos a un tercero, con coste y latencia de red y cifras sin verificar.
2. **LLM local (Ollama) o un perfil existente**: pesado para PCs modestos o consume cuota.
3. **Motor propio y local**: reglas iniciales, aprendizaje en línea con los resultados del propio usuario, umbrales y explicaciones.

## Decisión

**Opción 3: "Criterio"** (`src/criterio/`), TypeScript puro y sin dependencias:

- **Interfaz tipada al estilo JEV:** preguntas `choice`, `score` (2–10) y `yesno`. Cada `Decision` incluye `value`, `p` (confianza), `why` (señales con su peso) y `source` (`rules`, `model` o `blend`).
- **Señales locales:** el texto del brief (tokens es/en normalizados y raíz por prefijo), métricas del diff y del repositorio, y el estado del perfil (uso de las ventanas de cuota que reporta el CLI). Se aplica *feature hashing*: el modelo guarda pesos por hash, **nunca texto**.
- **Capas:** reglas con priors (útiles desde el primer día), regresión logística multinomial en línea (SGD + L2) y una mezcla `α = n/(n+k)` que da más peso al modelo a medida que acumula ejemplos.
- **Perfiles:** Beta-Bernoulli por (perfil, tipo de tarea) con muestreo de Thompson y semilla inyectable. El margen de cuota **no diferencia entre perfiles de la misma aplicación** (FR-012).
- **Umbrales:** `auto` (confianza alta y acción inocua), `confirm` y `escalate` (confianza baja o riesgo alto). Enviar datos o cambiar de perfil **siempre** es `confirm`.
- **Calibración:** puntuación de Brier y curva de fiabilidad sobre las últimas decisiones con resultado conocido.
- **Persistencia:** `criterio/model.json` en el directorio de configuración del usuario (0600, escritura atómica, esquema versionado).

## Consecuencias

- Positivas: menos de 5 ms por decisión, cero coste, cero red, explicable y adaptado a cada usuario.
- Negativas: comprensión superficial del texto (léxico, no semántica) y arranque en frío basado en reglas. Se mitiga con explicaciones visibles y confirmación humana.
- Riesgo: envenenamiento del modelo por etiquetas erróneas. Mitigación: los umbrales nunca permiten acciones sensibles automáticas, existe `hilvan criterio reset` y la tasa de aprendizaje está acotada (ver análisis de riesgos).
