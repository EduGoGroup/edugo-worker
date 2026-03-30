# Tareas - Fase 1: Funcionalidad Crítica

Ver documento completo de tareas detalladas en: [README.md](./README.md)

---

## 📋 Resumen de Tareas

### Semana 1: Registry y Routing

**T1.1: Crear interfaz Processor** (4h)
- Definir interfaz común para todos los processors
- Métodos: `EventType() string` y `Process(ctx, payload) error`

**T1.2: Implementar ProcessorRegistry** (6h)
- Registry con map de event_type → processor
- Método Register() y Process()
- Tests unitarios

**T1.3: Adaptar processors existentes** (8h)
- Implementar interfaz en cada processor
- Adaptar firma de métodos
- Tests actualizados

**T1.4: Conectar a processMessage()** (6h)
- Reemplazar TODO con registry.Process()
- Manejo de errores
- Tests de integración

### Semana 2: Refactoring Bootstrap

**T1.5: Diseñar ResourceBuilder** (4h)
- Diseño del nuevo patrón
- Eliminar doble puntero
- Planificar cleanup

**T1.6: Implementar ResourceBuilder** (8h)
- Implementar builder para cada recurso
- Cleanup ordenado
- Tests

**T1.7: Migrar main.go** (4h)
- Usar nuevo ResourceBuilder en main
- Eliminar código antiguo
- Validar funcionamiento

### Semana 2-3: Limpieza y Tests

**T1.8: Unificar logger** (4h)
- Reemplazar log.Printf
- Logger en context
- Tests

**T1.9: Eliminar código deprecado** (4h)
- Eliminar TODOs resueltos
- Marcar pendientes para Fase 2
- Documentar

**T1.10: Tests y cobertura** (12h)
- Tests para processors
- Tests para registry
- Mocks básicos
- Alcanzar >60% cobertura

**T1.11: Documentación** (4h)
- Actualizar README
- Documentar nuevos patrones
- Diagramas de flujo

---

## ✅ Total Estimado: 64 horas (~2.5 semanas)
