# Fase 1: Funcionalidad Crítica

> **Objetivo:** Implementar el procesamiento real de eventos, eliminar código deprecado y refactorizar el sistema de bootstrap.
>
> **Duración estimada:** 2-3 semanas
> **Complejidad:** Alta
> **Riesgo:** Medio
> **Prerequisito:** Fase 0 completada

---

## 🎯 Objetivos

1. ✅ Implementar routing real de eventos a processors
2. ✅ Eliminar código deprecado y simulado
3. ✅ Refactorizar sistema de bootstrap (simplificar doble puntero)
4. ✅ Unificar uso de logger estructurado
5. ✅ Aumentar cobertura de tests a >60%

---

## 📦 Entregables

### E1.1: ProcessorRegistry Implementado
- Interfaz `Processor` común
- Registry para mapear event_type → processor
- Routing real desde `processMessage()`
- Tests unitarios del registry

### E1.2: Bootstrap Refactorizado
- Eliminar patrón de doble puntero
- Implementar ResourceBuilder más simple
- Cleanup ordenado de recursos
- Tests del nuevo bootstrap

### E1.3: Código Limpio
- Eliminar TODOs con código comentado
- Unificar uso de logger (eliminar `log.Printf`)
- Eliminar código simulado/hardcoded
- Documentar código faltante para fases futuras

### E1.4: Tests Mejorados
- Tests unitarios para processors
- Tests del routing de eventos
- Mocks básicos para dependencias
- Cobertura >60%

---

## 🔄 Proceso de Trabajo

### Rama de Trabajo
```bash
git checkout dev
git pull origin dev
git checkout -b feature/fase-1-funcionalidad-critica
```

### Sub-tareas con Commits Atómicos

**Commit 1: Crear interfaz Processor y Registry**
```bash
git commit -m "feat(fase-1): crear interfaz Processor y Registry

- Agregar interfaz Processor con EventType() y Process()
- Implementar ProcessorRegistry con map y Register()
- Agregar tests unitarios del registry

Refs: documents/mejoras/REFACTORING.md RF-002"
```

**Commit 2: Adaptar processors existentes**
```bash
git commit -m "refactor(fase-1): adaptar processors a interfaz Processor

- MaterialUploadedProcessor implementa interfaz
- MaterialDeletedProcessor implementa interfaz
- AssessmentAttemptProcessor implementa interfaz
- StudentEnrolledProcessor implementa interfaz

Refs: documents/mejoras/REFACTORING.md RF-002"
```

**Commit 3: Conectar registry a processMessage()**
```bash
git commit -m "feat(fase-1): implementar routing real en processMessage

- Crear registry en main()
- Reemplazar TODO con llamada a registry.Process()
- Agregar manejo de errores por tipo
- Tests de integración del routing

Refs: documents/mejoras/CODIGO_DEPRECADO.md - processMessage()"
```

**Commit 4: Refactorizar bootstrap**
```bash
git commit -m "refactor(fase-1): simplificar bootstrap eliminando doble puntero

- Crear ResourceBuilder pattern
- Eliminar customFactoriesWrapper
- Cleanup ordenado con defer
- Tests del nuevo bootstrap

Refs: documents/mejoras/REFACTORING.md RF-001"
```

**Commit 5: Unificar logger**
```bash
git commit -m "refactor(fase-1): unificar uso de logger estructurado

- Reemplazar todos los log.Printf con logger
- Agregar logger a context
- Eliminar imports de log estándar
- Documentar patrón de logging

Refs: documents/mejoras/CODIGO_DEPRECADO.md"
```

**Commit 6: Eliminar código deprecado**
```bash
git commit -m "chore(fase-1): eliminar código deprecado y TODOs resueltos

- Eliminar TODOs con código comentado (ahora implementado)
- Marcar código pendiente para Fase 2 con issue references
- Actualizar documentación

Refs: documents/mejoras/CODIGO_DEPRECADO.md"
```

**Commit 7: Tests y cobertura**
```bash
git commit -m "test(fase-1): agregar tests y aumentar cobertura

- Tests unitarios para processors
- Tests del registry
- Mocks básicos para DB
- Cobertura: XX% → 65%

Refs: documents/mejoras/DEUDA_TECNICA.md DT-009"
```

---

## 📋 Checklist de Validación

Antes de crear PR:

### Funcionalidad
- [ ] `processMessage()` llama a registry.Process()
- [ ] Eventos conocidos se rutean correctamente
- [ ] Eventos desconocidos se loguean y no fallan
- [ ] Cada processor tiene su event_type registrado

### Refactoring
- [ ] Bootstrap no usa doble puntero
- [ ] ResourceBuilder es más simple que antes
- [ ] Cleanup de recursos funciona correctamente
- [ ] No hay regresiones en inicialización

### Código Limpio
- [ ] Sin `log.Printf` en código (usar logger)
- [ ] TODOs resueltos eliminados
- [ ] TODOs pendientes tienen issue reference
- [ ] Código documentado

### Tests
- [ ] Tests unitarios para todos los processors
- [ ] Tests del registry con todos los event types
- [ ] Tests del nuevo bootstrap
- [ ] Cobertura >60%

### Compilación y Tests
- [ ] `make build` exitoso
- [ ] `make test` todos pasan
- [ ] `make lint` sin errores
- [ ] Sin warnings críticos

---

## 🎯 Criterios de Aceptación

La Fase 1 se considera **COMPLETADA** cuando:

1. ✅ Worker procesa eventos realmente (no mock)
2. ✅ Registry funciona para todos los event types
3. ✅ Bootstrap simplificado (sin doble puntero)
4. ✅ Logger unificado en todo el código
5. ✅ Código deprecado eliminado
6. ✅ Tests >60% cobertura
7. ✅ PR aprobado y mergeado a `dev`
8. ✅ Tag `fase-1-complete` creado

---

## 📚 Referencias

- [Código Deprecado](../../documents/mejoras/CODIGO_DEPRECADO.md)
- [Deuda Técnica](../../documents/mejoras/DEUDA_TECNICA.md)
- [Refactorizaciones](../../documents/mejoras/REFACTORING.md) - RF-001, RF-002
- [Roadmap](../../documents/mejoras/ROADMAP.md) - Fase 1

---

## ⏭️ Siguiente Fase

**Fase 2: Integraciones Externas**
- Implementar cliente OpenAI
- Implementar extracción de PDF
- Implementar cliente S3

Ver: `plan-mejoras/fase-2/README.md`
