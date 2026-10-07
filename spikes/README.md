# Spikes de viabilidad

Pruebas acotadas que responden una pregunta técnica antes del diseño. **El código aquí es desechable** y no se importa desde el producto.

Cada spike vive en `spikes/NNN-nombre/` con un `README.md` basado en [`_plantilla/README.md`](_plantilla/README.md) (o use `/nuevo-spike`).

| Nº | Pregunta | Estado | Resultado |
|---|---|---|---|
| — | _Ninguno iniciado_ | | |

Candidatos (de CONTEXTO.md, próximos pasos 3):

1. Autenticación aislada de dos cuentas de la misma aplicación en Windows nativo y en WSL (AC-015, AC-017, NFR-008).
2. Invocación no interactiva de un CLI de agente: argumentos, directorio de trabajo, streaming, cancelación y código de salida (FR-014, Q-009).
3. Orquestador en Windows controlando WSL frente a runner por entorno (Q-010).
4. Aislamiento de MCPs/herramientas por perfil (FR-006, AC-006).
5. Qué datos de consumo y límites expone cada proveedor (FR-010, NFR-007).

**Seguridad:** nunca versionar credenciales ni directorios de autenticación. Use variables de entorno y documente el método de aislamiento, no los valores.
