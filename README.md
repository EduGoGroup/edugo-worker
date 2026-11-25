# EduGo Worker

Worker de procesamiento de eventos para la plataforma EduGo. Consume mensajes de RabbitMQ y ejecuta trabajo pesado/asíncrono detrás de `edugo-api-learning`.

## Estado: esqueleto para el carril LLM (037 D-037.11)

Tras el plan 037 el worker quedó como **esqueleto sin processors**. El único carril que le quedaba (`material.uploaded` / `material.reprocess`) persistía su salida **100% en Mongo** y **no tenía consumidor** (el worker nunca se desplegó en Cloud Run). Al retirarse Mongo del ecosistema, esos processors se eliminaron.

Lo que **sigue vivo** hoy es la cáscara operativa:

- Conecta a **RabbitMQ**, declara el exchange/cola y arranca el consumer con soporte DLQ.
- Expone **healthcheck** (liveness/readiness) y servidor de **métricas** Prometheus.
- Mantiene **AuthClient** (identity), rate limiter, circuit breakers, graceful shutdown y el pipeline de infraestructura (PDF/NLP) listo para reusar.
- El **`ProcessorRegistry` arranca vacío**: no hay processor de negocio. Cualquier mensaje que llegara iría al DLQ por "no processor registered".

Ya **no** hay Postgres ni Mongo: sus conexiones, config, health checks y métricas de BD se retiraron. Los processors del carril LLM (store y orquestación nuevos) llegan en **037-F3**.

## 📋 Tabla de Contenidos

