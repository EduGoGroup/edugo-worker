# EduGo Worker - Documentación Técnica Completa

> **Última actualización:** Diciembre 2024  
> **Versión:** 1.0.0  
> **Mantenedor:** EduGo Team

---

## 📋 Índice de Documentación

### Documentación Principal

| Documento | Descripción | Audiencia |
|-----------|-------------|----------|
| [ARQUITECTURA.md](./ARQUITECTURA.md) | Diagrama de arquitectura, capas, componentes y patrones | Desarrolladores, Arquitectos |
| [BASE_DE_DATOS.md](./BASE_DE_DATOS.md) | Esquema de bases de datos PostgreSQL + MongoDB | Desarrolladores, DBAs |
| [PROCESOS.md](./PROCESOS.md) | Flujos de procesamiento, máquina de estados, diagramas | Desarrolladores |
| [EVENTOS.md](./EVENTOS.md) | Eventos RabbitMQ, DTOs, estructura JSON | Desarrolladores, QA |
| [CONFIGURACION.md](./CONFIGURACION.md) | Variables de entorno, archivos YAML, Docker | DevOps, Desarrolladores |
| [SERVICIOS.md](./SERVICIOS.md) | Dependencias externas y servicios requeridos | DevOps, Arquitectos |

### Documentación de Mejoras

| Documento | Descripción |
|-----------|-------------|
| [mejoras/CODIGO_DEPRECADO.md](./mejoras/CODIGO_DEPRECADO.md) | Código identificado como deprecado o candidato a eliminación |
| [mejoras/REFACTORING.md](./mejoras/REFACTORING.md) | Propuestas de refactorización y mejoras de código |
| [mejoras/DEUDA_TECNICA.md](./mejoras/DEUDA_TECNICA.md) | Deuda técnica identificada y plan de resolución |
| [mejoras/ROADMAP.md](./mejoras/ROADMAP.md) | Roadmap de mejoras técnicas planificadas |

---

## 🎯 ¿Qué es EduGo Worker?

**EduGo Worker** es un servicio de procesamiento asíncrono que consume eventos de RabbitMQ para procesar materiales educativos. Es parte del ecosistema EduGo, una plataforma educativa.

### Responsabilidades Principales

1. **Procesar materiales subidos** - Cuando un docente sube un PDF, el worker:
   - Extrae texto del documento
   - Genera resúmenes con IA (OpenAI GPT-4)
   - Crea evaluaciones/quizzes automáticamente
   - Almacena los resultados en MongoDB

2. **Limpiar datos eliminados** - Cuando se elimina un material, limpia datos relacionados

3. **Procesar intentos de evaluación** - Registra y analiza resultados de quizzes

4. **Gestionar inscripciones** - Procesa eventos de inscripción de estudiantes

---

## 🏗️ Visión General de la Arquitectura

