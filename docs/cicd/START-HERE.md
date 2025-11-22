# 🎯 COMIENZA AQUÍ - edugo-worker

⚠️ **UBICACIÓN Y CONTEXTO DE TRABAJO:**

```
┌─────────────────────────────────────────────────────────────────────┐
│ 📍 Estás en: 05-worker/ (dentro de cicd-analysis)                 │
│ 📂 Ruta: 00-Projects-Isolated/cicd-analysis/.../05-worker/        │
│ ⚠️ NO uses archivos de otros proyectos (01-shared, 03-api-mobile) │
│ ✅ SOLO usa archivos dentro de 05-worker/                         │
└─────────────────────────────────────────────────────────────────────┘
```

**Última actualización:** 20 Nov 2025, 18:30 hrs

---

## 🗺️ MAPA DE UBICACIÓN

```
00-Projects-Isolated/cicd-analysis/implementation-plans/
│
├── 01-shared/                                  ← Otro proyecto
├── 02-infrastructure/                          ← Otro proyecto
├── 03-api-mobile/                              ← Otro proyecto
├── 04-api-administracion/                      ← Otro proyecto
│
└── 05-worker/                                  ← 👉 ESTÁS AQUÍ
    ├── START-HERE.md                           ← Este archivo
    ├── INDEX.md                                ← Navegación completa
    ├── PROMPTS.md                              ← ⭐ Prompts para cada fase
    ├── README.md                               ← Plan de CI/CD
    ├── docs/                                   ← Documentación
    ├── sprints/                                ← ⭐ Planes de sprint
    │   ├── SPRINT-3-TASKS.md
    │   └── SPRINT-4-TASKS.md
    ├── tracking/                               ← Estado y seguimiento
    │   ├── SPRINT-STATUS.md                    ← Estado actual
    │   ├── REGLAS.md                           ← Reglas de ejecución
    │   └── PR-TEMPLATE.md                      ← Template de PR
    └── assets/
```

---

## 🎯 ¿QUÉ QUIERES HACER?

### 🔍 Opción 1: Ver Estado Actual del Proyecto
```bash
cat tracking/SPRINT-STATUS.md | head -40
```

Lee el archivo para saber:
- Sprint activo
- Fase actual (1, 2, o 3)
- Progreso
- Próxima tarea

### ▶️ Opción 2: Continuar Trabajo desde donde quedó

**Prompt a usar:**
```
Continúa el trabajo de CI/CD en edugo-worker desde donde quedó.
```

Ver detalles en: [PROMPTS.md](PROMPTS.md#continuar-desde-donde-quedó)

### 🆕 Opción 3: Iniciar Nuevo Sprint

**Prompt a usar:**
```
Ejecuta FASE 1 del SPRINT-X en edugo-worker.
```

Reemplaza X con: 3 o 4  
Ver detalles en: [PROMPTS.md](PROMPTS.md#fase-1)

### 📚 Opción 4: Entender el Sistema Completo

**Lee en orden:**
1. [INDEX.md](INDEX.md) - Navegación general (5 min)
2. [README.md](README.md) - Plan completo (25 min)
3. [tracking/REGLAS.md](tracking/REGLAS.md) - Reglas detalladas (15 min)

---

## 📍 NAVEGACIÓN RÁPIDA

| Quiero... | Abrir... |
|-----------|----------|
| 🗺️ Navegar el proyecto | [INDEX.md](INDEX.md) |
| 🎯 Prompts para ejecutar | [PROMPTS.md](PROMPTS.md) ⭐ |
| 📊 Estado actual | [tracking/SPRINT-STATUS.md](tracking/SPRINT-STATUS.md) |
| 📜 Reglas de ejecución | [tracking/REGLAS.md](tracking/REGLAS.md) |
| 📖 Plan completo | [README.md](README.md) |
| 🎯 Ver tareas del sprint | [sprints/](sprints/) |
| 📈 Ver progreso | [tracking/SPRINT-STATUS.md](tracking/SPRINT-STATUS.md) |

---

## 🤖 PARA CLAUDE CODE (INSTRUCCIONES CRÍTICAS)

### ⚠️ Antes de Hacer CUALQUIER COSA:

1. **Lee SIEMPRE:** `INDEX.md`
2. **Verifica ubicación:**
   ```bash
   pwd
   # Debe contener: /05-worker
   ```
3. **Lee estado:** `tracking/SPRINT-STATUS.md`
4. **Identifica:**
   - Sprint activo
   - Fase actual
   - Próxima tarea

### ⚠️ NO Uses Archivos de Otros Proyectos

**Archivos PROHIBIDOS:**
- ❌ `01-shared/sprints/*` (otro proyecto)
- ❌ `03-api-mobile/sprints/*` (otro proyecto)
- ❌ `04-api-administracion/sprints/*` (otro proyecto)
- ❌ `../02-PROPUESTAS-MEJORA.md` (documentación general)

**Archivos PERMITIDOS:**
- ✅ `05-worker/sprints/SPRINT-X-TASKS.md`
- ✅ `05-worker/tracking/*`
- ✅ `05-worker/docs/*`

### ⚠️ Cómo Verificar que Estás en el Archivo Correcto:

```bash
# Al abrir un archivo de sprint, verifica:
readlink -f sprints/SPRINT-3-TASKS.md
# Debe mostrar: .../05-worker/sprints/SPRINT-3-TASKS.md

# Si muestra otra ruta, estás en el lugar equivocado
```

---

## 🔗 Enlaces Importantes

- **Plan general del proyecto:** [README.md](README.md)
- **Navegación completa:** [INDEX.md](INDEX.md)
- **Prompts para ejecutar:** [PROMPTS.md](PROMPTS.md) ⭐
- **Estado en tiempo real:** [tracking/SPRINT-STATUS.md](tracking/SPRINT-STATUS.md)
- **Reglas de ejecución:** [tracking/REGLAS.md](tracking/REGLAS.md)

---

## 📊 COMANDOS RÁPIDOS

### Ver estado actual:
```bash
cd docs/cicd
cat tracking/SPRINT-STATUS.md | head -40
```

### Ver próxima tarea:
```bash
grep "⏳\|🔄" tracking/SPRINT-STATUS.md | head -1
```

### Ver sprints disponibles:
```bash
ls -1 sprints/
```

### Ver logs de sesiones anteriores:
```bash
ls -lt tracking/logs/ | head -5
```

---

**Generado por:** Claude Code  
**Fecha:** 20 de Noviembre, 2025  
**Versión:** 2.0 (con sistema de prompts)
