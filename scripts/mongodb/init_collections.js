// init_collections.js
// Script de inicialización de collections MongoDB para edugo-worker
//
// Proyecto: edugo-worker
// Sprint: Sprint-01 Fase 1 - Auditoría y Diseño de Schemas MongoDB
// Autor: Claude Code Web
// Fecha: 2025-11-18
//
// EJECUCIÓN:
//   Con mongosh: mongosh "mongodb://localhost:27017/edugo" --username <user> --password <pass> < scripts/mongodb/init_collections.js
//   O directamente: mongosh "mongodb://localhost:27017/edugo" --eval "load('scripts/mongodb/init_collections.js')"
//
// IMPORTANTE: Este script es idempotente. Puede ejecutarse múltiples veces sin errores.

print("\n╔════════════════════════════════════════════════════════════════╗");
print("║                                                                ║");
print("║       MongoDB Schema Initialization - edugo-worker            ║");
print("║                                                                ║");
print("╚════════════════════════════════════════════════════════════════╝\n");

// Seleccionar base de datos
const dbName = "edugo";
db = db.getSiblingDB(dbName);

print(`📊 Using database: ${dbName}\n`);

// ═══════════════════════════════════════════════════════════════════
// FUNCIÓN AUXILIAR: Crear collection con validation
// ═══════════════════════════════════════════════════════════════════

function createCollectionIfNotExists(collectionName, validationSchema) {
  const collections = db.getCollectionNames();

  if (collections.includes(collectionName)) {
    print(`⚠️  Collection "${collectionName}" already exists. Updating validation schema...`);
    try {
      db.runCommand({
        collMod: collectionName,
        validator: validationSchema.validator,
        validationLevel: "strict",
        validationAction: "error"
      });
      print(`✅ Validation schema updated for "${collectionName}"\n`);
    } catch (error) {
      print(`❌ Error updating validation for "${collectionName}": ${error}\n`);
    }
  } else {
    print(`🔧 Creating collection: "${collectionName}"...`);
    try {
      db.createCollection(collectionName, {
        validator: validationSchema.validator,
        validationLevel: "strict",
        validationAction: "error"
      });
      print(`✅ Collection "${collectionName}" created successfully\n`);
    } catch (error) {
      print(`❌ Error creating "${collectionName}": ${error}\n`);
      throw error;
    }
  }
}

// ═══════════════════════════════════════════════════════════════════
// FUNCIÓN AUXILIAR: Crear índice si no existe
// ═══════════════════════════════════════════════════════════════════

function createIndexIfNotExists(collection, indexSpec, options) {
  const indexName = options.name;
  const existingIndexes = collection.getIndexes();
  const indexExists = existingIndexes.some(idx => idx.name === indexName);

  if (indexExists) {
    print(`  ⚠️  Index "${indexName}" already exists. Skipping...`);
  } else {
    try {
      collection.createIndex(indexSpec, options);
      print(`  ✅ Index "${indexName}" created`);
    } catch (error) {
      print(`  ❌ Error creating index "${indexName}": ${error}`);
      throw error;
    }
  }
}

// ═══════════════════════════════════════════════════════════════════
// 1. Collection: material_summary
// ═══════════════════════════════════════════════════════════════════

print("┌──────────────────────────────────────────────────────────────┐");
print("│ 1. Collection: material_summary                              │");
print("└──────────────────────────────────────────────────────────────┘\n");

