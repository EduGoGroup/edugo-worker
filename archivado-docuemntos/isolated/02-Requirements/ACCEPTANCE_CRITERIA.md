# Criterios de Aceptación
# spec-02: Worker - Procesamiento IA

**Versión:** 1.0.0  
**Fecha:** 14 de Noviembre, 2025

---

## CRITERIOS GLOBALES

### AC-GLOBAL-001: Material Procesado End-to-End
**Descripción:** Desde evento RabbitMQ hasta MongoDB+PostgreSQL actualizado

**Validación:**
```bash
# Publicar evento de prueba
# Verificar que aparece en MongoDB
mongosh --eval "db.material_summary.findOne({material_id: 'test-id'})"

# Verificar que aparece en PostgreSQL
psql -c "SELECT * FROM assessment WHERE material_id = 'test-id';"
```

**Criterio:** Material procesado en <3 minutos

---

### AC-GLOBAL-002: Resumen de Calidad
**Descripción:** Resumen tiene estructura correcta

**Validación:**
- Tiene 5-7 secciones
- Tiene 10-15 términos en glosario
- Tiene 5-7 preguntas de reflexión
- Texto coherente en español

---

### AC-GLOBAL-003: Quiz Generado
**Descripción:** Quiz tiene preguntas válidas

**Validación:**
- 5-10 preguntas
- Cada pregunta tiene 4 opciones
- 1 respuesta correcta por pregunta
- Distractores plausibles

---

### AC-GLOBAL-004: Reintentos Funcionan
**Descripción:** Errores transitorios se reintentan

**Validación:**
- Simular timeout OpenAI → worker reintenta
- Simular MongoDB down → worker reintenta
- Después de 5 intentos → envía a DLQ

---

**Total Criterios:** 12 criterios medibles