```
┌─────────────────────────────────────────────────────────────────────┐
│                         ECOSISTEMA EDUGO                             │
├─────────────────────────────────────────────────────────────────────┤
│                                                                      │
│   ┌──────────────┐         ┌──────────────┐         ┌─────────────┐ │
│   │  API Mobile  │         │  API Admin   │         │  Frontend   │ │
│   │   (REST)     │         │   (REST)     │         │   (React)   │ │
│   └──────┬───────┘         └──────┬───────┘         └─────────────┘ │
│          │                        │                                  │
│          │ Publica eventos        │ Valida tokens                   │
│          ▼                        │                                  │
│   ┌──────────────────────────────┴────────────────────────────────┐ │
│   │                         RabbitMQ                               │ │
│   │  Exchange: edugo.materials (topic)                             │ │
│   │  Queue: edugo.material.uploaded                                │ │
│   └───────────────────────────┬───────────────────────────────────┘ │
│                               │                                      │
│                               │ Consume eventos                      │
│                               ▼                                      │
│   ┌───────────────────────────────────────────────────────────────┐ │
│   │                      EDUGO WORKER                              │ │
│   │  ┌──────────────┐  ┌──────────────┐  ┌───────────────────────┐│ │
│   │  │  Processors  │  │   Domain     │  │   Infrastructure      ││ │
│   │  │              │  │   Services   │  │                       ││ │
│   │  │ • Material   │  │              │  │  • MongoDB Repos      ││ │
│   │  │   Uploaded   │  │ • State      │  │  • Auth Client        ││ │
│   │  │ • Material   │  │   Machine    │  │  • RabbitMQ Consumer  ││ │
│   │  │   Deleted    │  │ • Validators │  │                       ││ │
│   │  │ • Assessment │  │              │  │                       ││ │
│   │  │   Attempt    │  │              │  │                       ││ │
│   │  └──────────────┘  └──────────────┘  └───────────────────────┘│ │
│   └───────────────────────────┬───────────────────────────────────┘ │
│                               │                                      │
│          ┌────────────────────┼────────────────────┐                │
│          ▼                    ▼                    ▼                │
│   ┌─────────────┐      ┌─────────────┐      ┌─────────────┐        │
│   │  PostgreSQL │      │   MongoDB   │      │   OpenAI    │        │
│   │  (Estado)   │      │  (Contenido)│      │   (GPT-4)   │        │
│   └─────────────┘      └─────────────┘      └─────────────┘        │
│                                                                      │
└─────────────────────────────────────────────────────────────────────┘
```

---

## 🚀 Quick Start

### Prerrequisitos

- Go 1.25+
- Docker & Docker Compose
- Acceso a repositorios privados de EduGoGroup

### Variables de Entorno Requeridas

```bash
export POSTGRES_PASSWORD=your_password
export MONGODB_URI=mongodb://user:pass@host:27017/edugo?authSource=admin
export RABBITMQ_URL=amqp://user:pass@host:5672/
export OPENAI_API_KEY=sk-your-key
export APP_ENV=local  # local, dev, qa, prod
```

### Ejecución Local

```bash
# 1. Clonar e instalar dependencias
git clone https://github.com/EduGoGroup/edugo-worker.git
cd edugo-worker
make deps

# 2. Ejecutar
make run

# 3. O con Docker
make docker-build
make docker-run
```

---

## 📁 Estructura del Proyecto

```
edugo-worker/
├── cmd/
│   └── main.go                 # Punto de entrada
├── config/
│   ├── config.yaml             # Config base
│   ├── config-local.yaml       # Override local
│   ├── config-dev.yaml         # Override desarrollo
│   ├── config-qa.yaml          # Override QA
│   └── config-prod.yaml        # Override producción
├── internal/
│   ├── application/
│   │   ├── dto/                # Data Transfer Objects
│   │   └── processor/          # Procesadores de eventos
│   ├── bootstrap/              # Inicialización de recursos
│   ├── client/                 # Clientes HTTP (AuthClient)
│   ├── config/                 # Carga de configuración
│   ├── container/              # Dependency Injection
│   ├── domain/
│   │   ├── constants/          # Constantes de dominio
│   │   ├── service/            # Servicios de dominio
│   │   └── valueobject/        # Value Objects (MaterialID)
│   └── infrastructure/
│       ├── messaging/          # RabbitMQ
│       ├── nlp/                # Integración OpenAI
│       ├── pdf/                # Extracción de texto PDF
│       ├── persistence/        # Repositorios MongoDB
│       └── storage/            # AWS S3
├── documents/                  # Esta documentación
├── Dockerfile
├── docker-compose.yml
├── Makefile
└── go.mod
```

---

## 🔑 Tecnologías Clave

| Tecnología | Uso |
|------------|-----|
| **Go 1.25** | Lenguaje principal |
| **RabbitMQ** | Message broker para eventos |
| **PostgreSQL** | Estado de materiales (transaccional) |
| **MongoDB** | Contenido generado (resúmenes, quizzes) |
| **OpenAI GPT-4** | Generación de resúmenes y evaluaciones |
| **AWS S3** | Almacenamiento de PDFs |
| **edugo-shared** | Librería compartida del ecosistema |
| **edugo-infrastructure** | Entidades MongoDB compartidas |

