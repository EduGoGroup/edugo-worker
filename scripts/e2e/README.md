# scripts/e2e — los guiones de inyección de punta a punta

Dos guiones en Python 3 (solo biblioteca estándar) que inyectan casos reales contra **el entorno local**
y dejan un JSON por caso. Nacieron en la corrida del 2026-11-24 (ecosistema,
`documentacion/analisis/inferencia-e2e/`, que queda congelada como evidencia) y viven aquí desde la AR-IA.2.

| Guion | Casos |
|---|---|
| `revision/run.py` | P1 a P3 y P3n (preparación), R1 a R13, R7n, R12n y R12b (revisión) |
| `material/run.py` | M1 a M5 (material → evaluación) |
| `concurrency/run.py` | C1 (el tope de inferencias: tres revisiones y dos PDF cortos a la vez) |

`e2e_env.py` es lo único que comparten: la guarda, las rutas de lo que vive fuera del repo, `--out-dir` y
la regla del código de salida. Los ayudantes de HTTP, SQL y RabbitMQ siguen duplicados entre los dos
guiones (deuda apuntada en el ecosistema).

## Uso

Desde la raíz del worker:

```bash
python3 scripts/e2e/revision/run.py --list
python3 scripts/e2e/material/run.py --list
```

```bash
EDUGO_BASE_PASSWORD=<clave> python3 scripts/e2e/revision/run.py --cases R5,R8 --out-dir <carpeta>
```

```bash
EDUGO_BASE_PASSWORD=<clave> python3 scripts/e2e/revision/run.py --no-model --out-dir <carpeta>
```

```bash
EDUGO_BASE_PASSWORD=<clave> python3 scripts/e2e/material/run.py --no-model --out-dir <carpeta>
```

- `--list` no necesita entorno ni pasa por la guarda: lista cada caso con su marca.
- `--cases` o `--no-model`, uno de los dos. `--no-model` corre solo los casos marcados «sin modelo».
- `--out-dir` es **obligatorio** para correr. Dentro se crea **una carpeta por corrida** con la fecha en UTC
  (`20261125T120000Z`): nada se pisa. Se rechaza una ruta dentro de
  `documentacion/analisis/inferencia-e2e/`.
- Material: M4 lee el `M1.json` de su misma corrida (`--cases M1,M4`) o recibe `--assessment-id`. M1 sube
  `ciclo_del_agua.pdf` salvo que se le dé `--material-id`. Además: `--pdf`, `--answers-file`,
  `--timeout-min`.
- Un caso cada vez, en serie: hay un solo entorno.

## El caso de concurrencia (C1)

```bash
EDUGO_BASE_PASSWORD=<clave> python3 scripts/e2e/concurrency/run.py --out-dir <carpeta>
```

`C1` es **con modelo**: se mide, no es gate, y arranca Ollama (se pregunta antes de correrlo). Siembra en
serie tres exámenes con la preparación fijada a mano y guarda un intento de cada uno; con el worker parado,
entrega los tres intentos y pide la generación de dos `ciclo_del_agua.pdf`; después arranca el worker, que
recibe los cinco mensajes de golpe. Reusa los ayudantes de `revision/` y de `material/` sin cambiarlos
(importa `material/run.py` con otro nombre de módulo; su `main` está tras `if __name__`).

- **Afirma solo relojes y colas**: ninguna línea `Client.Timeout exceeded` y ningún `Mensaje fallido: se
  reintenta` en el log del worker durante la ventana, y las tres colas muertas igual que antes.
- **Anota sin afirmar** (`measured`): el estado final de cada respuesta, intento y job, la espera de cada
  turno (`turn_waits`, de las líneas `turno de inferencia …`) y el reloj de pared, junto a los 1.023 s
  medidos sin tope con menos carga.
- `--timeout-min` cambia el tope de la ventana (40 minutos por defecto). `--list` no necesita entorno.
- Deja sembrado: tres evaluaciones con su intento, dos materiales y sus jobs.

## Qué necesita levantado

`make -C EduBack up-local` (Postgres `:5433`, RabbitMQ, las API y el worker), `edugo-minio` arriba para los
casos de material que suben un PDF y la clave de las cuentas `@edugo.test` en `EDUGO_BASE_PASSWORD`. El
login pasa por identity-core, por internet. Los casos con modelo piden además Ollama con el modelo de
revisión y, en material, `embeddinggemma`.

