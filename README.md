# EduGo Worker

Worker de procesamiento de eventos para la plataforma EduGo. Este servicio consume mensajes de RabbitMQ y procesa eventos relacionados con materiales educativos, evaluaciones y estudiantes.

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
│ Processors (Procesadores)       │
├─────────────────────────────────┤
│ • MaterialUploadedProcessor     │
│ • MaterialDeletedProcessor      │
│ • MaterialReprocessProcessor    │
│ • AssessmentAttemptProcessor    │
│ • StudentEnrolledProcessor      │
└─────────────────────────────────┘
```

## 📦 Requisitos

- Go 1.23 o superior
- PostgreSQL 14+
- MongoDB 6.0+
- RabbitMQ 3.11+
- Docker y Docker Compose (para desarrollo)

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
# PostgreSQL
POSTGRES_HOST=localhost
POSTGRES_PORT=5432
POSTGRES_USER=edugo
POSTGRES_PASSWORD=secret
POSTGRES_DB=edugo_db
POSTGRES_SSLMODE=disable

# MongoDB
MONGODB_URI=mongodb://localhost:27017
MONGODB_DATABASE=edugo_materials

# RabbitMQ
RABBITMQ_URL=amqp://guest:guest@localhost:5672/

# Logging
LOG_LEVEL=info
LOG_FORMAT=json

# API Admin (Autenticación)
API_ADMIN_BASE_URL=http://localhost:8081
API_ADMIN_TIMEOUT=5s
API_ADMIN_CACHE_TTL=60s
API_ADMIN_CACHE_ENABLED=true
```

### Ejemplo config.yaml

```yaml
database:
  postgres:
    host: localhost
    port: 5432
    database: edugo_db
    user: edugo
    password: secret
    ssl_mode: disable
    max_connections: 25
  mongodb:
    uri: mongodb://localhost:27017
    database: edugo_materials
    timeout: 10s

messaging:
  rabbitmq:
    url: amqp://guest:guest@localhost:5672/
    prefetch_count: 10
    queues:
      material_uploaded: material.uploaded
      assessment_attempt: assessment.attempt
    exchanges:
      materials: materials

logging:
  level: info
  format: json

api_admin:
  base_url: http://localhost:8081
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
│   │   └── processor/          # Procesadores de eventos
│   │       ├── registry.go     # Registro de procesadores
│   │       └── *_processor.go  # Implementaciones
│   ├── bootstrap/              # Inicialización de recursos
│   │   ├── adapter/            # Adaptadores (logger)
│   │   ├── resource_builder.go # Builder Pattern para recursos
│   │   └── DESIGN_*.md         # Documentación de diseño
│   ├── client/                 # Clientes externos (AuthClient)
│   ├── config/                 # Configuración
│   ├── container/              # Contenedor de dependencias
│   ├── domain/                 # Lógica de dominio
│   │   ├── service/            # Servicios de dominio
│   │   └── valueobject/        # Value Objects
│   └── infrastructure/         # Capa de infraestructura
│       ├── messaging/          # RabbitMQ consumer
│       └── persistence/        # Repositorios
├── docs/                       # Documentación adicional
├── improvements/               # Planes de mejora
├── Dockerfile
├── docker-compose.yml
├── Makefile
└── README.md
```

## 🔄 Procesadores de Eventos

El worker procesa los siguientes tipos de eventos:

### material_uploaded
Procesa materiales educativos subidos (PDFs, imágenes, videos).
- Extrae texto de PDFs
- Genera embeddings para búsqueda semántica
- Almacena metadatos en PostgreSQL y MongoDB

### material_deleted
Elimina materiales educativos del sistema.
- Limpia datos en PostgreSQL
- Elimina documentos de MongoDB
- Gestiona cleanup de recursos asociados

### material_reprocess
Reprocesa materiales existentes.
- Re-extrae texto
- Regenera embeddings
- Actualiza metadatos

### assessment_attempt
Procesa intentos de evaluación.
- Registra respuestas del estudiante
- Calcula puntuación
- Actualiza estadísticas

### student_enrolled
Procesa inscripciones de estudiantes.
- Registra inscripción
- Inicializa progreso
- Notifica al sistema

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

- [ProcessorRegistry Design](internal/application/processor/DESIGN_PROCESSOR_REGISTRY.md)
- [ResourceBuilder Design](internal/bootstrap/DESIGN_RESOURCE_BUILDER.md)

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
