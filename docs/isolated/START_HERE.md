# 🚀 START HERE - Worker (Procesamiento IA Asíncrono)

## ⭐ PUNTO DE ENTRADA ÚNICO

**Bienvenido a la documentación COMPLETA y AUTÓNOMA de edugo-worker.**

Esta carpeta contiene TODO lo necesario para implementar el sistema de procesamiento asíncrono sin depender de archivos externos.

---

## 📍 ¿Qué es edugo-worker?

**Consumer de mensajes** que procesa PDFs y genera contenido educativo con IA.

**Transporte:** RabbitMQ  
**Tecnología:** Go + Consumer Pattern + OpenAI API + MongoDB  
**Arquitectura:** Event-Driven

### Funcionalidades Principales
- ✅ **Procesamiento de PDFs** (lectura y extracción de contenido)
- ✅ **Generación de Resúmenes** (síntesis con OpenAI GPT-4)
- ✅ **Creación de Quizzes** (generación automática de cuestionarios)
- ✅ **Almacenamiento en MongoDB** (persistencia de resultados)
- ✅ **Manejo de Errores** (retry logic con exponential backoff)

---

## 🎯 ¿Qué Vamos a Implementar?

**Sistema Completo de Procesamiento Asíncrono:**

1. **Consumer de Materiales**
   - Escuchar eventos `material.created` en RabbitMQ
   - Descargar PDF de S3
   - Extraer texto con pdfium-go o similar

2. **Generador de Resúmenes**
   - Procesar texto con OpenAI
   - Crear resumen detallado
   - Guardar en MongoDB colección `material_summary`

3. **Generador de Quizzes**
   - Generar preguntas con opciones múltiples
   - Incluir respuesta correcta y explicación
   - Guardar en MongoDB colección `material_assessment`

4. **Manejo de Estado**
   - Actualizar `processing_status` en PostgreSQL
   - Publicar evento `material.processing_completed`
   - Registrar logs de ejecución

---

## 📂 Estructura de Esta Carpeta

```
worker/
│
├── START_HERE.md                ⭐ Este archivo - LEER PRIMERO
├── EXECUTION_PLAN.md            Plan paso a paso de ejecución
│
├── 01-Context/                  Contexto del proyecto
│   ├── PROJECT_OVERVIEW.md      Overview detallado
│   ├── ECOSYSTEM_CONTEXT.md     Cómo encaja en el ecosistema
│   ├── DEPENDENCIES.md          Qué necesita de otros proyectos
│   └── TECH_STACK.md            Stack tecnológico
│
├── 02-Requirements/             Requisitos funcionales y técnicos
│   ├── PRD.md                   Product Requirements Document
│   ├── FUNCTIONAL_SPECS.md      Especificaciones funcionales
│   ├── TECHNICAL_SPECS.md       Especificaciones técnicas
│   └── ACCEPTANCE_CRITERIA.md   Criterios de aceptación
│
├── 03-Design/                   Diseño arquitectónico
│   ├── ARCHITECTURE.md          Arquitectura event-driven
│   ├── MESSAGE_FLOW.md          Flujo de mensajes
│   ├── DATA_MODEL.md            Modelo de datos MongoDB
│   └── ERROR_HANDLING.md        Estrategia de errores
│
├── 04-Implementation/           Implementación (6 sprints)
│   ├── Sprint-01-Setup/         Setup y configuración
│   ├── Sprint-02-RabbitMQ/      Consumer RabbitMQ
│   ├── Sprint-03-PDF-Process/   Procesamiento de PDFs
│   ├── Sprint-04-OpenAI-Integr/ Integración OpenAI
│   ├── Sprint-05-Storage/       Almacenamiento MongoDB + S3
│   └── Sprint-06-Testing-Deploy/ Testing y deployment
│
├── 05-Testing/                  Estrategia de testing
│   ├── TEST_STRATEGY.md
│   ├── TEST_CASES.md
│   └── COVERAGE_REPORT.md
│
├── 06-Deployment/               Deployment y monitoreo
│   ├── DEPLOYMENT_GUIDE.md
│   ├── SCALING.md
│   └── MONITORING.md
│
└── PROGRESS.json                Tracking de progreso (JSON)
```