- [Arquitectura](#arquitectura)
- [Requisitos](#requisitos)
- [Instalación](#instalación)
- [Configuración](#configuración)
- [Uso](#uso)
- [Estructura del Proyecto](#estructura-del-proyecto)
- [Procesadores de Eventos](#procesadores-de-eventos)
- [Testing](#testing)
- [CI/CD](#cicd)
- [Mejoras Recientes](#mejoras-recientes)

## 🏗️ Arquitectura

El worker está construido con una arquitectura limpia basada en:

- **Bootstrap Pattern**: Inicialización ordenada de recursos usando Builder Pattern
- **Processor Registry**: Registro dinámico de procesadores de eventos
- **Dependency Injection**: Contenedor de dependencias para gestión centralizada
- **Structured Logging**: Logger estructurado usando logrus a través de edugo-shared

### Componentes Principales

```
┌─────────────────┐
│   RabbitMQ      │
│   (Mensajes)    │
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│ EventConsumer   │
│ (Consumidor)    │
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│ ProcessorRegistry│
│ (Enrutador)     │
└────────┬────────┘
         │
         ▼
┌─────────────────────────────────┐
│ ProcessorRegistry               │
│ (VACÍO — carril LLM en 037-F3)  │
└─────────────────────────────────┘
```

### Topología RabbitMQ (estado esqueleto, plan 037 D-037.11)

El worker declara y conecta la topología de RabbitMQ como cáscara, pero **no
consume negocio**: el `ProcessorRegistry` arranca vacío.

- **Un exchange** (`topic`): `edugo.materials` — existe; `edugo-api-learning`
  publica `material.uploaded`.
- **Una cola**: `edugo.material.uploaded`, con un único binding a la routing key
  `material.uploaded`, declarada al arrancar (soporte DLQ).
- **Consumo**: el worker arranca el consumer sobre esa cola, pero al no haber
  ningún processor registrado, cualquier mensaje sería rechazado al DLQ ("no
  processor registered"). En la práctica **el worker no procesa nada hasta F3**.
- **Actores**: `edugo-api-learning` (productor) → `edugo-worker` (consumidor
  cáscara, sin processors).

RabbitMQ se mantiene cableado a propósito: es la **cola del norte LLM** (D-037.3),
donde 037-F3 registrará los processors nuevos (evaluación por LLM), con su store
y orquestación propios.

Contexto de lo retirado antes del esqueleto:

- `material.uploaded` / `material.reprocess` — sus processors se **eliminaron** en
  D-037.11: persistían la salida en Mongo (retirado del ecosistema) y no tenían
  consumidor (worker nunca desplegado).
- `material.deleted` — la limpieza es **síncrona en edugo-api-learning** (D-037.4).
- `assessment.assigned` / `student.enrolled` — learning llama **directo al
  Notification Gateway** de platform (patrón 032-A4); ya no se publican a Rabbit.

## 📦 Requisitos

- Go 1.27.4 o superior
- RabbitMQ 3.11+
- Docker y Docker Compose (para desarrollo)

> Ya **no** requiere PostgreSQL ni MongoDB (estado esqueleto, plan 037 D-037.11).

## 🚀 Instalación

### Desarrollo Local

1. Clonar el repositorio:
```bash
git clone https://github.com/EduGoGroup/edugo-worker.git
cd edugo-worker
```

2. Instalar dependencias:
```bash
go mod download
```

3. Compilar el proyecto:
```bash
make build
```

### Usando Docker

```bash
# Construir la imagen
docker build -t edugo-worker .

# Ejecutar con docker-compose
docker-compose up -d
```

## ⚙️ Configuración

El worker se configura mediante variables de entorno o archivo `config.yaml`.

### Variables de Entorno Requeridas

```bash
# RabbitMQ (única dependencia obligatoria del esqueleto)
RABBITMQ_URL=amqp://guest:guest@localhost:5672/

# Logging
LOG_LEVEL=info
LOG_FORMAT=json

# API Identity (Autenticación)
API_IDENTITY_BASE_URL=http://localhost:8070/api
API_IDENTITY_TIMEOUT=5s
API_IDENTITY_CACHE_TTL=60s
API_IDENTITY_CACHE_ENABLED=true
```

### Variables del carril LLM y M2M (plan 039 F4)

El worker estrena clientes M2M de dominio y el puerto `LLMProvider`. Estas variables
son **opcionales en local** (el worker arranca sin ellas; el M2M queda inerte y el
provider LLM apunta a Ollama por defecto), y **obligatorias en cloud** cuando se
enciendan los planes 040/041.

```bash
# M2M — service JWT (HS256) que el worker presenta a las APIs de dominio.
# El secret es DISTINTO del de usuarios; en cloud vive en Secret Manager (patrón
# service-jwt-secret). Issuer/Audience por convención: academic espera
# iss=edugo-identity, aud=edugo-api-academic. Scope requerido: schools.settings.read.
SERVICE_JWT_SECRET=<secret-hs256>          # vacío en local => M2M no operativo (academic rechaza el token)

# M2M — base URLs de las APIs de dominio que consulta el worker.
API_ACADEMIC_BASE_URL=http://localhost:8060  # lectura de settings de escuela (política LLM)
API_LEARNING_BASE_URL=http://localhost:8065  # stub, lo consume el plan 040

# LLM local (modo "local" = Ollama). Credenciales/URL/modelo son de EduGo, NO por
# escuela (D-039.3): lo por-escuela es solo la política (se lee vía M2M).
LLM_LOCAL_BASE_URL=http://localhost:11434
LLM_LOCAL_MODEL=gemma4:e4b                  # único modelo de los 3 rieles (deuda 037, medido 2026-07-18)

# LLM por API (modo "api" = Claude/Gemini). La API key en cloud va en Secret Manager.
LLM_API_PROVIDER=anthropic                  # anthropic (completo) | gemini (stub)
LLM_API_KEY=<api-key>
LLM_API_MODEL=claude-sonnet-5
```

> **Cloud (Secret Manager):** `SERVICE_JWT_SECRET` y `LLM_API_KEY` se montan como
> secretos (nunca en la imagen ni en el repo), igual que el `service-jwt-secret`
> del plan 020. Las base URLs apuntan a los servicios internos de Cloud Run.

### Harness de providers LLM (D-039.8, ADR 0048)

`make llm-harness ARGS="..."` (o `go run ./cmd/llm-harness ...`) pasa casos por el
modelo real y **mide, no juzga** (ADR 0048 del ecosistema):

- **Una corrida por caso.** No hay reintentos ni casos apartados: una salida vacía o
  inválida es un dato (`inválidas: n`).
- **No hay PASS/FAIL por lo que escribe el modelo.** El código de salida solo es != 0
  si el arnés no pudo correr (sin modelo, sin banco, sin poder guardar).
- **Cada corrida se guarda** como fichero nuevo en `cmd/llm-harness/testdata/runs/`,
  con la fecha UTC en el nombre, el modelo y su digest, y el commit del worker. Nunca
  pisa una anterior.
- **El modelo** sale de `-model` o de `LLM_LOCAL_MODEL`. Sin modelo dicho no arranca.
  Antes de medir comprueba que Ollama responde y que el modelo está instalado. No
  descarga nada.

Modos: `generate` (contrato 038), `review`, `prep`, `review-prep`, `material`,
`material-pairs`, `embed`, `relevance` y `bank`.

**El modo `material`** (AR-IA.3) mide el riel material→evaluación entero. Por cada caso de
`cmd/llm-harness/testdata/material/cases.json` (un PDF) monta **el procesador de producción**
(`processor.NewMaterialPipelineProcessor` y su `Process`) sobre un almacén en memoria que hace de
learning: fase 0 (extraer y trocear), fase 1 (leer y preguntar por trozo) y las cuatro pasadas de
la fase 2, con los reintentos de producción y sus parámetros (300/400/200/80, 0,93 y 0,50,
relevancia 0,4, cupo 20). Sin cola, sin learning y sin base: solo Ollama, con el modelo generador
y el de embeddings (`-embed-model`, por defecto el de la configuración). Imprime cinco tablas
(generación, sustento, reducción, borrador y coste) y guarda la corrida con las candidatas
enteras. Una reentrega de la cola se imita volviendo a llamar a `Process` sobre el mismo almacén,
hasta 3 veces y solo tras un error transitorio; un caso que no termina se guarda como
`incomplete`. Antes de mirar si hay modelo comprueba los datos: un PDF que falta o una sección
declarada que no aparece en el texto extraído paran el arnés con código 1.

**El modo `material-pairs`** (AR-IA.3) pasa las parejas etiquetadas de
`testdata/material/pairs.json` por la pasada de duplicadas real (`reduce.DedupePass`), cada una
dos veces: con el embedder de producción (qué escalón decide: letras, coseno o juez) y con uno
fijo en la zona gris (el juez solo). Cuenta por categoría lo que coincide con la etiqueta, las
duplicadas que se escapan, las falsas duplicadas y las salidas inválidas.

```bash
LLM_LOCAL_MODEL=gemma4:e4b make llm-harness ARGS="-mode material"
LLM_LOCAL_MODEL=gemma4:e4b make llm-harness ARGS="-mode material-pairs"
```

Para medir otro PDF sin tocar los casos congelados: `-material-inputs ruta/al.pdf` (sin
secciones) o `-material-cases otro.json`.

**El modo `bank`** (AR-INF.1) pasa el banco etiquetado de learning por el carril de
corrección real (`processor.ReviewAnswerLane`). Solo pasan los casos que la parte
determinista dejó pendientes, que son los que en producción llegan al modelo. El
informe de esos casos lo escribe la prueba de learning:

```bash
# 1. En edugo-api-learning: el informe determinista (fuera de git).
ANSWER_BANK_REPORT=/tmp/bank-det.json GOWORK=off go test -count=1 -run TestAnswerBank ./internal/core/domain/

# 2. Aquí: el banco por el modelo local, a temperatura 0, una corrida por modelo.
for m in gemma4:e2b-it-qat gemma4:e4b; do
  make llm-harness ARGS="-mode bank -provider ollama -model $m -timeout 300s \
    -bank ../edugo-api-learning/internal/core/domain/testdata/answer_bank.json \
    -deterministic /tmp/bank-det.json"
done
```

Desde la HU-INF.5 el «antes» y el «después» se miden con los dos modelos, `gemma4:e2b-it-qat`
y `gemma4:e4b` (el de `LLM_LOCAL_MODEL`), con `-timeout 300s` por caso (no los 120 s de
producción). El banco lee el `answer_spec` de cada caso, como lo sirve el `GET` de pendientes.
La tabla separa las vacías resueltas por regla (sin modelo) de las que no hicieron ninguna
llamada (una lista resuelta sin modelo), cuenta los reintentos de una salida inválida, las
extracciones de ideas con error y vacías, y los casos en que el `score` del modelo no es el
número que propone Go: en los carriles globales el número sale del veredicto.

**Lo que cierra la salida del modelo** (HU-INF.5). Las llamadas del carril de revisión (el
juicio global, el par, el criterio y la extracción de ideas) mandan a Ollama el JSON Schema de su
salida en `format`, y Go lo revalida siempre (`internal/llm/schema.go`). El proveedor de pago no
manda esquema: la versión de API del worker no consta como compatible. Una salida inválida se
reintenta una vez a temperatura 0,3 y, si sigue mal, es un error: la respuesta queda sin
propuesta para la docente. Una respuesta sin letras ni números se cierra como incorrecta sin
llamar al modelo.

Otros ejemplos:

```bash
make llm-harness ARGS="-mode review -provider ollama -model qwen2.5:7b"
make llm-harness ARGS="-provider api -api-provider anthropic -api-key $LLM_API_KEY -api-model claude-sonnet-5 -material ./material.txt"
```

### Ejemplo config.yaml

```yaml
messaging:
  rabbitmq:
    url: amqp://guest:guest@localhost:5672/
    prefetch_count: 10
    queues:
      material_uploaded: material.uploaded
    exchanges:
      materials: edugo.materials

logging:
  level: info
  format: json

api_identity:
  base_url: http://localhost:8070/api
  timeout: 5s
  cache_ttl: 60s
  cache_enabled: true
```

## 🎯 Uso

### Ejecutar el Worker

```bash
# Usando el binario compilado
./bin/worker

# Usando go run
go run cmd/main.go

# Usando make
make run
```

### Comandos Make Disponibles

```bash
make build          # Compilar el proyecto
make test           # Ejecutar tests
make test-coverage  # Tests con reporte de cobertura
make test-integration  # Tests de integración (gate local previo al PR; requiere Docker)
make lint           # Ejecutar linter (falla si hay hallazgos o si falta golangci-lint)
make fmt            # Formatear código
make ci-local       # fmt + vet + lint + test-unit + govulncheck
make clean          # Limpiar binarios
```

## 📁 Estructura del Proyecto

```
edugo-worker/
├── cmd/
│   └── main.go                 # Punto de entrada
├── internal/
│   ├── application/
│   │   ├── dto/                # Data Transfer Objects
│   │   └── processor/          # Registry + interfaz + retry (registry VACÍO)
│   │       ├── registry.go     # Registro de procesadores
│   │       └── processor.go    # Interfaz Processor (sin implementaciones aún)
│   ├── bootstrap/              # Inicialización de recursos
│   │   ├── resource_builder.go # Builder Pattern para recursos
│   │   └── DESIGN_RESOURCE_BUILDER.md # Documentación de diseño
│   ├── client/                 # Clientes externos (AuthClient)
│   ├── config/                 # Configuración
│   ├── domain/                 # Lógica de dominio
│   │   └── valueobject/        # Value Objects
│   └── infrastructure/         # Capa de infraestructura
│       ├── messaging/          # RabbitMQ consumer
│       ├── pdf/ nlp/           # extracción PDF · NLP (listos para F3)
│       ├── health/ metrics/    # Liveness/readiness · Prometheus
│       └── ratelimiter/ circuitbreaker/ shutdown/
├── docs/                       # Documentación técnica (arquitectura, eventos, etc.)
├── Dockerfile
├── docker-compose.yml
├── Makefile
└── README.md
```

## 🔄 Procesadores de Eventos

**El `ProcessorRegistry` arranca vacío** (estado esqueleto, plan 037 D-037.11): el
worker no registra ningún processor de negocio. Los processors del carril LLM se
añadirán en **037-F3**, que definirá su store y orquestación.

Para agregar uno nuevo (cuando llegue F3): implementa un `*_processor.go` que
satisfaga la interfaz de `processor.go`, regístralo en el `ProcessorRegistry`
(bootstrap, `WithProcessors`) y mapea su `event_type`/binding en RabbitMQ.

> Los processors `material.uploaded` / `material.reprocess` se **eliminaron** en
> D-037.11 (persistían en Mongo, ya retirado, y no tenían consumidor). Los eventos
> `material.deleted`, `assessment.assigned` y `student.enrolled` ya se habían
> retirado antes: su negocio migró a las APIs de dominio (limpieza síncrona en
> learning; notificaciones directas al Notification Gateway de platform, patrón
> 032-A4). Ver [Topología RabbitMQ](#topología-rabbitmq-estado-esqueleto-plan-037-d-03711).

## 🧪 Testing

### Ejecutar Tests

```bash
# Todos los tests
make test

# Tests con cobertura
make test-coverage

# Tests de un paquete específico
go test ./internal/bootstrap/... -v

# Tests con cobertura detallada
go test ./... -coverprofile=coverage.out
go tool cover -html=coverage.out
```

### Cobertura Actual

```
✅ adapter:    82.2%
✅ container:  84.2%
✅ client:     82.3%
⚠️  bootstrap: 33.1%
⚠️  processor: 22.0%
```

### Estructura de Tests

- **Unit Tests**: Tests unitarios para componentes individuales
- **Integration Tests**: Tests de integración con bases de datos
- **Mocks**: Uso de interfaces para facilitar testing

## 🔧 CI/CD

El proyecto usa GitHub Actions para CI/CD:

### Pipeline de PR

```yaml
# .github/workflows/ci.yml
- Format Check (gofmt)
- go.mod/go.sum Validation
- go vet
- Tests (go test -race ./...)
- govulncheck
- Linting (golangci-lint)
- Docker Build Test
```

Los tests de integración **no corren en CI**: son un gate local obligatorio antes de cada PR
(`RUN_INTEGRATION_TESTS=true GOWORK=off make test-integration`, con Docker).

### Validaciones

- ✅ Código formateado con `gofmt`
- ✅ Sin errores de linter
- ✅ Tests pasando
- ✅ Cobertura > 30%
- ✅ go.mod sincronizado

## 🎉 Mejoras Recientes

### Fase 1: Refactorización Bootstrap (Completada)

**Objetivo**: Mejorar la inicialización de recursos y eliminar código complejo.

**Cambios Implementados**:

1. **ProcessorRegistry Pattern** (T1.1-T1.4)
   - ✅ Eliminado switch gigante en favor de registro dinámico
   - ✅ Registry con enrutamiento automático basado en event_type
   - ✅ Desacoplamiento de consumer y processors
   - **Reducción**: -180 líneas de código

2. **ResourceBuilder Pattern** (T1.5-T1.9)
   - ✅ Builder con API fluida para inicialización
   - ✅ Eliminación de doble punteros (**Type)
   - ✅ Cleanup LIFO garantizado
   - ✅ Validación de dependencias en tiempo de build
   - **Reducción**: -360 líneas de código complejo

3. **Tests y Cobertura** (T1.10)
   - ✅ Logger Adapter: 82.2% (0% → 82.2%)
   - ✅ Container: 84.2% (0% → 84.2%)
   - ✅ Total: +655 líneas de tests

4. **Documentación** (T1.11)
   - ✅ README completo
   - ✅ Documentación de diseño
   - ✅ Ejemplos de uso

**Impacto**:
- 📉 -540 líneas de código complejo eliminadas
- 📈 +655 líneas de tests agregadas
- 🎯 Cobertura mejorada significativamente
- 🚀 Código más mantenible y testeable

### Documentos de Diseño

- [ResourceBuilder Design](internal/bootstrap/DESIGN_RESOURCE_BUILDER.md)

### Documentación Técnica

Ver [`docs/`](docs/) para referencia detallada: arquitectura, configuración, eventos, infraestructura, procesadores y testing.

## 📝 Contribuir

1. Crear una rama desde `dev`:
```bash
git checkout dev
git pull origin dev
git checkout -b feature/mi-feature
```

2. Hacer cambios y crear commits atómicos:
```bash
git add .
git commit -m "feat: descripción del cambio"
```

3. Ejecutar validaciones locales:
```bash
make ci-local                                               # fmt, vet, lint, tests unitarios y govulncheck
RUN_INTEGRATION_TESTS=true GOWORK=off make test-integration # obligatorio: la integración no corre en CI (requiere Docker)
```

4. Push y crear PR:
```bash
git push origin feature/mi-feature
# Crear PR en GitHub apuntando a 'dev'
```

## 📄 Licencia

Propietario: EduGo Group

## 🔗 Enlaces

- [Repositorio](https://github.com/EduGoGroup/edugo-worker)
- [Issues](https://github.com/EduGoGroup/edugo-worker/issues)
- [Pull Requests](https://github.com/EduGoGroup/edugo-worker/pulls)

## 📞 Soporte

Para preguntas o problemas, crear un issue en el repositorio.