Lo que vive fuera de este repo se busca en `EduBack/`, la carpeta madre del worker: su `Makefile` (targets
`_stop-worker`, `_start-worker-local`, `_stop-ollama`, `_ensure-ollama`) y el `.env.local` de
`edugo-api-learning` y `edugo-api-identity`. Si el worker está en otra parte (un worktree), se apunta con
`EDUGO_EDUBACK_DIR`; si falta algo, el mensaje nombra esa variable.

Otras variables, todas con su valor local por defecto: `EDUGO_IDENTITY_URL`, `EDUGO_LEARNING_URL`,
`EDUGO_ACADEMIC_URL`, `EDUGO_OLLAMA_URL`, `RABBIT_API`, `RABBIT_USER`, `RABBIT_PASS`,
`EDUGO_PG_CONTAINER`, `EDUGO_PG_USER`, `EDUGO_PG_DB`, `EDUGO_ANSWER_TIMEOUT`.

## La guarda de entorno local

Antes de la primera petición, el guion comprueba tres cosas, de solo lectura. Si falla una, sale con
`rc=3` sin haber llamado a nada con token:

1. Todas las URL que usa son de `localhost`, `127.0.0.1` o `::1`. Una URL de fuera no se llama nunca.
2. `/tmp/edugo-mode` dice `local` (lo escribe `make up-local`; `make down` lo borra).
3. El proceso que escucha en el puerto de identity, learning y academic tiene una conexión abierta al
   Postgres local (`lsof` sobre su PID, tras un `/health` sin token). Es la única que mira el proceso
   vivo: caza una API arrancada a mano contra Neon después de `up-local`.

Falla cerrado: lo que no se pudo leer cuenta como problema.

## La marca y el código de salida

Cada caso lleva `needs_model` (en `--list` y en su JSON). Sin modelo: P3n, R5, R7n, R8, R11, R12n, R13 y M3.
Solo esos pueden ser un gate (ADR 0048 del ecosistema: la salida de un modelo se mide, no se prueba).

| `rc` | Qué significa |
|---|---|
| 0 | Todo lo afirmado por los casos sin modelo pasó |
| 1 | Un caso sin modelo tiene una afirmación en rojo o un error, o un caso cualquiera no pudo correr (rompió antes de sembrar nada, o falta algo de `EduBack/`) |
| 2 | Uso inválido |
| 3 | La guarda: no es el entorno local |

En un caso con modelo, las afirmaciones en rojo y las esperas agotadas se cuentan en la línea
`casos con modelo no cumplidos: n` y no suben el código.

## Las variantes sin modelo

Afirmaban la conducta con defecto; cada HU que arregló el suyo cambió el valor esperado en un solo sitio.
Hoy afirman la conducta corregida.

| Caso | Función común | Qué afirma hoy | La invirtió |
|---|---|---|---|
| P3n (y P3) | `case_p3n`, `case_p3` | El mensaje de una pregunta ya `ready` deja la línea `preparación ya lista, no se repite`, no llama al modelo y no cambia la fila; el 409 por hash viejo sigue | HU-IA.4 |
| R7n (y R7) | `direct_flow_review` | El candado queda liberado tras finalizar | HU-IA.1 |
| R12n (y R12) | `review_until_dead_letter` | El mensaje muerto es persistente y lleva `x-error` y `x-dead-reason=max_retries`; siguen las cuatro fallas y `x-retry-count=3` | HU-IA.3 |
| R13 | `case_r13` | Cada mensaje falla una sola vez y muere al momento: `x-retry-count=0`, `x-dead-reason=permanent`, `x-error` no vacío y persistente; con la IA apagada, la sonda queda `none` | HU-IA.3 y HU-IA.4 |

R12n **para Ollama** si está arriba (y lo vuelve a levantar al terminar); si ya está abajo, no toca nada.
Si lo que escucha en `:11434` es el Ollama de escritorio, lo mata: se corre con permiso. Un caso sin modelo
no arranca Ollama al restaurar el entorno.

## Qué deja sembrado

Nada se limpia: cada caso deja su evaluación, su intento y, según el caso, mensajes en colas muertas (R12n,
R13) o dos jobs `failed` con su motivo, que no bloquean su material (M3). El único SQL que escribe es el de R7 y R7n
(`llm.review.flow`), que se restaura al terminar el caso.

## Gate

Sin red ni Docker, desde la raíz del worker:

```bash
python3 -m unittest discover -s scripts/e2e/tests -v
```

```bash
python3 -m py_compile scripts/e2e/e2e_env.py scripts/e2e/revision/*.py scripts/e2e/material/run.py scripts/e2e/tests/*.py
```