---

## 🚦 Flujo de Inicio Rápido

### Paso 1: Leer Contexto (15 min)
```bash
# Entender qué es este proyecto y cómo encaja
cat 01-Context/PROJECT_OVERVIEW.md
cat 01-Context/ECOSYSTEM_CONTEXT.md
cat 01-Context/DEPENDENCIES.md
```

### Paso 2: Revisar Requisitos (30 min)
```bash
# Entender QUÉ vamos a construir
cat 02-Requirements/PRD.md
cat 02-Requirements/FUNCTIONAL_SPECS.md
cat 02-Requirements/ACCEPTANCE_CRITERIA.md
```

### Paso 3: Estudiar Arquitectura (45 min)
```bash
# Entender CÓMO lo vamos a construir
cat 03-Design/ARCHITECTURE.md
cat 03-Design/MESSAGE_FLOW.md
cat 03-Design/DATA_MODEL.md
```

### Paso 4: Ejecutar Plan (Ver EXECUTION_PLAN.md)
```bash
# Plan detallado de implementación
cat EXECUTION_PLAN.md
```

### Paso 5: Implementar Sprint por Sprint (18 días estimados)
```bash
cd 04-Implementation/Sprint-01-Setup/
cat README.md
cat TASKS.md
# ... ejecutar tareas ...
# Repetir para cada sprint
```

---

## 🔗 Dependencias Externas

Este proyecto **NECESITA** de otros componentes del ecosistema:

### 1. edugo-infrastructure v0.1.1 (NUEVO)
**Versión requerida:** v0.1.1  
**Qué usar:**
- `schemas/events/material.uploaded.json` - Validar eventos que consume
- `schemas/events/assessment.generated.json` - Validar eventos que publica
- `schemas/events/summary.completed.json` - Validar eventos que publica

**Estado:** ✅ COMPLETADO (96%)

**Integración:**
```go
import "github.com/EduGoGroup/edugo-infrastructure/schemas"

// Validar evento recibido antes de procesar
func ConsumeMaterialEvent(msg []byte) error {
    if err := schemas.Validate("material.uploaded", msg); err != nil {
        logger.Error("Invalid event received", err)
        return err // Rechazar mensaje
    }
    // Procesar evento válido...
}

// Validar evento antes de publicar
func PublishAssessmentEvent(assessment Assessment) error {
    event := buildEvent(assessment)
    
    if err := schemas.Validate("assessment.generated", event); err != nil {
        return fmt.Errorf("invalid event: %w", err)
    }
    
    return publisher.Publish("assessment-events", "assessment.generated", event)
}
```

### 2. edugo-shared v0.7.0 (FROZEN)
**Versión requerida:** v0.7.0 (FROZEN hasta post-MVP)  
**❌ NO USAR:** v1.3.0+ (no existen)

**Módulos usados:**
- `config` - Configuración multi-ambiente
- `database/postgres` - Conexiones PostgreSQL
- `database/mongodb` - Conexiones MongoDB
- `logger` - Logging estructurado
- `messaging/rabbit` - RabbitMQ consumer/publisher con DLQ (NUEVO en v0.7.0)
- `evaluation` - Modelos de evaluación (NUEVO en v0.7.0)

**Estado:** ✅ COMPLETADO - 12 módulos publicados

**Novedades en v0.7.0:**
- **messaging/rabbit con DLQ:** Dead Letter Queue automático para retry
- **evaluation module:** Modelos compartidos con api-mobile

### 3. RabbitMQ 3.12+
**Uso:** Message broker principal  
**Exchanges:**
- `material-events` (topic exchange)

**Queues:**
- `material.processing` (worker consume eventos)
- `material.processed` (worker publica resultados)

**Flujo:** API publica → Worker consume → Procesa → Publica resultado

### 4. PostgreSQL 15+
**Uso:** Actualizar estado de procesamiento  
**Tablas requeridas:**
- `materials` (modificar campo `processing_status`)

