// seed_data.js
// Script de datos de prueba para desarrollo y testing
//
// Proyecto: edugo-worker
// Sprint: Sprint-01 Fase 1 - Auditoría y Diseño de Schemas MongoDB
// Autor: Claude Code Web
// Fecha: 2025-11-18
//
// EJECUCIÓN:
//   Con mongosh: mongosh "mongodb://localhost:27017/edugo" --username <user> --password <pass> < scripts/mongodb/seed_data.js
//   O directamente: mongosh "mongodb://localhost:27017/edugo" --eval "load('scripts/mongodb/seed_data.js')"
//
// IMPORTANTE: Este script inserta datos de ejemplo. Solo ejecutar en desarrollo/testing.

print("\n╔════════════════════════════════════════════════════════════════╗");
print("║                                                                ║");
print("║           MongoDB Seed Data - edugo-worker                    ║");
print("║                  (Development/Testing)                         ║");
print("║                                                                ║");
print("╚════════════════════════════════════════════════════════════════╝\n");

// Seleccionar base de datos
const dbName = "edugo";
db = db.getSiblingDB(dbName);

print(`📊 Using database: ${dbName}\n`);

// ═══════════════════════════════════════════════════════════════════
// CONFIRMACIÓN DE SEGURIDAD
// ═══════════════════════════════════════════════════════════════════

print("⚠️  WARNING: This script will insert test data into the database.");
print("⚠️  Only run this in development or testing environments!\n");

// ═══════════════════════════════════════════════════════════════════
// LIMPIAR DATOS EXISTENTES (OPCIONAL)
// ═══════════════════════════════════════════════════════════════════

print("🧹 Cleaning existing test data...\n");

// Opcional: Descomentar para limpiar datos previos
// db.material_summary.deleteMany({});
// db.material_assessment.deleteMany({});
// db.material_event.deleteMany({});

print("✅ Cleanup completed (skipped - commented out)\n");

// ═══════════════════════════════════════════════════════════════════
// DATOS DE PRUEBA
// ═══════════════════════════════════════════════════════════════════

// UUIDs de ejemplo para materiales
const materialIds = [
  "550e8400-e29b-41d4-a716-446655440000", // Material 1: MongoDB Intro
  "650e8400-e29b-41d4-a716-446655440001", // Material 2: Clean Architecture
  "750e8400-e29b-41d4-a716-446655440002", // Material 3: Go Best Practices
  "850e8400-e29b-41d4-a716-446655440003", // Material 4: Microservices
  "950e8400-e29b-41d4-a716-446655440004"  // Material 5: Design Patterns
];

// UUIDs de ejemplo para usuarios
const userIds = [
  "user-123e4567-e89b-12d3-a456-426614174000",
  "user-223e4567-e89b-12d3-a456-426614174001",
  "user-323e4567-e89b-12d3-a456-426614174002"
];

// ═══════════════════════════════════════════════════════════════════
// 1. SEED: material_summary
// ═══════════════════════════════════════════════════════════════════

print("┌──────────────────────────────────────────────────────────────┐");
print("│ 1. Seeding: material_summary                                 │");
print("└──────────────────────────────────────────────────────────────┘\n");