---

## 📊 Métricas y Monitoreo

El worker expone información útil para monitoreo:

- **Logs estructurados** en formato JSON
- **Circuit breaker** para llamadas a api-admin
- **Cache de tokens** con estadísticas disponibles
- **Graceful shutdown** para cierre ordenado

### Logs Estructurados

Todos los logs siguen un formato estructurado para facilitar el parsing:

```json
{
  "level": "info",
  "msg": "processing material uploaded",
  "material_id": "550e8400-e29b-41d4-a716-446655440000",
  "s3_key": "materials/courses/unit-123/document.pdf",
  "timestamp": "2024-01-15T10:30:00Z"
}
```

### Niveles de Log

| Nivel | Uso | Ejemplo |
|-------|-----|--------|
| `debug` | Información detallada para desarrollo | "extracting PDF text" |
| `info` | Eventos normales de operación | "material processing completed" |
| `warn` | Situaciones anómalas no críticas | "retry attempt 2 of 3" |
| `error` | Errores que requieren atención | "failed to connect to MongoDB" |
| `fatal` | Errores que impiden continuar | "configuration invalid" |

### Circuit Breaker States

El AuthClient usa circuit breaker con tres estados:

```
┌─────────────┐     60% fallos      ┌─────────────┐     30s timeout     ┌─────────────┐
│   CLOSED    │ ──────────────▶ │    OPEN     │ ──────────────▶ │  HALF-OPEN  │
│ (normal)    │                │ (bloqueado) │                │ (probando)  │
└─────────────┘                └─────────────┘                └──────┬──────┘
       ▲                                                              │
       │                            Éxito                             │
       └──────────────────────────────────────────────────────────────┘
```

---

## 🧪 Testing

```bash
# Tests unitarios
make test-unit

# Tests con cobertura
make test-coverage

# Todos los tests
make test

# Benchmarks
make benchmark
```

---

## 📝 Comandos Make Disponibles

```bash
make help          # Ver todos los comandos
make build         # Compilar binario
make run           # Ejecutar en desarrollo
make test          # Ejecutar tests
make test-coverage # Tests con reporte HTML
make fmt           # Formatear código
make lint          # Linter completo
make docker-build  # Build imagen Docker
make docker-run    # Ejecutar con compose
make clean         # Limpiar artefactos
```

---

## 🔒 Seguridad

### Manejo de Secretos

| Secreto | Variable de Entorno | Nunca en... |
|---------|---------------------|-------------|
| PostgreSQL Password | `POSTGRES_PASSWORD` | config.yaml, logs |
| MongoDB URI | `MONGODB_URI` | config.yaml, logs |
| RabbitMQ URL | `RABBITMQ_URL` | config.yaml, logs |
| OpenAI API Key | `OPENAI_API_KEY` | config.yaml, logs, código |

### Recomendaciones de Seguridad

1. **Variables de entorno:** Usar siempre para secretos
2. **AWS Secrets Manager:** Para producción
3. **Rotación de secretos:** Cada 90 días mínimo
4. **Audit logs:** Registrar accesos a recursos sensibles
5. **Network policies:** Restringir acceso entre servicios

---

## 🛠️ Troubleshooting

### Problemas Comunes

#### El worker no procesa mensajes

```bash
# 1. Verificar conexión a RabbitMQ
docker exec -it rabbitmq rabbitmqctl list_queues

# 2. Verificar que la cola tiene mensajes
# Buscar: edugo.material.uploaded

# 3. Verificar logs del worker
docker logs edugo-worker --tail 100

# 4. Verificar variables de entorno
docker exec edugo-worker env | grep -E 'RABBITMQ|POSTGRES|MONGODB'
```

#### Error de conexión a PostgreSQL

