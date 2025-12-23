# Tareas - Fase 2: Integraciones Externas

---

## 📋 Resumen de Tareas

### Semana 1-2: Cliente OpenAI

**T2.1: Configuración y estructura** (4h)
- Agregar config para OpenAI
- Crear estructura de archivos
- Definir interfaces

**T2.2: Cliente HTTP OpenAI** (8h)
- Implementar cliente HTTP
- Autenticación con API key
- Manejo de rate limits (429)
- Timeouts y retries

**T2.3: Prompts** (12h)
- Diseñar prompt para resumen
- Diseñar prompt para quiz
- Validar salida con ejemplos reales
- Iterar hasta obtener calidad

**T2.4: Parser de respuestas** (6h)
- Parsear JSON de OpenAI
- Validar estructura
- Manejo de errores

**T2.5: Tests** (8h)
- Mocks del cliente
- Tests unitarios
- Tests con API real (opcional)

### Semana 2-3: Extracción PDF y S3

**T2.6: Cliente S3** (8h)
- Setup AWS SDK
- Implementar descarga
- Retry con backoff
- Tests

**T2.7: Extractor PDF** (12h)
- Evaluar librerías (pdfcpu vs unidoc)
- Implementar extracción
- Limpieza de texto
- Tests con PDFs ejemplo

### Semana 3-4: Integración

**T2.8: Actualizar MaterialUploadedProcessor** (12h)
- Integrar S3 → PDF → OpenAI
- Reemplazar código hardcoded
- Manejo de errores por etapa
- Logs detallados

**T2.9: Tests de integración** (12h)
- Setup con Docker/Localstack
- Tests end-to-end
- Validar datos generados

**T2.10: Documentación** (4h)
- README de cada componente
- Ejemplos de uso
- Troubleshooting

---

## ✅ Total Estimado: 86 horas (~3.5 semanas)