const summaries = [
  {
    material_id: materialIds[0],
    summary: "Este material introduce los conceptos fundamentales de MongoDB, una base de datos NoSQL orientada a documentos. Se exploran las diferencias clave con bases de datos relacionales tradicionales, destacando la flexibilidad del modelo de documentos BSON y las ventajas de escalabilidad horizontal. El contenido cubre operaciones CRUD básicas, diseño de schemas efectivos, y casos de uso apropiados para aplicaciones modernas que requieren alta concurrencia y datos semi-estructurados.",
    key_points: [
      "MongoDB es una base de datos NoSQL orientada a documentos que usa BSON",
      "Ofrece escalabilidad horizontal mediante sharding automático",
      "No requiere schema fijo, permitiendo evolución ágil del modelo de datos",
      "Soporta transacciones ACID multi-documento desde la versión 4.0",
      "Ideal para aplicaciones con alta concurrencia y datos semi-estructurados",
      "Incluye aggregation pipeline potente para análisis de datos complejos"
    ],
    language: "es",
    word_count: 87,
    version: 1,
    ai_model: "gpt-4",
    processing_time_ms: 2340,
    token_usage: {
      prompt_tokens: 850,
      completion_tokens: 120,
      total_tokens: 970
    },
    metadata: {
      source_length: 4500,
      has_images: true
    },
    created_at: new Date("2025-11-18T10:30:00Z"),
    updated_at: new Date("2025-11-18T10:30:00Z")
  },
  {
    material_id: materialIds[1],
    summary: "Clean Architecture es un patrón arquitectónico que promueve la separación de responsabilidades mediante capas bien definidas: Dominio, Aplicación e Infraestructura. El núcleo de negocio (Domain) permanece independiente de frameworks, bases de datos y detalles de implementación. Esta arquitectura facilita el testing, mejora la mantenibilidad y permite evolucionar componentes de forma independiente sin afectar la lógica de negocio central.",
    key_points: [
      "Separación estricta entre lógica de negocio e infraestructura",
      "Dependency Inversion: las dependencias apuntan hacia el dominio",
      "Facilita testing mediante inyección de dependencias",
      "Reduce acoplamiento y mejora cohesión del código",
      "Permite cambiar frameworks sin afectar el core de negocio"
    ],
    language: "es",
    word_count: 75,
    version: 1,
    ai_model: "gpt-4-turbo",
    processing_time_ms: 1890,
    token_usage: {
      prompt_tokens: 720,
      completion_tokens: 105,
      total_tokens: 825
    },
    metadata: {
      source_length: 3800,
      has_images: false
    },
    created_at: new Date("2025-11-18T11:00:00Z"),
    updated_at: new Date("2025-11-18T11:00:00Z")
  },
  {
    material_id: materialIds[2],
    summary: "This material covers best practices for Go development, including effective error handling patterns, proper use of goroutines and channels, and idiomatic code structures. Key topics include context propagation for cancellation, table-driven tests, interface design principles, and common pitfalls to avoid. The guide emphasizes simplicity, readability, and leveraging Go's unique features for building robust concurrent applications.",
    key_points: [
      "Always handle errors explicitly, never ignore them",
      "Use context.Context for cancellation and deadlines",
      "Prefer composition over inheritance via small interfaces",
      "Table-driven tests for comprehensive test coverage",
      "Avoid goroutine leaks by ensuring proper cleanup",
      "Follow effective naming conventions and package structure"
    ],
    language: "en",
    word_count: 82,
    version: 1,
    ai_model: "gpt-4o",
    processing_time_ms: 2100,
    token_usage: {
      prompt_tokens: 950,
      completion_tokens: 115,
      total_tokens: 1065
    },
    metadata: {
      source_length: 5200,
      has_images: true
    },
    created_at: new Date("2025-11-18T12:00:00Z"),
    updated_at: new Date("2025-11-18T12:00:00Z")
  },
  {
    material_id: materialIds[3],
    summary: "Los microservicios son un estilo arquitectónico que estructura una aplicación como una colección de servicios pequeños, autónomos y débilmente acoplados. Cada servicio se enfoca en una capacidad de negocio específica, puede desplegarse independientemente y comunicarse mediante APIs bien definidas. Esta aproximación ofrece escalabilidad granular, resiliencia mejorada y permite equipos autónomos trabajando en paralelo.",
    key_points: [
      "Servicios pequeños, autónomos y enfocados en una sola responsabilidad",
      "Despliegue independiente permite releases frecuentes",
      "Comunicación mediante APIs REST, gRPC o mensajería asíncrona",
      "Base de datos por servicio para evitar acoplamiento de datos",
      "Requiere infraestructura robusta (service mesh, observability)",
      "Trade-off: mayor complejidad operacional vs escalabilidad"
    ],
    language: "es",
    word_count: 92,
    version: 1,
    ai_model: "gpt-4",
    processing_time_ms: 2650,
    token_usage: {
      prompt_tokens: 1100,
      completion_tokens: 135,
      total_tokens: 1235
    },
    metadata: {
      source_length: 6000,
      has_images: true
    },
    created_at: new Date("2025-11-18T13:00:00Z"),
    updated_at: new Date("2025-11-18T13:00:00Z")
  },
  {
    material_id: materialIds[4],
    summary: "Design Patterns são soluções reutilizáveis para problemas comuns no desenvolvimento de software. Este material cobre os padrões clássicos do Gang of Four: criacionais (Singleton, Factory, Builder), estruturais (Adapter, Decorator, Facade) e comportamentais (Observer, Strategy, Command). Cada padrão é explicado com exemplos práticos, indicando quando aplicá-los e quais trade-offs considerar.",
    key_points: [
      "Padrões criacionais controlam a criação de objetos",
      "Padrões estruturais facilitam composição de classes e objetos",
      "Padrões comportamentais gerenciam algoritmos e responsabilidades",
      "Singleton garante instância única mas dificulta testes",
      "Strategy permite trocar algoritmos dinamicamente",
      "Observer implementa comunicação desacoplada entre objetos"
    ],
    language: "pt",
    word_count: 78,
    version: 1,
    ai_model: "gpt-4-turbo",
    processing_time_ms: 2200,
    token_usage: {
      prompt_tokens: 880,
      completion_tokens: 110,
      total_tokens: 990
    },
    metadata: {
      source_length: 4200,
      has_images: false
    },
    created_at: new Date("2025-11-18T14:00:00Z"),
    updated_at: new Date("2025-11-18T14:00:00Z")
  }
];