**Cambios:** Agregar columna `processing_completed_at` (timestamp)

### 5. MongoDB 7.0+
**Uso:** Almacenamiento de resúmenes y quizzes generados  
**Colecciones:**
- `material_summary` (resúmenes de textos)
- `material_assessment` (quizzes generados)

**Índices:** `material_id`, `created_at`

### 6. OpenAI API
**Modelo recomendado:** gpt-4-turbo-preview  
**Alternativa:** gpt-3.5-turbo (más barato, menor calidad)

**Uso:** Generación de resúmenes y preguntas

**⚠️ COSTOS ESTIMADOS POR MATERIAL:**

| Componente | Tokens | Costo gpt-4-turbo | Costo gpt-3.5-turbo |
|------------|--------|-------------------|---------------------|
| Extracción PDF | ~5,000 (input) | $0.050 | $0.0025 |
| Generación resumen | ~2,000 (output) | $0.060 | $0.003 |
| Generación quiz (10 preguntas) | ~3,000 (output) | $0.090 | $0.0045 |
| **Total por material** | ~10,000 | **~$0.20** | **~$0.01** |

**Proyección mensual:**
- 100 materiales/mes → $20 (gpt-4) o $1 (gpt-3.5)
- 500 materiales/mes → $100 (gpt-4) o $5 (gpt-3.5)
- 1,000 materiales/mes → $200 (gpt-4) o $10 (gpt-3.5)

**Rate Limits:**
- gpt-4-turbo: 500 RPM (requests per minute)
- gpt-3.5-turbo: 3,500 RPM

**SLA OpenAI:**
- Uptime: 99.9%
- P95 latency: ~18 segundos (gpt-4)
- P95 latency: ~5 segundos (gpt-3.5)

**Recomendación:**
- **Desarrollo/Testing:** gpt-3.5-turbo (barato)
- **Producción:** gpt-4-turbo (mejor calidad)
- **Implementar:** Caché de resultados para evitar regenerar

### 7. AWS S3 (Almacenamiento de PDFs)
**Bucket:** `edugo-materials` (o similar)  
**Uso:** Descargar PDFs originales para procesamiento  
**Permisos:** ReadOnly

---

## ⚙️ Configuración Requerida

### Variables de Entorno
```bash
# RabbitMQ
RABBITMQ_URL=amqp://guest:guest@localhost:5672/
RABBITMQ_EXCHANGE=material-events
RABBITMQ_QUEUE=material.processing

# PostgreSQL
DATABASE_URL=postgres://user:pass@localhost:5432/edugo_dev?sslmode=disable

# MongoDB
MONGO_URI=mongodb://localhost:27017
MONGO_DATABASE=edugo_dev

# OpenAI
OPENAI_API_KEY=sk-...
OPENAI_MODEL=gpt-4-turbo
OPENAI_TEMPERATURE=0.7

# AWS S3
AWS_ACCESS_KEY_ID=...
AWS_SECRET_ACCESS_KEY=...
AWS_REGION=us-east-1
S3_BUCKET=edugo-materials

# Worker
ENVIRONMENT=local  # local, dev, qa, prod
LOG_LEVEL=debug
WORKER_CONCURRENCY=2  # Número de goroutines procesando simultáneamente
RETRY_MAX_ATTEMPTS=3
RETRY_INITIAL_BACKOFF=1s  # Backoff exponencial
```

### Prerequisitos de Sistema
```bash
# Go 1.21+
go version

# RabbitMQ 3.12+
rabbitmq-server --version

# PostgreSQL 15+
psql --version

# MongoDB 7.0+
mongosh --version

# Docker (recomendado)
docker --version
```

---

## 📋 Plan de Implementación

Ver archivo **EXECUTION_PLAN.md** para el plan detallado.

Resumen:
1. **Sprint 01:** Setup de proyecto y configuración (2 días)
2. **Sprint 02:** Consumer RabbitMQ (3 días)
3. **Sprint 03:** Procesamiento de PDFs (3 días)
4. **Sprint 04:** Integración OpenAI (4 días)
5. **Sprint 05:** Almacenamiento (MongoDB + S3) (2 días)
6. **Sprint 06:** Testing y deployment (3 días)

