# Tareas Sprint 01 - Auditoría

## TASK-01-001: Auditar Código Actual del Worker
**Prioridad:** HIGH  
**Estimación:** 3h

#### Implementación
```bash
cd /Users/jhoanmedina/source/EduGo/repos-separados/edugo-worker

# Revisar estructura
ls -la

# Revisar main.go
cat cmd/worker/main.go

# Revisar consumers
ls internal/consumer/

# Revisar servicios
ls internal/services/
```

#### Criterios
- [ ] Código existente revisado
- [ ] Funcionalidades actuales documentadas
- [ ] Gaps identificados

---

## TASK-01-002: Crear/Validar Schema MongoDB
**Prioridad:** HIGH  
**Estimación:** 2h

#### Implementación
```javascript
// Crear collections si no existen
db.createCollection("material_summary")
db.createCollection("material_assessment")
db.createCollection("material_event")

// Crear índices
db.material_summary.createIndex({material_id: 1}, {unique: true})
db.material_assessment.createIndex({material_id: 1}, {unique: true})
```

#### Criterios
- [ ] Collections creadas
- [ ] Índices optimizados
- [ ] Schema validado

**Tiempo total:** 5h
