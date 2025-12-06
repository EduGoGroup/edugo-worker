# Arquitectura
# spec-02: Worker - Procesamiento IA

**Versión:** 1.0.0  
**Fecha:** 14 de Noviembre, 2025

---

## 1. ARQUITECTURA EVENT-DRIVEN

```
┌─────────────┐
│ API Mobile  │ Publica material
└──────┬──────┘
       │ Publica evento
       ▼
┌─────────────┐
│  RabbitMQ   │ Cola: material_processing_high
└──────┬──────┘
       │ Consume evento
       ▼
┌─────────────┐
│   Worker    │ Procesamiento asíncrono
│             │
│ ┌─────────┐ │
│ │Consumer │ │ Recibe evento
│ └────┬────┘ │
│      │      │
│ ┌────▼────┐ │
│ │PDF Proc │ │ Descarga S3 → Extrae texto
│ └────┬────┘ │
│      │      │
│ ┌────▼────┐ │
│ │AI Service│ │ OpenAI GPT-4 → Resumen + Quiz
│ └────┬────┘ │
│      │      │
│ ┌────▼────┐ │
│ │Repository│ │ Guarda en MongoDB + PostgreSQL
│ └─────────┘ │
└─────────────┘
       │
       ├─────────────────┐
       ▼                 ▼
┌─────────────┐   ┌─────────────┐
│  MongoDB    │   │ PostgreSQL  │
│ Collections │   │   Tables    │
│ - summary   │   │ - assessment│
│ - assessment│   │ - link      │
└─────────────┘   └─────────────┘
```

---

## 2. COMPONENTES

### Consumer (main.go)
- Conecta a RabbitMQ
- Declara cola y exchange
- Inicia N goroutines de procesamiento
- Maneja graceful shutdown

### PDFProcessor
- Descarga PDF de S3
- Ejecuta pdftotext
- Limpia y normaliza texto
- Validaciones de longitud mínima

### AIService
- Construye prompts para OpenAI
- Llama API con retry logic
- Parsea y valida JSON responses
- Manejo de rate limits

### Repository
- MongoDB: Upsert de resúmenes y quizzes
- PostgreSQL: Insert en assessment
- Transacciones donde sea necesario

---

## 3. FLUJO DE PROCESAMIENTO

```
1. Recibir evento RabbitMQ
2. Descargar PDF de S3
3. Extraer texto (pdftotext)
4. Validar texto (>500 palabras)
5. Generar resumen (OpenAI)
6. Generar quiz (OpenAI)
7. Guardar en MongoDB (2 collections)
8. Actualizar PostgreSQL (2 tablas)
9. ACK mensaje RabbitMQ
10. Log métricas
```

**Si falla algún paso:** Ejecutar retry logic según tipo de error

---

## 4. ESCALABILIDAD

### Horizontal Scaling
- Múltiples workers consumiendo misma cola
- RabbitMQ distribuye mensajes (round-robin)
- Sin estado compartido entre workers

### Configuración por Ambiente
```go
// Producción: 3 workers
WORKER_INSTANCES=3

// Development: 1 worker
WORKER_INSTANCES=1
```

---

**Generado con:** Claude Code