try {
  const summaryResult = db.material_summary.insertMany(summaries);
  print(`✅ Inserted ${summaryResult.insertedIds.length} material summaries`);
} catch (error) {
  print(`❌ Error inserting summaries: ${error}`);
}

print("");

// ═══════════════════════════════════════════════════════════════════
// 2. SEED: material_assessment
// ═══════════════════════════════════════════════════════════════════

print("┌──────────────────────────────────────────────────────────────┐");
print("│ 2. Seeding: material_assessment                              │");
print("└──────────────────────────────────────────────────────────────┘\n");

const assessments = [
  {
    material_id: materialIds[0],
    title: "Quiz: Fundamentos de MongoDB",
    description: "Evaluación sobre conceptos básicos de MongoDB y bases de datos NoSQL",
    questions: [
      {
        id: "q-f3e4d5c6-b7a8-4c3d-9e2f-1a0b9c8d7e6f",
        text: "¿Qué es MongoDB?",
        type: "multiple_choice",
        difficulty: "easy",
        points: 5,
        options: [
          {
            id: "opt-1a",
            text: "Una base de datos relacional como MySQL",
            is_correct: false,
            order: 1
          },
          {
            id: "opt-2a",
            text: "Una base de datos NoSQL orientada a documentos",
            is_correct: true,
            order: 2
          },
          {
            id: "opt-3a",
            text: "Un lenguaje de programación",
            is_correct: false,
            order: 3
          },
          {
            id: "opt-4a",
            text: "Un sistema operativo",
            is_correct: false,
            order: 4
          }
        ],
        explanation: "MongoDB es una base de datos NoSQL que almacena datos en documentos BSON (Binary JSON), permitiendo flexibilidad en el schema y escalabilidad horizontal.",
        bloom_taxonomy_level: "remember",
        order: 1
      },
      {
        id: "q-a1b2c3d4-e5f6-4a7b-8c9d-0e1f2a3b4c5d",
        text: "MongoDB soporta transacciones ACID multi-documento desde la versión 4.0",
        type: "true_false",
        difficulty: "medium",
        points: 5,
        correct_answer: "true",
        explanation: "A partir de MongoDB 4.0, se introdujo soporte completo para transacciones multi-documento ACID, similar a bases de datos relacionales.",
        bloom_taxonomy_level: "understand",
        order: 2
      },
      {
        id: "q-b2c3d4e5-f6a7-4b8c-9d0e-1f2a3b4c5d6e",
        text: "Explica la diferencia entre sharding y replicación en MongoDB",
        type: "open",
        difficulty: "hard",
        points: 10,
        explanation: "Sharding es la distribución horizontal de datos entre múltiples servidores para escalar capacidad y throughput, mientras que replicación crea copias redundantes de los datos para alta disponibilidad y tolerancia a fallos.",
        bloom_taxonomy_level: "analyze",
        order: 3
      },
      {
        id: "q-c3d4e5f6-a7b8-4c9d-0e1f-2a3b4c5d6e7f",
        text: "¿Cuál es el tamaño máximo de un documento BSON en MongoDB?",
        type: "multiple_choice",
        difficulty: "medium",
        points: 5,
        options: [
          {
            id: "opt-1b",
            text: "4 MB",
            is_correct: false,
            order: 1
          },
          {
            id: "opt-2b",
            text: "16 MB",
            is_correct: true,
            order: 2
          },
          {
            id: "opt-3b",
            text: "32 MB",
            is_correct: false,
            order: 3
          },
          {
            id: "opt-4b",
            text: "No hay límite",
            is_correct: false,
            order: 4
          }
        ],
        explanation: "El tamaño máximo de un documento BSON en MongoDB es 16 MB. Para datos más grandes, se debe usar GridFS.",
        bloom_taxonomy_level: "remember",
        order: 4
      },
      {
        id: "q-d4e5f6a7-b8c9-4d0e-1f2a-3b4c5d6e7f8a",
        text: "Los índices en MongoDB mejoran la performance de lecturas pero pueden ralentizar escrituras",
        type: "true_false",
        difficulty: "easy",
        points: 5,
        correct_answer: "true",
        explanation: "Los índices aceleran las queries (lecturas) al permitir búsquedas eficientes, pero cada índice debe actualizarse en cada operación de escritura, lo que genera overhead.",
        bloom_taxonomy_level: "understand",
        order: 5
      }
    ],
    total_questions: 5,
    total_points: 30,
    passing_score: 18,
    time_limit_minutes: 20,
    difficulty_distribution: {
      easy: 2,
      medium: 2,
      hard: 1
    },
    version: 1,
    ai_model: "gpt-4",
    processing_time_ms: 3500,
    created_at: new Date("2025-11-18T10:30:05Z"),
    updated_at: new Date("2025-11-18T10:30:05Z")
  },
  {
    material_id: materialIds[1],
    title: "Quiz: Clean Architecture Principles",
    description: "Evaluación sobre principios y conceptos de Clean Architecture",
    questions: [
      {
        id: "q-e5f6a7b8-c9d0-4e1f-2a3b-4c5d6e7f8a9b",
        text: "En Clean Architecture, ¿hacia dónde deben apuntar las dependencias?",
        type: "multiple_choice",
        difficulty: "medium",
        points: 10,
        options: [
          {
            id: "opt-1c",
            text: "Hacia la infraestructura",
            is_correct: false,
            order: 1
          },
          {
            id: "opt-2c",
            text: "Hacia el dominio (núcleo de negocio)",
            is_correct: true,
            order: 2
          },
          {
            id: "opt-3c",
            text: "Hacia la capa de presentación",
            is_correct: false,
            order: 3
          },
          {
            id: "opt-4c",
            text: "No importa la dirección",
            is_correct: false,
            order: 4
          }
        ],
        explanation: "El Principio de Inversión de Dependencias establece que las dependencias deben apuntar hacia el dominio (inner layers), no hacia detalles de implementación (outer layers).",
        bloom_taxonomy_level: "understand",
        order: 1
      },
      {
        id: "q-f6a7b8c9-d0e1-4f2a-3b4c-5d6e7f8a9b0c",
        text: "Clean Architecture hace que el testing sea más fácil porque permite inyectar mocks para dependencias externas",
        type: "true_false",
        difficulty: "easy",
        points: 5,
        correct_answer: "true",
        explanation: "Al depender de abstracciones (interfaces) en lugar de implementaciones concretas, se pueden inyectar fácilmente mocks o stubs para testing unitario.",
        bloom_taxonomy_level: "apply",
        order: 2
      },
      {
        id: "q-a7b8c9d0-e1f2-4a3b-4c5d-6e7f8a9b0c1d",
        text: "Describe las tres capas principales de Clean Architecture y sus responsabilidades",
        type: "open",
        difficulty: "hard",
        points: 15,
        explanation: "1) Dominio: entidades, value objects y lógica de negocio pura. 2) Aplicación: casos de uso y orquestación. 3) Infraestructura: frameworks, bases de datos, APIs externas y detalles de implementación.",
        bloom_taxonomy_level: "analyze",
        order: 3
      }
    ],
    total_questions: 3,
    total_points: 30,
    passing_score: 20,
    time_limit_minutes: 15,
    difficulty_distribution: {
      easy: 1,
      medium: 1,
      hard: 1
    },
    version: 1,
    ai_model: "gpt-4-turbo",
    processing_time_ms: 2800,
    created_at: new Date("2025-11-18T11:00:05Z"),
    updated_at: new Date("2025-11-18T11:00:05Z")
  },
  {
    material_id: materialIds[2],
    title: "Quiz: Go Best Practices",
    description: "Assessment on idiomatic Go code and common patterns",
    questions: [
      {
        id: "q-b8c9d0e1-f2a3-4b4c-5d6e-7f8a9b0c1d2e",
        text: "Which statement about error handling in Go is correct?",
        type: "multiple_choice",
        difficulty: "easy",
        points: 5,
        options: [
          {
            id: "opt-1d",
            text: "Errors should be ignored if they are unlikely to occur",
            is_correct: false,
            order: 1
          },
          {
            id: "opt-2d",
            text: "Always handle errors explicitly, never ignore them",
            is_correct: true,
            order: 2
          },
          {
            id: "opt-3d",
            text: "Use panic for all error conditions",
            is_correct: false,
            order: 3
          },
          {
            id: "opt-4d",
            text: "Errors are optional in Go",
            is_correct: false,
            order: 4
          }
        ],
        explanation: "Go philosophy emphasizes explicit error handling. Every error should be checked and handled appropriately, never silently ignored.",
        bloom_taxonomy_level: "remember",
        order: 1
      },
      {
        id: "q-c9d0e1f2-a3b4-4c5d-6e7f-8a9b0c1d2e3f",
        text: "context.Context should be passed as the first parameter to functions",
        type: "true_false",
        difficulty: "medium",
        points: 5,
        correct_answer: "true",
        explanation: "By convention in Go, context.Context is passed as the first parameter to functions that need it, typically named 'ctx'.",
        bloom_taxonomy_level: "understand",
        order: 2
      }
    ],
    total_questions: 2,
    total_points: 10,
    passing_score: 7,
    time_limit_minutes: 10,
    difficulty_distribution: {
      easy: 1,
      medium: 1,
      hard: 0
    },
    version: 1,
    ai_model: "gpt-4o",
    processing_time_ms: 2100,
    created_at: new Date("2025-11-18T12:00:05Z"),
    updated_at: new Date("2025-11-18T12:00:05Z")
  }
];

