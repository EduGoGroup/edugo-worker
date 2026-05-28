# Fase 6: Sistemas de Notificaciones

> **Objetivo:** Implementar sistemas de notificación por email y push notifications, junto con processors adicionales para eventos de assessment y enrollment.
>
> **Duración estimada:** 3-4 semanas
> **Complejidad:** Media-Alta
> **Riesgo:** Medio
> **Prerequisito:** Fase 5 completada
> **Origen:** Plan de trabajo PT-009 de edugo_analisis

---

## 🎯 Objetivos

1. ✅ Implementar servicio de email con SendGrid
2. ✅ Implementar push notifications con Firebase
3. ✅ Crear templates de email
4. ✅ Implementar AssessmentAttemptProcessor
5. ✅ Implementar StudentEnrolledProcessor
6. ✅ Configurar notificaciones automáticas

---

## 📦 Entregables

### E6.1: Servicio de Email
- `internal/infrastructure/email/sendgrid_client.go`
- Interface EmailService
- Envío de emails simples
- Envío de emails con templates
- Tests con mocks

### E6.2: Servicio de Push Notifications
- `internal/infrastructure/push/firebase_client.go`
- Interface PushService
- Notificaciones a usuarios individuales
- Notificaciones a múltiples usuarios
- Notificaciones por topics
- Tests con mocks

### E6.3: Templates de Email
- `templates/emails/welcome_student.html`
- `templates/emails/low_score_alert.html`
- Sistema de interpolación de variables
- Diseño responsive

### E6.4: AssessmentAttemptProcessor
- `internal/application/processor/assessment_attempt_processor.go`
- Detección de puntajes bajos (<60%)
- Notificación a docente por email
- Notificación a docente por push
- Actualización de estadísticas
- Tests unitarios

### E6.5: StudentEnrolledProcessor
- `internal/application/processor/student_enrolled_processor.go`
- Email de bienvenida al estudiante
- Inicialización de progreso
- Notificación a docente
- Tests unitarios

---

## 🔑 Tecnologías y Dependencias

### Nuevas Dependencias Go
```bash
go get github.com/sendgrid/sendgrid-go
go get firebase.google.com/go/v4
```

### Variables de Entorno Requeridas
```bash
# SendGrid
SENDGRID_API_KEY=SG.xxx
EMAIL_FROM=noreply@edugo.com
EMAIL_FROM_NAME=EduGo

# Firebase
FIREBASE_CREDENTIALS_FILE=/path/to/firebase-credentials.json
```

---

## 📋 Commits Sugeridos

**Commit 1: Email Service**
```
feat(fase-6): implementar servicio de email con SendGrid

- Agregar EmailService interface
- Implementar SendGrid client
- Agregar templates de email (welcome_student, low_score_alert)

Refs: PT-009, documents/mejoras/DEUDA_TECNICA.md
```

**Commit 2: Push Service**
```
feat(fase-6): implementar push notifications con Firebase

- Agregar PushService interface
- Implementar Firebase Cloud Messaging client
- Soporte para notificaciones a usuarios y topics

Refs: PT-009, documents/mejoras/DEUDA_TECNICA.md
```

**Commit 3: AssessmentAttemptProcessor**
```
feat(fase-6): implementar AssessmentAttemptProcessor

- Notificar docente si score < 60%
- Enviar email y push notification
- Actualizar estadísticas de material
- Registrar evento de analytics

Refs: PT-009
```

**Commit 4: StudentEnrolledProcessor**
```
feat(fase-6): implementar StudentEnrolledProcessor

- Enviar email de bienvenida
- Inicializar progreso del estudiante
- Notificar al docente con push

Refs: PT-009
```

**Commit 5: Tests**
```
test(fase-6): agregar tests para processors de notificaciones

- Tests para AssessmentAttemptProcessor
- Tests para StudentEnrolledProcessor
- Mocks para EmailService y PushService

Refs: PT-009, documents/mejoras/DEUDA_TECNICA.md DT-009
```

---

## ✅ Checklist de Validación

### Email Service
- [ ] Interface EmailService definida
- [ ] SendEmail implementado
- [ ] SendTemplatedEmail implementado
- [ ] Templates HTML creados
- [ ] Variables interpoladas correctamente
- [ ] Tests con mocks pasan

### Push Service
- [ ] Interface PushService definida
- [ ] SendToUser implementado
- [ ] SendToUsers implementado
- [ ] SendToTopic implementado
- [ ] Tests con mocks pasan

### AssessmentAttemptProcessor
- [ ] Lógica de score < 60% correcta
- [ ] Email enviado correctamente
- [ ] Push notification enviada
- [ ] Estadísticas actualizadas
- [ ] Analytics registrado
- [ ] Tests unitarios pasan

### StudentEnrolledProcessor
- [ ] Email de bienvenida enviado
- [ ] Progreso inicializado
- [ ] Notificación a docente enviada
- [ ] Tests unitarios pasan

### General
- [ ] `make build` exitoso
- [ ] `make test` todos pasan
- [ ] `make lint` sin errores
- [ ] Cobertura >70%
- [ ] Documentación actualizada

---

## 🎯 Criterios de Aceptación

La Fase 6 se considera **COMPLETADA** cuando:

1. ✅ EmailService funcional con SendGrid
2. ✅ PushService funcional con Firebase
3. ✅ Templates de email creados y funcionando
4. ✅ AssessmentAttemptProcessor notifica correctamente
5. ✅ StudentEnrolledProcessor envía bienvenidas
6. ✅ Tests >70% cobertura
7. ✅ PR aprobado y mergeado a `dev`
8. ✅ Tag `fase-6-complete` creado

---

## 💰 Costos Estimados

| Servicio | Costo Mensual |
|----------|--------------|
| SendGrid (Plan Essentials) | ~$20/mes (hasta 50K emails) |
| Firebase Cloud Messaging | Gratis (mensajes ilimitados) |
| **Total** | **~$20/mes** |

---

## 🚨 Gestión de Riesgos

### Riesgo: Rate limiting en envío de emails
**Mitigación:**
- Implementar queue para emails
- Respetar límites de SendGrid
- Retry con backoff exponencial

### Riesgo: Notificaciones push no llegan
**Mitigación:**
- Validar tokens de dispositivos
- Logging detallado de envíos
- Manejo de tokens expirados

### Riesgo: Spam o emails no deseados
**Mitigación:**
- Validar preferencias de usuario
- Implementar unsubscribe
- Respetar regulaciones (GDPR, CAN-SPAM)

---

## 📚 Referencias

- **Plan Técnico Detallado:** [PLAN_TECNICO.md](./PLAN_TECNICO.md)
- **Tareas:** [TAREAS.md](./TAREAS.md)
- **Validación:** [VALIDACION.md](./VALIDACION.md)
- **Deuda Técnica:** [../../documents/mejoras/DEUDA_TECNICA.md](../../documents/mejoras/DEUDA_TECNICA.md)

---

## ⏭️ Siguientes Pasos

Después de completar la Fase 6, revisar el roadmap para priorizar:
- Fase 3: Testing y Calidad
- Fase 4: Observabilidad y Resiliencia
- Nuevas funcionalidades según demanda

---

**Última actualización:** 2025-12-23
**Versión:** 1.0
**Origen:** PT-009 edugo_analisis
