# 🔄 Workflow Templates - Ejecución en 2 Fases

**Fecha:** 16 de Noviembre, 2025  
**Versión:** 1.0.0  
**Propósito:** Templates genéricos para workflow de 2 fases (Web + Local)

---

## 🎯 ¿Qué es esto?

Estos templates implementan el **workflow de 2 fases** para ejecución desatendida:

- **Fase 1 (Claude Code Web):** Implementación con stubs/mocks
- **Fase 2 (Claude Code Local):** Implementación real, CI/CD, merge

---

## 📦 Templates Incluidos

### 1. WORKFLOW_ORCHESTRATION.md
Sistema completo de orquestación de 2 fases

### 2. TRACKING_SYSTEM.md
Sistema de tracking con PROGRESS.json

### 3. PHASE2_BRIDGE_TEMPLATE.md
Template para documento puente entre fases

### 4. EXECUTION_REPORT_TEMPLATE.md
Template para reporte de ejecución

### 5. PROGRESS_TEMPLATE.json
Template de archivo de tracking

### 6. scripts/
Scripts de automatización (update-progress.sh, recover.sh, etc.)

---

## 🚀 Cómo Usar

### Paso 1: Copiar Templates a tu Proyecto

```bash
# Ir a tu proyecto en 00-Projects-Isolated
cd /Users/jhoanmedina/source/EduGo/Analisys/00-Projects-Isolated/[proyecto]/

# Copiar archivos base
cp /path/to/workflow-templates/WORKFLOW_ORCHESTRATION.md ./
cp /path/to/workflow-templates/TRACKING_SYSTEM.md ./
cp /path/to/workflow-templates/PROGRESS_TEMPLATE.json ./PROGRESS.json

# Copiar scripts
mkdir -p scripts
cp -r /path/to/workflow-templates/scripts/* ./scripts/
```

### Paso 2: Adaptar PROGRESS.json

Editar PROGRESS.json con los sprints de tu proyecto:

```json
{
  "project": "edugo-api-mobile",
  "sprints": {
    "Sprint-01-Schema-BD": {
      "name": "Schema de Base de Datos",
      "status": "pending",
      "tasks": {
        "TASK-001": {
          "name": "Crear migraciones PostgreSQL",
          "status": "pending"
        }
      }
    }
  }
}
```

### Paso 3: Crear PHASE2_BRIDGE.md por Sprint

Para cada sprint, crear:
```
04-Implementation/Sprint-XX-Nombre/PHASE2_BRIDGE.md
```

Usar template: `PHASE2_BRIDGE_TEMPLATE.md`

---

## 📋 Estructura Recomendada en cada Proyecto

```
proyecto/
├── WORKFLOW_ORCHESTRATION.md     ← Copiado de template
├── TRACKING_SYSTEM.md             ← Copiado de template  
├── PROGRESS.json                  ← Adaptado del template
│
├── 04-Implementation/
│   ├── Sprint-01-.../
│   │   ├── README.md
│   │   ├── TASKS.md
│   │   ├── DEPENDENCIES.md
│   │   ├── VALIDATION.md
│   │   ├── PHASE2_BRIDGE.md       ← Generado en Fase 1
│   │   └── EXECUTION_REPORT.md    ← Generado en Fase 2
│   │
│   └── Sprint-02-.../
│       └── [misma estructura]
│
└── scripts/
    ├── update-progress.sh         ← Copiado de template
    ├── recover.sh                 ← Copiado de template
    └── daily-report.sh            ← Copiado de template
```

---

## ✅ Beneficios del Workflow

1. **Ejecución desatendida** en Claude Code Web
2. **Continuación local** con recursos reales
3. **Recuperación** ante interrupciones
4. **Tracking** automático de progreso
5. **CI/CD** validado antes de merge
6. **Code review** de Copilot atendido automáticamente

---

**Siguiente paso:** Ver archivos de templates individuales