try {
  const assessmentResult = db.material_assessment.insertMany(assessments);
  print(`✅ Inserted ${assessmentResult.insertedIds.length} material assessments`);
} catch (error) {
  print(`❌ Error inserting assessments: ${error}`);
}

print("");

// ═══════════════════════════════════════════════════════════════════
// 3. SEED: material_event
// ═══════════════════════════════════════════════════════════════════

print("┌──────────────────────────────────────────────────────────────┐");
print("│ 3. Seeding: material_event                                   │");
print("└──────────────────────────────────────────────────────────────┘\n");

const events = [
  // Eventos exitosos
  {
    event_type: "material_uploaded",
    event_id: "evt-550e8400-e29b-41d4-a716-446655440001",
    material_id: materialIds[0],
    user_id: userIds[0],
    payload: {
      event_type: "material_uploaded",
      material_id: materialIds[0],
      author_id: userIds[0],
      s3_key: "materials/2025/11/18/mongodb-intro.pdf",
      preferred_language: "es",
      timestamp: "2025-11-18T10:29:45Z"
    },
    status: "completed",
    processing_time_ms: 5840,
    processed_at: new Date("2025-11-18T10:30:06Z"),
    created_at: new Date("2025-11-18T10:30:00Z")
  },
  {
    event_type: "material_uploaded",
    event_id: "evt-650e8400-e29b-41d4-a716-446655440002",
    material_id: materialIds[1],
    user_id: userIds[1],
    payload: {
      event_type: "material_uploaded",
      material_id: materialIds[1],
      author_id: userIds[1],
      s3_key: "materials/2025/11/18/clean-architecture.pdf",
      preferred_language: "es",
      timestamp: "2025-11-18T10:59:50Z"
    },
    status: "completed",
    processing_time_ms: 4750,
    processed_at: new Date("2025-11-18T11:00:05Z"),
    created_at: new Date("2025-11-18T11:00:00Z")
  },
  {
    event_type: "material_uploaded",
    event_id: "evt-750e8400-e29b-41d4-a716-446655440003",
    material_id: materialIds[2],
    user_id: userIds[2],
    payload: {
      event_type: "material_uploaded",
      material_id: materialIds[2],
      author_id: userIds[2],
      s3_key: "materials/2025/11/18/go-best-practices.pdf",
      preferred_language: "en",
      timestamp: "2025-11-18T11:59:52Z"
    },
    status: "completed",
    processing_time_ms: 6100,
    processed_at: new Date("2025-11-18T12:00:06Z"),
    created_at: new Date("2025-11-18T12:00:00Z")
  },
  // Evento en procesamiento
  {
    event_type: "material_uploaded",
    event_id: "evt-850e8400-e29b-41d4-a716-446655440004",
    material_id: materialIds[3],
    user_id: userIds[0],
    payload: {
      event_type: "material_uploaded",
      material_id: materialIds[3],
      author_id: userIds[0],
      s3_key: "materials/2025/11/18/microservices.pdf",
      preferred_language: "es",
      timestamp: "2025-11-18T12:59:45Z"
    },
    status: "processing",
    processing_time_ms: 3200,
    processed_at: null,
    created_at: new Date("2025-11-18T13:00:00Z")
  },
  // Evento fallido
  {
    event_type: "material_uploaded",
    event_id: "evt-950e8400-e29b-41d4-a716-446655440005",
    material_id: "a50e8400-e29b-41d4-a716-446655440009",
    user_id: userIds[1],
    payload: {
      event_type: "material_uploaded",
      material_id: "a50e8400-e29b-41d4-a716-446655440009",
      author_id: userIds[1],
      s3_key: "materials/2025/11/18/corrupted.pdf",
      preferred_language: "es",
      timestamp: "2025-11-18T13:30:00Z"
    },
    status: "failed",
    error_message: "failed to extract PDF text: file corrupted or invalid format",
    error_stack: "goroutine 42 [running]:\ngithub.com/EduGoGroup/edugo-worker/internal/infrastructure/pdf.(*Extractor).Extract(...)\n\t/app/internal/infrastructure/pdf/extractor.go:45\ngithub.com/EduGoGroup/edugo-worker/internal/application/processor.(*MaterialUploadedProcessor).Process(...)\n\t/app/internal/application/processor/material_uploaded_processor.go:56",
    retry_count: 3,
    processing_time_ms: 1200,
    processed_at: new Date("2025-11-18T13:30:08Z"),
    created_at: new Date("2025-11-18T13:30:00Z")
  },
  // Evento de assessment attempt
  {
    event_type: "assessment_attempt",
    event_id: "evt-a60e8400-e29b-41d4-a716-446655440006",
    material_id: materialIds[0],
    user_id: userIds[2],
    payload: {
      event_type: "assessment_attempt",
      material_id: materialIds[0],
      user_id: userIds[2],
      answers: {
        "q-f3e4d5c6-b7a8-4c3d-9e2f-1a0b9c8d7e6f": "opt-2a",
        "q-a1b2c3d4-e5f6-4a7b-8c9d-0e1f2a3b4c5d": "true"
      },
      score: 85.5,
      timestamp: "2025-11-18T14:15:00Z"
    },
    status: "completed",
    processing_time_ms: 340,
    processed_at: new Date("2025-11-18T14:15:01Z"),
    created_at: new Date("2025-11-18T14:15:00Z")
  },
  // Evento de student enrolled
  {
    event_type: "student_enrolled",
    event_id: "evt-b70e8400-e29b-41d4-a716-446655440007",
    material_id: null,
    user_id: userIds[0],
    payload: {
      event_type: "student_enrolled",
      student_id: userIds[0],
      unit_id: "unit-123e4567-e89b-12d3-a456-426614174000",
      timestamp: "2025-11-18T15:00:00Z"
    },
    status: "completed",
    processing_time_ms: 120,
    processed_at: new Date("2025-11-18T15:00:01Z"),
    created_at: new Date("2025-11-18T15:00:00Z")
  },
  // Evento de material deleted
  {
    event_type: "material_deleted",
    event_id: "evt-c80e8400-e29b-41d4-a716-446655440008",
    material_id: "c80e8400-e29b-41d4-a716-446655440010",
    user_id: null,
    payload: {
      event_type: "material_deleted",
      material_id: "c80e8400-e29b-41d4-a716-446655440010",
      timestamp: "2025-11-18T16:00:00Z"
    },
    status: "completed",
    processing_time_ms: 850,
    processed_at: new Date("2025-11-18T16:00:01Z"),
    created_at: new Date("2025-11-18T16:00:00Z")
  },
  // Evento de reprocess
  {
    event_type: "material_reprocess",
    event_id: "evt-d90e8400-e29b-41d4-a716-446655440009",
    material_id: materialIds[0],
    user_id: userIds[0],
    payload: {
      event_type: "material_reprocess",
      material_id: materialIds[0],
      author_id: userIds[0],
      s3_key: "materials/2025/11/18/mongodb-intro.pdf",
      preferred_language: "es",
      timestamp: "2025-11-18T17:00:00Z"
    },
    status: "completed",
    processing_time_ms: 5200,
    processed_at: new Date("2025-11-18T17:00:06Z"),
    created_at: new Date("2025-11-18T17:00:00Z")
  }
];

