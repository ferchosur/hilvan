# Spike 003 — Rendimiento de Hilvan en varios lenguajes

- **Estado:** Done
- **Fecha:** 2026-10-07
- **Límite de tiempo:** 2 h
- **Relacionado:** NFR-013 (ligereza), ADR-0002 (stack), Q-013 (nueva)

## Pregunta

¿Qué lenguaje permite que Hilvan sea más ligero y rápido en equipos modestos, sin sacrificar mantenibilidad: TypeScript/Node (actual), Go, Python o Java?

## Método

Cada lenguaje implementa la **misma porción funcional** de Hilvan usando solo su biblioteca estándar (`node/`, `go/`, `python/`, `java/`):

1. `version`: coste de arranque.
2. `criterio N`: N decisiones del motor Criterio (tokenización, feature hashing FNV-1a, softmax de 7 clases).
3. `jsonl FILE`: parseo y normalización de 200 000 líneas de eventos reales de `claude -p` (fixtures del spike 002, 265 MB).
4. `spawn FILE`: lanzar un proceso hijo y procesar su stdout en streaming (como un adaptador).

`bench.py` genera los datos, ejecuta cada caso (30 arranques y 5 ejecuciones de cada carga), toma la mediana del tiempo y el pico de RSS (`os.wait4`), y comprueba la **paridad**: las tres implementaciones completas devuelven los mismos checksums.

Entorno: Linux x86-64, 4 núcleos, 31 GB RAM, Node 22.22.1, Go 1.17, Python 3.14.4, OpenJDK 25 (`javac` 11). Reproducir con `python3 bench.py <dir_salida>`.

## Resultados

| Implementación | Arranque (ms) | RSS arranque (MB) | Criterio 100k (ms) | JSONL 200k (ms) | RSS JSONL (MB) | Spawn+stream (ms) | Artefacto | Runtime requerido |
|---|---:|---:|---:|---:|---:|---:|---:|---|
| Node.js 22 | 132 | 58 | 637 | 3184 | 121 | 3082 | 3 KB de JS | Node ≥ 22 (~100 MB) |
| **Go 1.17** | **1** | **15** | **276** | 2503 | **15** | 3405 | 1.7 MB (binario estático) | **ninguno** |
| Python 3.14 | 38 | 15 | 4907 | 3621 | 15 | 3957 | 3 KB | Python 3 (~100 MB) |
| Java 25 | 33 | 42 | 905 | 950\* | 285 | 1133\* | 6 KB de `.class` | JRE (~200 MB) |

\* Java no trae parser JSON en el JDK: solo cuenta líneas. Su cifra no es comparable y necesitaría Jackson u otra biblioteca.

**El CLI real de Hilvan (TypeScript)** con sus dependencias (commander, zod, yaml, @clack/prompts, cross-spawn):

| Medición | Antes | Con carga perezosa + compile cache | Además empaquetado con esbuild |
|---|---:|---:|---:|
| `hilvan --version` | 298 ms / 82 MB | 149 ms / 66 MB | 143 ms / 59 MB |
| `hilvan criterio ask` | ~300 ms | 220 ms / 76 MB | 204 ms / 73 MB |
| `hilvan profile list` | ~300 ms | 271 ms / 83 MB | 214 ms / 73 MB |
| `hilvan --help` | 304 ms | 276 ms / 82 MB | 209 ms / 74 MB |
| `node -e ""` (suelo de Node) | 102 ms / 57 MB | — | — |

El coste de importar cada dependencia por primera vez: zod 66 ms (8.2 MB), @clack/prompts 55 ms, yaml 47 ms, commander 41 ms y cross-spawn 33 ms.

## Análisis

- **Arranque y memoria**, que es lo que siente un usuario con un PC modesto. Go es unas **100 veces más rápido al arrancar** (1 ms frente a ~145 ms como mínimo en Node) y usa **4–5 veces menos memoria** (15 MB frente a 60–80 MB). En Node, el propio runtime se lleva ~100 ms y 57 MB antes de ejecutar nada de Hilvan.
- **Cómputo:** Criterio es más rápido en Go (2.8 µs/decisión) que en Node (6.4 µs) y mucho más que en Python (49 µs). Los tres cumplen de sobra el presupuesto de 5 ms.
- **Parseo JSONL:** Go y Node son comparables (el `encoding/json` de Go usa reflexión; con un decodificador especializado sería más rápido). Lanzar procesos y leer su salida en streaming cuesta lo mismo en todos: domina el SO.
- **Distribución:** Go genera **un binario único por SO sin dependencias**: lo ideal para "casi cualquier PC" y para Windows. Node exige instalar un runtime de ~100 MB. Ojo: hoy Claude Code tiene instalador nativo sin Node y Codex es un binario de Rust, así que **Node puede ser una dependencia extra solo para Hilvan**.
- **Contexto:** los procesos de los agentes (`claude`, `codex`) consumen mucha más memoria que Hilvan. El ahorro de Go importa sobre todo en latencia de arranque, porque el CLI se invoca muchas veces, y en facilidad de instalación.
- **Python:** el arranque es aceptable, pero la CPU es lenta y en Windows también exige runtime. **Java:** memoria alta, JRE pesado y sin JSON en el JDK. Ambos quedan descartados.
- **Mantenibilidad:** TypeScript ya tiene ~3 000 líneas con 107 tests. Go tiene un ecosistema maduro para CLIs (cobra), TUI (Bubble Tea) y web embebida (`net/http` + `embed`). La migración ahora es barata; después de M2–M5 sería cara.

## Conclusión

Para el objetivo explícito del usuario (ligereza y funcionar en casi cualquier PC), **Go es la mejor opción técnica**. Con confianza alta en arranque, memoria y distribución, y media en productividad de desarrollo.

Si se mantiene TypeScript, las optimizaciones aplicadas (carga perezosa por subcomando y compile cache) ya reducen el arranque a la mitad. Empaquetar con esbuild y sustituir zod por validadores propios lo acercaría al suelo de Node (~110–140 ms y ~60 MB), pero no puede bajar de ahí.

## Impacto

- Se registra **Q-013 — ¿Migrar Hilvan a Go antes de M2?** para el usuario. Recomendación: **sí**, con el código TypeScript actual como implementación de referencia y sus tests como especificación ejecutable.
- Requisito si se migra: instalar Go ≥ 1.23 (aquí hay 1.17, demasiado antiguo para genéricos modernos, `slog` y `min`/`max`). Este spike no lo instala.
- Aplicado ya al código TypeScript: carga perezosa por grupo de comandos y `module.enableCompileCache()` (patrón de rendimiento, ver `docs/design/patrones.md`).
