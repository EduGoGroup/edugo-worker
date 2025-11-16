# Preguntas Sprint 01

## Q001: ¿El worker actual funciona?
**Decisión por Defecto:** Auditar primero, no asumir

**Implementación:**
```bash
# Revisar si hay código
ls /Users/jhoanmedina/source/EduGo/repos-separados/edugo-worker/

# Revisar tests
go test ./... -v
```

## Q002: ¿Qué collections MongoDB usar?
**Decisión por Defecto:** 
- `material_summary` (resúmenes)
- `material_assessment` (quizzes)
- `material_event` (logs de procesamiento)

**Justificación:** Alineado con arquitectura existente de EduGo