try {
  const eventResult = db.material_event.insertMany(events);
  print(`✅ Inserted ${eventResult.insertedIds.length} material events`);
} catch (error) {
  print(`❌ Error inserting events: ${error}`);
}

print("");

// ═══════════════════════════════════════════════════════════════════
// RESUMEN FINAL
// ═══════════════════════════════════════════════════════════════════

print("\n╔════════════════════════════════════════════════════════════════╗");
print("║                                                                ║");
print("║                  ✅ SEED DATA COMPLETED                       ║");
print("║                                                                ║");
print("╚════════════════════════════════════════════════════════════════╝\n");

print("📊 Summary:");
print("─────────────────────────────────────────────────────────────────");

const summaryCount = db.material_summary.countDocuments();
const assessmentCount = db.material_assessment.countDocuments();
const eventCount = db.material_event.countDocuments();

print(`✅ material_summary: ${summaryCount} documents`);
print(`✅ material_assessment: ${assessmentCount} documents`);
print(`✅ material_event: ${eventCount} documents`);

print("\n📝 Test Data Details:");
print("─────────────────────────────────────────────────────────────────");

print("\nMaterial Summaries:");
print("  - Languages: es (3), en (1), pt (1)");
print("  - AI Models: gpt-4 (2), gpt-4-turbo (2), gpt-4o (1)");

print("\nMaterial Assessments:");
print("  - Total questions: varies by assessment");
print("  - Question types: multiple_choice, true_false, open");
print("  - Difficulty levels: easy, medium, hard");

print("\nMaterial Events:");
print("  - Event types: material_uploaded, assessment_attempt, student_enrolled, material_deleted, material_reprocess");
print("  - Statuses: completed (7), processing (1), failed (1)");
print("  - Failed event demonstrates error handling with stack trace");

print("\n─────────────────────────────────────────────────────────────────");
print("\n✅ Test data inserted successfully!");
print("\n📝 Next steps:");
print("   1. Verify data: db.material_summary.find().pretty()");
print("   2. Test queries from MONGODB_SCHEMA.md");
print("   3. Validate schemas by trying invalid inserts");
print("   4. Monitor TTL index on material_event");
print("\n═════════════════════════════════════════════════════════════════\n");