**Total estimado:** 17-20 días laborables

---

## ✅ Checklist Pre-Implementación

Antes de comenzar Sprint 01, verifica:

### Ambiente de Desarrollo
- [ ] Go 1.21+ instalado
- [ ] RabbitMQ 3.12+ corriendo (Management UI en localhost:15672)
- [ ] PostgreSQL 15+ corriendo
- [ ] MongoDB 7.0+ corriendo
- [ ] Repositorio edugo-worker clonado
- [ ] Rama feature creada: `git checkout -b feature/ia-processor`

### Dependencias
- [ ] edugo-shared v1.3.0 publicado en GitHub
- [ ] Tabla `materials` existe en PostgreSQL
- [ ] RabbitMQ exchange `material-events` creado
- [ ] OpenAI API key obtenida y testeada

### Configuración
- [ ] Archivo `.env.local` creado con variables necesarias
- [ ] Conexión a RabbitMQ verificada
- [ ] Conexión a PostgreSQL verificada
- [ ] Conexión a MongoDB verificada
- [ ] OpenAI API key funciona (test simple)

### Opcional (para testing completo)
- [ ] AWS S3 bucket creado y accesible
- [ ] Al menos 1 PDF de prueba cargado en S3
- [ ] edugo-api-mobile publicando eventos (para testing end-to-end)

---

## 🎯 Resultado Esperado

Al completar los 6 sprints, tendrás:

### Funcionalidades
- ✅ Consumer RabbitMQ funcional
- ✅ Procesamiento de PDFs automatizado
- ✅ Integración OpenAI completa
- ✅ Generación de resúmenes de calidad
- ✅ Generación de quizzes variados
- ✅ Manejo de errores con reintentos

### Calidad
- ✅ Cobertura de tests >80%
- ✅ Tests de integración con Testcontainers
- ✅ Logs estructurados y trazables
- ✅ CI/CD funcionando (GitHub Actions)

### Arquitectura
- ✅ Patrón event-driven implementado
- ✅ Separación de concerns (handlers, services)
- ✅ Manejo robusto de errores
- ✅ Código escalable y mantenible

---

## 📞 Soporte y Recursos

### Dentro de Esta Carpeta
- **Dudas de arquitectura:** `03-Design/ARCHITECTURE.md`
- **Dudas de flujo de mensajes:** `03-Design/MESSAGE_FLOW.md`
- **Dudas de requisitos:** `02-Requirements/`
- **Dudas de implementación:** `04-Implementation/Sprint-XX/TASKS.md`
- **Dudas de testing:** `05-Testing/TEST_STRATEGY.md`

### Contexto del Ecosistema
- **Cómo encaja este proyecto:** `01-Context/ECOSYSTEM_CONTEXT.md`
- **Qué depende de qué:** `01-Context/DEPENDENCIES.md`
- **Stack tecnológico:** `01-Context/TECH_STACK.md`

---

## 🚀 Comenzar AHORA

```bash
# 1. Lee el overview del proyecto
cat 01-Context/PROJECT_OVERVIEW.md

# 2. Lee el plan de ejecución
cat EXECUTION_PLAN.md

# 3. Inicia Sprint 01
cd 04-Implementation/Sprint-01-Setup/
cat README.md
cat TASKS.md

# 4. Ejecuta las tareas paso a paso
# ... sigue las instrucciones de TASKS.md
```

---

**Última actualización:** 15 de Noviembre, 2025  
**Generado con:** Claude Code  
**Proyecto:** edugo-worker - Procesamiento IA Asíncrono  
**Tipo de documentación:** Aislada y autónoma

---

## 🎓 Filosofía de Esta Documentación

> **"Todo lo que necesitas está aquí. No necesitas buscar en archivos externos. Esta carpeta es autónoma."**

**Si encuentras que falta algo, es un bug en la documentación. Repórtalo.**

---

¡Éxito en tu implementación! 🚀
