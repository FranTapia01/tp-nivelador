# Informe

## 1. Protocolo de Comunicación

El protocolo de aplicación opera sobre TCP y resuelve el problema de delimitación de mensajes de longitud variable (*framing*) mediante prefijos de tamaño (*length-prefixed framing*) y códigos de operación (*opcodes*) de 1 byte con convención Big-Endian (Network Byte Order).

### Opcodes Definidos

| Opcode | Hex | Dirección | Descripción |
| :--- | :--- | :--- | :--- |
| `CODE_SEND_BATCH` | `0x01` | Cliente -> Servidor | Envío de un lote de apuestas agrupadas |
| `CODE_GET_WINNERS` | `0x02` | Cliente -> Servidor | Solicitud de ganadores del sorteo |
| `CODE_SEND_WINNERS` | `0x03` | Servidor -> Cliente | Envío consolidado de la lista de ganadores |
| `CODE_LAST_BATCH` | `0x04` | Cliente -> Servidor | Notificación de fin de apuestas de la agencia |
| `CODE_ACK` | `0x05` | Servidor -> Cliente | Confirmación de procesamiento de lote |
| `CODE_EXIT` | `0x00` | Cliente -> Servidor | Cierre explícito de sesión |

### Formato de Mensajes

#### 1. Lote de Apuestas (`0x01 - CODE_SEND_BATCH`)
Para minimizar las llamadas al sistema (*syscalls*) y evitar la fragmentación de paquetes IP, el cliente serializa el lote completo dentro de un único buffer contiguo antes de emitir la transferencia:

* **Estructura del Mensaje:**
  * Opcode: 1 byte (`0x01`).
  * ID de Agencia: 2 bytes (entero sin signo, Big-Endian).
  * Cantidad de apuestas en el lote: 2 bytes (entero sin signo, Big-Endian).
  * Secuencia consecutiva de apuestas serializadas:
    * Largo del registro CSV: 2 bytes (entero sin signo, Big-Endian).
    * Contenido del registro CSV codificado en UTF-8.
* **Control de Flujo:** El servidor procesa el bloque en su totalidad y responde sincrónicamente con un byte `0x05 (ACK)` antes de que el cliente despache el siguiente bloque.

#### 2. Fin de Apuestas (`0x04 - CODE_LAST_BATCH`)
Notifica al servidor que una agencia finalizo el envío de sus registros:

* Opcode: 1 byte (`0x04`).
* ID de Agencia: 2 bytes (entero sin signo, Big-Endian).

#### 3. Consulta de Ganadores (`0x02 - CODE_GET_WINNERS`)
Solicitud emitida por el cliente tras el envío de lotes:

* Opcode: 1 byte (`0x02`).

#### 4. Respuesta de Ganadores (`0x03 - CODE_SEND_WINNERS`)
El servidor empaqueta todos los registros completos de las apuestas ganadoras dentro de un único mensaje continuo:

* Opcode: 1 byte (`0x03`).
* Total de apuestas ganadoras: 2 bytes (entero sin signo, Big-Endian).
* Secuencia consecutiva de registros ganadores:
  * Largo de la línea de la apuesta: 2 bytes (entero sin signo, Big-Endian).
  * Contenido completo de la apuesta serializada.

---

## 2. Concurrencia y Sincronización

### Selección de `threading` y el Impacto del GIL en Python
Se optó por un modelo concurrente basado en hilos (`threading.Thread`) en lugar de múltiples procesos (`multiprocessing`), dado que el cuello de botella del sistema es predominantemente de **I/O Bound** (recepción y transmisión de sockets de red y persistencia en disco) y no de CPU Bound.

* **Comportamiento del GIL:** En CPython, el intérprete libera automáticamente el *Global Interpreter Lock* (GIL) durante cualquier operación de entrada/salida bloqueante (como `socket.recv`, `socket.send` o llamadas al sistema subyacentes). Esto permite que múltiples hilos atiendan conexiones simultáneas en paralelo a nivel de red sin bloquearse entre sí, obteniendo concurrencia real para el flujo de trabajo requerido sin incurrir en el alto *overhead* de memoria y de comunicación inter-proceso (IPC) propio de `multiprocessing`.

### Sincronización del Sorteo y Barrera de Quórum
* **Espera No Activa:** Para sincronizar la resolución del sorteo se emplean primitivas de sincronización (`threading.Condition` / `threading.Event`).
* **Activación por Quórum:** Al recibir el mensaje `CODE_LAST_BATCH`, cada hilo actualiza el recuento de agencias finalizadas. Cuando el contador alcanza o supera `AGENCY_QUORUM_MIN`, se ejecuta la lógica de determinación de ganadores y se despierta a los hilos en espera mediante una señal de difusión (*notify all* o activación de evento).
* **Bloqueo Ordenado de Consultas:** Si un cliente solicita `CODE_GET_WINNERS` antes de completarse el quórum, el hilo asignado se suspende en la variable de condición sin consumir ciclos de CPU innecesarios, reanudando la respuesta apenas el sorteo queda disponible.

---

## 3. Manejo de Señales y Apagado Ordenado (Graceful Shutdown)

El sistema soporta interrupciones operativas asincrónicas asegurando la consistencia de los archivos y liberando los descriptores de red:

* **Servidor (Python):** Registra manejadores para `SIGTERM`. Al interceptar una señal, el servidor conmuta un indicador atómico de detención, cierra el socket de escucha para rechazar nuevas peticiones, notifica a los hilos de trabajo y aguarda su finalización (`join`) antes de culminar la ejecución con código de salida `0`.
* **Cliente (Go):** Emplea un contexto con cancelación vinculado a señales del sistema operativo (`signal.NotifyContext`). 
  * Ante la llegada de `SIGTERM`, una goroutine cierra el socket asociado para desbloquear operaciones bloqueantes de entrada/salida.
  * Cada fase del bucle principal inspecciona la cancelación del contexto (`ctx.Err() != nil`).
  * En caso de interrupción externa, el proceso concluye de forma limpia retornando error nulo y finalizando con código de salida `0` conforme a las pautas de integración del entorno.

---

## 5. Consideraciones sobre el Entorno de Pruebas

Para garantizar la correcta ejecución de la suite automatizada (`make test`), se deben tener en cuenta que,Dado que los tests de integración (en particular los de *batching*) levantan archivos `docker-compose` con configuraciones de red y puertos dedicados (`5678`), es imperativo que no existan servicios residuales ejecutándose previamente en el host. Si se utilizó `make up` para pruebas manuales, se debe ejecutar un `make down` previo a `make test` para liberar los puertos, evitar colisiones de contenedores huérfanos y prevenir la acumulación de datos o volúmenes no inicializados.
