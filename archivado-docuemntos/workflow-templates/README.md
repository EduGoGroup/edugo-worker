# 📚 Workflow Templates - EduGo

## 🎯 Propósito

Templates reutilizables para mantener consistencia en la documentación de todos los proyectos del ecosistema EduGo.

---

## 📁 Archivos Disponibles

### 1. WORKFLOW_ORCHESTRATION.md
**Descripción:** Sistema de orquestación de 2 fases (Claude Code Web + Local)

**Uso:**
- Workflow para desarrollo distribuido
- Coordinación entre fase de análisis (Web) y ejecución (Local)
- Sistema de bridge para transferir contexto

**Cuándo usar:** Proyectos que requieren análisis previo antes de implementación

---

### 2. TRACKING_SYSTEM.md
**Descripción:** Sistema de tracking de progreso con PROGRESS.json

**Uso:**
- Monitoreo de progreso en tiempo real
- Tracking de sprints y tareas
- Métricas de avance

**Cuándo usar:** Proyectos con múltiples sprints o fases

---

### 3. PHASE2_BRIDGE_TEMPLATE.md
**Descripción:** Template para documentos puente entre fases

**Uso:**
- Transferir contexto de Fase 1 (Análisis) a Fase 2 (Ejecución)
- Documentar decisiones tomadas
- Checklist de pre-requisitos

**Cuándo usar:** Al iniciar Fase 2 de un proyecto con workflow de 2 fases

---

### 4. PROGRESS_TEMPLATE.json
**Descripción:** Template JSON estructurado para tracking

**Uso:**
- Formato estándar de tracking
- Integración con herramientas de monitoreo
- Generación de reportes automáticos

**Cuándo usar:** Proyectos que requieren tracking automatizado

---

## 🚀 Cómo Usar Estos Templates

### Opción 1: Copiar a Nuevo Proyecto

```bash
# Copiar todos los templates a un nuevo proyecto
cp -r docs/workflow-templates/* /path/to/nuevo-proyecto/docs/workflow-templates/
```

### Opción 2: Referenciar Desde Otro Proyecto

```markdown
<!-- En docs de otro proyecto -->
## Workflows

Ver workflows estándar en:
- [edugo-worker/docs/workflow-templates/](../edugo-worker/docs/workflow-templates/)
```

### Opción 3: Adaptar Según Necesidad

1. Copiar template específico
2. Modificar según contexto del proyecto
3. Mantener estructura base
4. Documentar cambios

---

## 📋 Proyectos Usando Estos Templates

- ✅ `edugo-worker` (este proyecto)
- ✅ `edugo-api-administracion`
- ✅ `edugo-api-mobile`

---

## 🔄 Versionado

**Versión actual:** 1.0.0
**Última actualización:** 17 de Noviembre, 2025
**Mantenido por:** Equipo EduGo

---

## 📞 Soporte

Si necesitas ayuda con estos templates:
1. Revisar ejemplos en proyectos listados arriba
2. Consultar documentación específica dentro de cada template
3. Contactar al equipo de arquitectura

---

## 🎓 Filosofía

> **"Documentación consistente = Onboarding rápido + Mantenimiento sencillo"**

Estos templates existen para:
- ✅ Mantener coherencia entre proyectos
- ✅ Reducir tiempo de setup de nuevos proyectos
- ✅ Facilitar transferencia de conocimiento
- ✅ Estandarizar procesos de desarrollo