const materialSummaryValidation = {
  validator: {
    $jsonSchema: {
      bsonType: "object",
      required: [
        "material_id",
        "summary",
        "key_points",
        "language",
        "word_count",
        "version",
        "ai_model",
        "processing_time_ms",
        "created_at",
        "updated_at"
      ],
      properties: {
        material_id: {
          bsonType: "string",
          pattern: "^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$",
          description: "UUID v4 del material en PostgreSQL (requerido)"
        },
        summary: {
          bsonType: "string",
          minLength: 10,
          maxLength: 5000,
          description: "Resumen generado por IA (min 10, max 5000 caracteres)"
        },
        key_points: {
          bsonType: "array",
          minItems: 1,
          maxItems: 10,
          items: {
            bsonType: "string",
            minLength: 5,
            maxLength: 500
          },
          description: "Array de puntos clave (1-10 elementos)"
        },
        language: {
          enum: ["es", "en", "pt"],
          description: "Idioma del resumen: español, inglés o portugués"
        },
        word_count: {
          bsonType: "int",
          minimum: 1,
          description: "Número de palabras del resumen (mínimo 1)"
        },
        version: {
          bsonType: "int",
          minimum: 1,
          description: "Versión del resumen (>= 1, incrementa en reprocesos)"
        },
        ai_model: {
          enum: ["gpt-4", "gpt-3.5-turbo", "gpt-4-turbo", "gpt-4o"],
          description: "Modelo de IA utilizado para generar el resumen"
        },
        processing_time_ms: {
          bsonType: "int",
          minimum: 0,
          description: "Tiempo de procesamiento en milisegundos"
        },
        token_usage: {
          bsonType: "object",
          properties: {
            prompt_tokens: {
              bsonType: "int",
              minimum: 0,
              description: "Tokens consumidos en el prompt"
            },
            completion_tokens: {
              bsonType: "int",
              minimum: 0,
              description: "Tokens consumidos en la respuesta"
            },
            total_tokens: {
              bsonType: "int",
              minimum: 0,
              description: "Total de tokens consumidos"
            }
          },
          description: "Metadata de tokens consumidos (opcional)"
        },
        metadata: {
          bsonType: "object",
          properties: {
            source_length: {
              bsonType: "int",
              minimum: 0,
              description: "Longitud del texto fuente original"
            },
            has_images: {
              bsonType: "bool",
              description: "Si el material contiene imágenes"
            }
          },
          description: "Metadata adicional del procesamiento (opcional)"
        },
        created_at: {
          bsonType: "date",
          description: "Fecha de creación del resumen (requerido)"
        },
        updated_at: {
          bsonType: "date",
          description: "Fecha de última actualización (requerido)"
        }
      },
      additionalProperties: true
    }
  }
};

createCollectionIfNotExists("material_summary", materialSummaryValidation);

// Índices para material_summary
print("📑 Creating indexes for material_summary...");
const summaryCollection = db.getCollection("material_summary");

createIndexIfNotExists(
  summaryCollection,
  { material_id: 1 },
  { unique: true, name: "idx_material_id" }
);

createIndexIfNotExists(
  summaryCollection,
  { created_at: -1 },
  { name: "idx_created_at" }
);

createIndexIfNotExists(
  summaryCollection,
  { version: 1 },
  { name: "idx_version" }
);

createIndexIfNotExists(
  summaryCollection,
  { language: 1, created_at: -1 },
  { name: "idx_language_created" }
);

print("\n✅ material_summary setup completed\n");

// ═══════════════════════════════════════════════════════════════════
// 2. Collection: material_assessment
// ═══════════════════════════════════════════════════════════════════

print("┌──────────────────────────────────────────────────────────────┐");
print("│ 2. Collection: material_assessment                           │");
print("└──────────────────────────────────────────────────────────────┘\n");

