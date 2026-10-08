# Spikes de viabilidad

Pruebas acotadas que responden una pregunta técnica antes del diseño. **El código aquí es desechable** y no se importa desde el producto.

Cada spike vive en `spikes/NNN-nombre/` con un `README.md` basado en [`_plantilla/README.md`](_plantilla/README.md) (o use `/nuevo-spike`).

| Nº | Pregunta | Estado | Resultado |
|---|---|---|---|
| [001](001-auth-aislada/) | ¿Se aíslan varias cuentas por directorio de configuración en Linux, Windows y WSL? | In Progress | Linux parcial ✔; Windows/WSL pendiente |
| [006](006-paridad-go/) | ¿Reproduce Go exactamente la cadena de auditoría, el hashing de Criterio y la lectura de YAML de la versión TS? | Done (Linux) | Sí: 0 discrepancias en 44.243 casos dorados; `.cmd` de Windows pendiente para G2 |
| [004](004-ui-web/) | ¿Web local en vivo sin dependencias y con controles de seguridad? | Done | Sí: `node:http` + SSE + JS nativo; 9/9 controles verificados |
| [003](003-rendimiento/) | ¿Qué lenguaje hace a Hilvan más ligero y rápido? (Node, Go, Python, Java) | Done | Go: ~100× más rápido al arrancar, 4–5× menos memoria y binario sin runtime → Q-013 |
| [002](002-invocacion-headless/) | ¿Se pueden controlar `claude -p` y `codex exec` como procesos con JSONL, consumo y cancelación? | Done | Sí; Claude informa también el uso de sus ventanas de límite |

Candidatos (de CONTEXTO.md, próximos pasos 3):

1. Autenticación aislada de dos cuentas de la misma aplicación en Windows nativo y en WSL (AC-015, AC-017, NFR-008).
2. Invocación no interactiva de un CLI de agente: argumentos, directorio de trabajo, streaming, cancelación y código de salida (FR-014, Q-009).
3. Orquestador en Windows controlando WSL frente a runner por entorno (Q-010).
4. Aislamiento de MCPs/herramientas por perfil (FR-006, AC-006).
5. Qué datos de consumo y límites expone cada proveedor (FR-010, NFR-007).

**Seguridad:** nunca versionar credenciales ni directorios de autenticación. Use variables de entorno y documente el método de aislamiento, no los valores.