```bash
# Verificar que PostgreSQL está corriendo
docker ps | grep postgres

# Probar conexión directa
psql -h localhost -U edugo_user -d edugo -c "SELECT 1;"

# Verificar password
echo $POSTGRES_PASSWORD
```

#### Error de conexión a MongoDB

```bash
# Verificar que MongoDB está corriendo
docker ps | grep mongo

# Probar conexión
mongosh "$MONGODB_URI" --eval "db.runCommand({ping:1})"
```

#### Mensajes van a Dead Letter Queue

```bash
# Ver mensajes en DLQ
docker exec -it rabbitmq rabbitmqctl list_queues | grep dlq

# Inspeccionar mensaje fallido
# Usar RabbitMQ Management UI: http://localhost:15672
```

### Health Checks

```bash
# Verificar estado del worker (cuando se implemente endpoint /health)
curl http://localhost:8080/health

# Verificar métricas (cuando se implemente)
curl http://localhost:8080/metrics
```

---

## 📦 Releases y Versionado

El proyecto sigue [Semantic Versioning](https://semver.org/):

- **MAJOR:** Cambios incompatibles en API/eventos
- **MINOR:** Nueva funcionalidad compatible
- **PATCH:** Bug fixes

### Proceso de Release

1. Actualizar `CHANGELOG.md`
2. Crear tag: `git tag -a v1.2.3 -m "Release v1.2.3"`
3. Push tag: `git push origin v1.2.3`
4. GitHub Actions construye y publica imagen Docker

---

## 🤝 Contribuir

### Proceso de Contribución

1. **Fork** del repositorio
2. **Branch** desde `develop`: `git checkout -b feature/mi-feature`
3. **Commits** con mensajes descriptivos
4. **Tests** para nueva funcionalidad
5. **Pull Request** a `develop`

### Convenciones de Código

```bash
# Antes de commit
make fmt      # Formatear código
make vet      # Análisis estático
make test     # Ejecutar tests
make lint     # Linter completo
```

### Estructura de Commits

```
feat(processor): add support for material reprocessing
fix(mongodb): handle connection timeout properly
docs(readme): update configuration section
test(repository): add unit tests for summary repository
refactor(bootstrap): simplify factory pattern
```

---

## 📚 Documentación Relacionada

### Documentación Técnica

- **[ARQUITECTURA.md](./ARQUITECTURA.md)** - Diagramas de arquitectura, capas, componentes
- **[BASE_DE_DATOS.md](./BASE_DE_DATOS.md)** - Esquemas PostgreSQL y MongoDB
- **[PROCESOS.md](./PROCESOS.md)** - Flujos de procesamiento y máquina de estados
- **[EVENTOS.md](./EVENTOS.md)** - Eventos RabbitMQ y estructura de mensajes
- **[CONFIGURACION.md](./CONFIGURACION.md)** - Variables de entorno y archivos YAML
- **[SERVICIOS.md](./SERVICIOS.md)** - Dependencias externas

### Documentación de Mejoras

- **[mejoras/CODIGO_DEPRECADO.md](./mejoras/CODIGO_DEPRECADO.md)** - Código a eliminar
- **[mejoras/REFACTORING.md](./mejoras/REFACTORING.md)** - Propuestas de refactorización
- **[mejoras/DEUDA_TECNICA.md](./mejoras/DEUDA_TECNICA.md)** - Deuda técnica identificada
- **[mejoras/ROADMAP.md](./mejoras/ROADMAP.md)** - Plan de mejoras futuras

---

## 📁 Referencias Externas

- [Go Documentation](https://golang.org/doc/)
- [RabbitMQ Tutorials](https://www.rabbitmq.com/getstarted.html)
- [MongoDB Go Driver](https://www.mongodb.com/docs/drivers/go/current/)
- [OpenAI API Reference](https://platform.openai.com/docs/api-reference)
- [AWS S3 Go SDK](https://aws.github.io/aws-sdk-go-v2/docs/)

---

> **Nota:** Esta documentación se actualiza regularmente. Si encuentras información desactualizada, por favor crea un issue o PR.
