# EduGo Worker

Worker de procesamiento de eventos para la plataforma EduGo. Consume mensajes de RabbitMQ y ejecuta trabajo pesado/asíncrono detrás de `edugo-api-learning`.

## Estado: esqueleto para el carril LLM (037 D-037.11)

Tras el plan 037 el worker quedó como **esqueleto sin processors**. El único carril que le quedaba (`material.uploaded` / `material.reprocess`) persistía su salida **100% en Mongo** y **no tenía consumidor** (el worker nunca se desplegó en Cloud Run). Al retirarse Mongo del ecosistema, esos processors se eliminaron.

Lo que **sigue vivo** hoy es la cáscara operativa:

- Conecta a **RabbitMQ**, declara el exchange/cola y arranca el consumer con soporte DLQ.
- Expone **healthcheck** (liveness/readiness) y servidor de **métricas** Prometheus.
- Mantiene **AuthClient** (identity), rate limiter, circuit breakers, graceful shutdown y el pipeline de infraestructura (S3/PDF/NLP) listo para reusar.
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

- Go 1.25 o superior
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
make lint           # Ejecutar linter
make format         # Formatear código
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
│       ├── storage/ pdf/ nlp/  # S3 · extracción PDF · NLP (listos para F3)
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
# .github/workflows/pr.yml
- Format Check (gofmt)
- Linting (golangci-lint)
- Unit Tests
- Integration Tests
- Coverage Report
- go.mod/go.sum Validation
```

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
make format
make lint
make test
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