const materialAssessmentValidation = {
  validator: {
    $jsonSchema: {
      bsonType: "object",
      required: [
        "material_id",
        "title",
        "questions",
        "total_questions",
        "total_points",
        "passing_score",
        "version",
        "ai_model",
        "processing_time_ms",
        "created_at",
        "updated_at"
      ],
      properties: {
        material_id: {
          bsonType: "string",
          pattern: "^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$",
          description: "UUID v4 del material (requerido)"
        },
        title: {
          bsonType: "string",
          minLength: 3,
          maxLength: 200,
          description: "Título del quiz (3-200 caracteres)"
        },
        description: {
          bsonType: "string",
          maxLength: 1000,
          description: "Descripción del quiz (opcional, max 1000 caracteres)"
        },
        questions: {
          bsonType: "array",
          minItems: 1,
          maxItems: 50,
          items: {
            bsonType: "object",
            required: ["id", "text", "type", "difficulty", "points", "explanation", "order"],
            properties: {
              id: {
                bsonType: "string",
                pattern: "^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$",
                description: "UUID v4 de la pregunta"
              },
              text: {
                bsonType: "string",
                minLength: 10,
                maxLength: 1000,
                description: "Texto de la pregunta (10-1000 caracteres)"
              },
              type: {
                enum: ["multiple_choice", "true_false", "open"],
                description: "Tipo de pregunta"
              },
              difficulty: {
                enum: ["easy", "medium", "hard"],
                description: "Nivel de dificultad"
              },
              points: {
                bsonType: "int",
                minimum: 1,
                maximum: 100,
                description: "Puntos de la pregunta (1-100)"
              },
              options: {
                bsonType: "array",
                minItems: 2,
                maxItems: 6,
                items: {
                  bsonType: "object",
                  required: ["id", "text", "is_correct", "order"],
                  properties: {
                    id: {
                      bsonType: "string",
                      description: "UUID de la opción"
                    },
                    text: {
                      bsonType: "string",
                      minLength: 1,
                      maxLength: 500,
                      description: "Texto de la opción"
                    },
                    is_correct: {
                      bsonType: "bool",
                      description: "Si es la respuesta correcta"
                    },
                    order: {
                      bsonType: "int",
                      minimum: 1,
                      description: "Orden de visualización"
                    }
                  }
                },
                description: "Opciones para preguntas multiple_choice"
              },
              correct_answer: {
                bsonType: "string",
                description: "Respuesta correcta para preguntas true_false"
              },
              explanation: {
                bsonType: "string",
                minLength: 10,
                maxLength: 1000,
                description: "Explicación de la respuesta correcta"
              },
              bloom_taxonomy_level: {
                enum: ["remember", "understand", "apply", "analyze", "evaluate", "create"],
                description: "Nivel de taxonomía de Bloom (opcional)"
              },
              order: {
                bsonType: "int",
                minimum: 1,
                description: "Orden de la pregunta en el quiz"
              }
            }
          },
          description: "Array de preguntas (1-50 preguntas)"
        },
        total_questions: {
          bsonType: "int",
          minimum: 1,
          maximum: 50,
          description: "Cantidad total de preguntas"
        },
        total_points: {
          bsonType: "int",
          minimum: 1,
          description: "Suma total de puntos del quiz"
        },
        passing_score: {
          bsonType: "int",
          minimum: 0,
          description: "Puntaje mínimo para aprobar"
        },
        time_limit_minutes: {
          bsonType: ["int", "null"],
          minimum: 1,
          description: "Tiempo límite en minutos (null = sin límite)"
        },
        difficulty_distribution: {
          bsonType: "object",
          properties: {
            easy: {
              bsonType: "int",
              minimum: 0,
              description: "Cantidad de preguntas fáciles"
            },
            medium: {
              bsonType: "int",
              minimum: 0,
              description: "Cantidad de preguntas medianas"
            },
            hard: {
              bsonType: "int",
              minimum: 0,
              description: "Cantidad de preguntas difíciles"
            }
          },
          description: "Distribución de dificultad (opcional)"
        },
        version: {
          bsonType: "int",
          minimum: 1,
          description: "Versión del assessment (>= 1)"
        },
        ai_model: {
          enum: ["gpt-4", "gpt-3.5-turbo", "gpt-4-turbo", "gpt-4o"],
          description: "Modelo de IA utilizado"
        },
        processing_time_ms: {
          bsonType: "int",
          minimum: 0,
          description: "Tiempo de procesamiento en ms"
        },
        created_at: {
          bsonType: "date",
          description: "Fecha de creación"
        },
        updated_at: {
          bsonType: "date",
          description: "Fecha de última actualización"
        }
      },
      additionalProperties: true
    }
  }
};

createCollectionIfNotExists("material_assessment", materialAssessmentValidation);

// Índices para material_assessment
print("📑 Creating indexes for material_assessment...");
const assessmentCollection = db.getCollection("material_assessment");

createIndexIfNotExists(
  assessmentCollection,
  { material_id: 1 },
  { unique: true, name: "idx_material_id" }
);

createIndexIfNotExists(
  assessmentCollection,
  { created_at: -1 },
  { name: "idx_created_at" }
);

createIndexIfNotExists(
  assessmentCollection,
  { version: 1 },
  { name: "idx_version" }
);

createIndexIfNotExists(
  assessmentCollection,
  { "questions.difficulty": 1 },
  { name: "idx_questions_difficulty" }
);

createIndexIfNotExists(
  assessmentCollection,
  { total_questions: 1, created_at: -1 },
  { name: "idx_total_questions_created" }
);

print("\n✅ material_assessment setup completed\n");

// ═══════════════════════════════════════════════════════════════════
// 3. Collection: material_event
// ═══════════════════════════════════════════════════════════════════

print("┌──────────────────────────────────────────────────────────────┐");
print("│ 3. Collection: material_event                                │");
print("└──────────────────────────────────────────────────────────────┘\n");

