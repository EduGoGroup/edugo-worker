# Product Requirements Document (PRD)
# spec-02: Worker - Procesamiento IA de Materiales

**Versión:** 1.0.0  
**Fecha:** 14 de Noviembre, 2025  
**Proyecto:** edugo-worker  
**Repositorio:** /Users/jhoanmedina/source/EduGo/repos-separados/edugo-worker

---

## 1. RESUMEN EJECUTIVO

### 1.1 Visión del Producto

El **Worker de Procesamiento IA** es un servicio asíncrono que consume materiales educativos (PDFs) publicados por docentes, los procesa usando IA (OpenAI GPT-4), y genera automáticamente:
- **Resúmenes educativos** estructurados (secciones, glosarios, preguntas de reflexión)
- **Quizzes automáticos** (evaluaciones de 5-10 preguntas)

Esto permite a los estudiantes obtener contenido procesado y evaluaciones sin intervención manual del docente.

### 1.2 Problema a Resolver

**Situación Actual:**
- Docentes publican PDFs sin procesamiento
- Estudiantes leen material "raw" sin estructura
- NO hay evaluaciones automáticas
- Docentes deben crear quizzes manualmente (tiempo alto)

**Problema:**
- Baja engagement estudiantil (sin resúmenes)
- Carga alta de docentes (crear evaluaciones)
- No hay personalización del contenido

### 1.3 Solución Propuesta

Worker asíncrono que:
1. Consume eventos de RabbitMQ (`material_uploaded`)
2. Descarga PDF de S3
3. Extrae y limpia texto
4. Llama OpenAI GPT-4 para generar resumen + quiz
5. Persiste en MongoDB
6. Actualiza PostgreSQL
7. Notifica docente

**Arquitectura:** Event-driven, asíncrono, escalable horizontalmente

---

## 2. OBJETIVOS DE NEGOCIO

### OBJ-1: Automatización de Procesamiento
**Métrica:** >95% de materiales procesados automáticamente  
**Objetivo:** Reducir intervención manual del docente  
**Prioridad:** P0 - CRÍTICO

### OBJ-2: Calidad de Resúmenes
**Métrica:** >4/5 en evaluación manual de calidad  
**Objetivo:** Resúmenes útiles y educativos  
**Prioridad:** P0 - CRÍTICO

### OBJ-3: Velocidad de Procesamiento
**Métrica:** <3 minutos promedio por material  
**Objetivo:** Feedback rápido a docentes  
**Prioridad:** P1 - ALTA

### OBJ-4: Costo Controlado
**Métrica:** <$0.20 USD por material procesado  
**Objetivo:** Operación sustentable económicamente  
**Prioridad:** P1 - ALTA

---

## 3. STAKEHOLDERS

| Stakeholder | Rol | Interés |
|-------------|-----|---------|
| **Docentes** | Publishers | Materiales procesados sin esfuerzo manual |
| **Estudiantes** | Consumers | Resúmenes y evaluaciones de calidad |
| **Administradores** | Ops | Worker estable, costos controlados |
| **OpenAI** | Provider | Uso correcto de API, rate limits respetados |

---

## 4. REQUERIMIENTOS FUNCIONALES

### RF-001: Consumir Eventos de RabbitMQ
**Prioridad:** MUST  
**Descripción:** Worker debe consumir eventos `material_uploaded` de cola `material_processing_high`

### RF-002: Extracción de Texto de PDFs
**Prioridad:** MUST  
**Descripción:** Extraer texto de PDFs usando pdftotext, con fallback a OCR si es escaneado

### RF-003: Generación de Resúmenes con OpenAI
**Prioridad:** MUST  
**Descripción:** Generar resumen estructurado (secciones, glosario, preguntas) usando GPT-4

### RF-004: Generación de Quizzes Automáticos
**Prioridad:** MUST  
**Descripción:** Generar 5-10 preguntas de opción múltiple con respuestas correctas

### RF-005: Persistencia en MongoDB
**Prioridad:** MUST  
**Descripción:** Guardar resúmenes en `material_summary` y quizzes en `material_assessment`

### RF-006: Actualización de PostgreSQL
**Prioridad:** MUST  
**Descripción:** Crear registros en `assessment` y `material_summary_link`

### RF-007: Manejo de Errores con Reintentos
**Prioridad:** MUST  
**Descripción:** Retry logic con backoff exponencial para errores transitorios

### RF-008: Dead Letter Queue
**Prioridad:** SHOULD  
**Descripción:** Enviar a DLQ después de 5 intentos fallidos

---

## 5. REQUERIMIENTOS NO FUNCIONALES

### RNF-001: Performance
- Procesamiento promedio: <3 minutos
- p95: <5 minutos
- p99: <10 minutos

### RNF-002: Disponibilidad
- Worker debe reiniciarse automáticamente si falla
- Múltiples instancias para alta disponibilidad

### RNF-003: Escalabilidad
- Horizontal: Múltiples workers consumiendo misma cola
- Vertical: Configuración de workers por CPU/RAM disponible

### RNF-004: Observabilidad
- Logs estructurados (JSON)
- Métricas Prometheus (materiales procesados, errores, latencia)
- Trazas de procesamiento completo

---

## 6. STACK TECNOLÓGICO

| Componente | Tecnología | Versión |
|------------|------------|---------|
| **Lenguaje** | Go | 1.21+ |
| **Message Queue** | RabbitMQ | 3.12+ |
| **IA/NLP** | OpenAI API | GPT-4 |
| **Base de Datos** | MongoDB | 7.0+ |
| **Base de Datos** | PostgreSQL | 15+ |
| **Storage** | S3 / MinIO | - |
| **PDF Processing** | pdftotext | - |
| **OCR** | Tesseract | 5.0+ |
| **Testing** | Testify + Testcontainers | - |

---

## 7. MÉTRICAS DE ÉXITO (KPIs)

### KPI-1: Tasa de Éxito
**Fórmula:** (Materiales procesados exitosamente / Total materiales) × 100  
**Objetivo:** >95%  
**Alerta si:** <90%

### KPI-2: Tiempo Promedio
**Fórmula:** AVG(tiempo_procesamiento)  
**Objetivo:** <3 minutos  
**Alerta si:** >5 minutos

### KPI-3: Costo por Material
**Fórmula:** (Costo total OpenAI / Materiales procesados)  
**Objetivo:** <$0.20 USD  
**Alerta si:** >$0.30 USD

### KPI-4: Calidad de Resúmenes
**Fórmula:** Rating manual de muestra aleatoria  
**Objetivo:** >4/5  
**Medición:** Mensual

---

**Generado con:** Claude Code  
**Estado:** PRD de Worker Completo
