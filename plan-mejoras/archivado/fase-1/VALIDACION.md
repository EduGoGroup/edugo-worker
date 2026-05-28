# Validación - Fase 1: Funcionalidad Crítica

---

## ✅ Checklist de Validación

### 1. Funcionalidad - Registry y Routing

**Tests Unitarios del Registry:**
```bash
go test ./internal/application/processor -v -run TestRegistry
```

- [ ] Registry registra processors correctamente
- [ ] Registry rutea event_type conocidos
- [ ] Registry maneja event_type desconocidos sin fallar
- [ ] Registry retorna errores de processors correctamente

**Tests de Integración:**
```bash
go test ./cmd -v -run TestProcessMessage
```

- [ ] processMessage() usa registry
- [ ] Eventos se procesan realmente (no mock)
- [ ] Errores se manejan apropiadamente

---

### 2. Refactoring - Bootstrap

**Tests del ResourceBuilder:**
```bash
go test ./internal/bootstrap -v
```

- [ ] ResourceBuilder crea todos los recursos
- [ ] Cleanup funciona en orden correcto
- [ ] Sin fugas de recursos (conexiones)
- [ ] Manejo de errores durante bootstrap

**Validación Manual:**
```bash
# Iniciar worker y verificar logs
./bin/worker

# Debe mostrar:
# ✅ PostgreSQL conectado
# ✅ MongoDB conectado  
# ✅ RabbitMQ conectado
# ✅ Logger inicializado
# ✅ Processors registrados: 4
```

---

### 3. Código Limpio

**Verificar no hay log.Printf:**
```bash
grep -r "log.Printf" --include="*.go" internal/ cmd/
# Debe retornar: vacío o solo en vendor/
```

- [ ] Sin `log.Printf` en código propio
- [ ] Logger estructurado usado consistentemente

**Verificar TODOs:**
```bash
grep -rn "TODO" --include="*.go" internal/ cmd/
```

- [ ] TODOs resueltos eliminados
- [ ] TODOs pendientes tienen issue reference
- [ ] Código comentado eliminado

---

### 4. Cobertura de Tests

```bash
go test -coverprofile=coverage.out ./...
go tool cover -func=coverage.out | tail -1
```

**Meta: >60% cobertura global**

- [ ] `internal/application/processor/`: >70%
- [ ] `internal/bootstrap/`: >60%
- [ ] `cmd/`: >50%
- [ ] Global: >60%

---

### 5. Compilación y Tests Completos

```bash
make build
make test
make lint
```

- [ ] Compilación exitosa
- [ ] Todos los tests pasan
- [ ] Linters sin errores críticos
- [ ] Sin regresiones

---

## 🎯 Criterios de Aceptación

✅ **FASE 1 EXITOSA** si:

1. Worker procesa eventos realmente
2. Registry implementado y funcionando
3. Bootstrap simplificado (sin doble puntero)
4. Logger unificado
5. Código deprecado eliminado
6. Tests >60% cobertura
7. CI/CD pasa
8. PR aprobado y mergeado
