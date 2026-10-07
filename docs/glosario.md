# Glosario

Términos del dominio con el significado que tienen en este proyecto. Si un documento usa un término de forma distinta, corrija el documento o actualice aquí y registre el cambio.

| Término | Definición | Referencia |
|---|---|---|
| **Aplicación** | Herramienta o servicio de IA que el usuario habilita (p. ej. Claude Code, Codex, Perplexity). El producto no presupone una lista fija. | D-007, FR-001 |
| **Perfil** | Combinación de una cuenta concreta de una aplicación con sus credenciales, capacidades, conectores, permisos **y** su destino de ejecución. Dos cuentas del mismo proveedor son perfiles distintos. | D-001, D-011 |
| **Destino de ejecución** | Entorno concreto donde corre el agente de un perfil: tipo de entorno (Windows nativo, distribución WSL…), ejecutable/adaptador, directorio de trabajo y modo de invocación. | FR-013 |
| **Adaptador** | Componente que entrega instrucciones y contexto al destino del perfil y devuelve estado, salida, errores y código de finalización. | D-012, FR-014 |
| **Lanzador** | Función que transforma un comando en la invocación concreta para un destino: `native`, `wsl` (`wsl.exe -d <distro>`), `windows` (desde WSL) y, más adelante, `ssh`. | ADR-0005 |
| **Runner** | Proceso residente dentro de un entorno que ejecuta el adaptador. **No existe en el MVP** (D-017); queda como opción futura. | ADR-0005 |
| **Elegibilidad** | Resultado de comprobar si un perfil puede recibir la clasificación de datos de la tarea y tiene las herramientas y capacidades requeridas. Se evalúa **antes** de recomendar. | D-008, FR-011 |
| **Recomendación** | Ordenación explicada de perfiles **elegibles** según capacidad suficiente, disponibilidad, consumo/costo conocido y preferencia. Requiere confirmación. | D-009, FR-012 |
| **Paquete de continuidad** (o de traspaso) | Resumen compacto y revisable con objetivo, estado, hechos, decisiones, supuestos, archivos/diffs, resultados y siguiente paso. No es el historial completo. | FR-003 |
| **Cruce de cuentas** | Envío de contexto de una tarea a un perfil con distinta clasificación (p. ej. corporativa → personal). Requiere vista previa y aprobación. | D-003, FR-004 |
| **Restricción obligatoria** | Política organizacional que excluye perfiles o transferencias y no puede anularse con la aprobación del usuario. | D-010, AC-014 |
| **Clasificación de datos** | Etiqueta de sensibilidad de una tarea; por defecto `public`, `personal`, `corporate`, `confidential`, configurable por proyecto. | D-022 |
| **Conector / herramienta** | Capacidad externa disponible para un perfil (p. ej. un servidor MCP). Se asocia al perfil, no a la aplicación. | FR-006 |
| **Ejecución (run)** | Una invocación de un agente para una tarea, con su sesión, sus eventos, su consumo y su resultado. Una tarea puede tener varias. | modelo de datos |
| **Worktree de tarea** | Copia de trabajo git aislada (`git worktree`) y rama `hilvan/<tarea>` donde trabaja una tarea que escribe código. | ADR-0008 |
| **Bloqueada (`blocked`)** | Ejecución detenida por límite de uso, autenticación o facturación; espera una decisión del usuario. | FR-017 |
| **Incierta (`uncertain`)** | Ejecución cuyo proceso terminó sin resultado final; no se reintenta sola. | FR-017 |
| **Revisión cruzada** | Revisión de solo lectura del resultado de una tarea, hecha por otro perfil (normalmente de otro proveedor). | FR-015 |
| **Política gestionada** | Archivo de política de solo lectura para el usuario cuyas reglas `enforced` no se pueden aprobar por encima. | ADR-0009 |
| **Spike** | Prueba de viabilidad acotada en tiempo que responde una pregunta técnica; su código es desechable. | `spikes/` |
| **Baseline** | Versión aprobada (`Approved`) de una especificación que autoriza diseño e implementación. | ADR-0001 |