const materialEventValidation = {
  validator: {
    $jsonSchema: {
      bsonType: "object",
      required: ["event_type", "payload", "status", "created_at"],
      properties: {
        event_type: {
          enum: [
            "material_uploaded",
            "material_reprocess",
            "material_deleted",
            "assessment_attempt",
            "student_enrolled"
          ],
          description: "Tipo de evento procesado"
        },
        event_id: {
          bsonType: "string",
          description: "ID único del evento (opcional)"
        },
        material_id: {
          bsonType: ["string", "null"],
          pattern: "^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$",
          description: "UUID del material (nullable para eventos sin material)"
        },
        user_id: {
          bsonType: ["string", "null"],
          pattern: "^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$",
          description: "UUID del usuario (nullable)"
        },
        payload: {
          bsonType: "object",
          description: "Payload completo del evento (JSON original)"
        },
        status: {
          enum: ["pending", "processing", "completed", "failed"],
          description: "Estado del procesamiento del evento"
        },
        error_message: {
          bsonType: "string",
          maxLength: 1000,
          description: "Mensaje de error si status = 'failed' (opcional)"
        },
        error_stack: {
          bsonType: "string",
          maxLength: 5000,
          description: "Stack trace del error (opcional)"
        },
        retry_count: {
          bsonType: "int",
          minimum: 0,
          maximum: 10,
          description: "Número de reintentos (opcional)"
        },
        processing_time_ms: {
          bsonType: "int",
          minimum: 0,
          description: "Tiempo de procesamiento en ms (opcional)"
        },
        processed_at: {
          bsonType: ["date", "null"],
          description: "Fecha/hora de procesamiento completado (opcional)"
        },
        created_at: {
          bsonType: "date",
          description: "Fecha/hora de recepción del evento (requerido)"
        }
      },
      additionalProperties: true
    }
  }
};

createCollectionIfNotExists("material_event", materialEventValidation);

// Índices para material_event
print("📑 Creating indexes for material_event...");
const eventCollection = db.getCollection("material_event");

createIndexIfNotExists(
  eventCollection,
  { event_type: 1 },
  { name: "idx_event_type" }
);

createIndexIfNotExists(
  eventCollection,
  { material_id: 1 },
  { name: "idx_material_id" }
);

createIndexIfNotExists(
  eventCollection,
  { status: 1 },
  { name: "idx_status" }
);

createIndexIfNotExists(
  eventCollection,
  { created_at: -1 },
  { name: "idx_created_at" }
);

createIndexIfNotExists(
  eventCollection,
  { processed_at: -1 },
  { name: "idx_processed_at" }
);

createIndexIfNotExists(
  eventCollection,
  { status: 1, created_at: -1 },
  { name: "idx_status_created" }
);

// ⚠️ TTL Index: Eliminar eventos después de 90 días
print("\n⏰ Creating TTL index (90 days retention)...");
createIndexIfNotExists(
  eventCollection,
  { created_at: 1 },
  { expireAfterSeconds: 7776000, name: "idx_ttl_created_at" }
);

print("\n✅ material_event setup completed (with TTL index)\n");

// ═══════════════════════════════════════════════════════════════════
// RESUMEN FINAL
// ═══════════════════════════════════════════════════════════════════

print("\n╔════════════════════════════════════════════════════════════════╗");
print("║                                                                ║");
print("║                   ✅ INITIALIZATION COMPLETED                 ║");
print("║                                                                ║");
print("╚════════════════════════════════════════════════════════════════╝\n");

print("📊 Summary:");
print("─────────────────────────────────────────────────────────────────");

const collections = db.getCollectionNames();
print(`✅ Collections in database "${dbName}": ${collections.length}`);
collections.forEach(col => {
  const stats = db.getCollection(col).stats();
  const indexes = db.getCollection(col).getIndexes();
  print(`   - ${col}: ${stats.count} documents, ${indexes.length} indexes`);
});

print("\n📑 Indexes created:");
print("─────────────────────────────────────────────────────────────────");

["material_summary", "material_assessment", "material_event"].forEach(collectionName => {
  if (collections.includes(collectionName)) {
    const indexes = db.getCollection(collectionName).getIndexes();
    print(`\n${collectionName}:`);
    indexes.forEach(idx => {
      const keys = Object.keys(idx.key).map(k => `${k}: ${idx.key[k]}`).join(", ");
      const unique = idx.unique ? " [UNIQUE]" : "";
      const ttl = idx.expireAfterSeconds ? ` [TTL: ${idx.expireAfterSeconds}s]` : "";
      print(`  ✓ ${idx.name}: { ${keys} }${unique}${ttl}`);
    });
  }
});

print("\n─────────────────────────────────────────────────────────────────");
print("\n✅ MongoDB schema initialization completed successfully!");
print("\n📝 Next steps:");
print("   1. Run seed_data.js to insert test data");
print("   2. Verify collections: db.getCollectionNames()");
print("   3. Test validation: Try inserting invalid documents");
print("   4. Monitor TTL index: db.material_event.getIndexes()");
print("\n═════════════════════════════════════════════════════════════════\n");
