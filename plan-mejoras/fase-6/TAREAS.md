# Tareas - Fase 6: Sistemas de Notificaciones

> **Rama:** `feature/fase-6-sistemas-notificaciones`
> **Origen:** PT-009

---

## 📋 Lista de Tareas

### 1. Servicio de Email (SendGrid)
- [ ] Crear interface `EmailService`
- [ ] Implementar `SendEmail(to, subject, body)`
- [ ] Implementar `SendTemplatedEmail(to, templateID, data)`
- [ ] Agregar configuración SendGrid en config.yaml
- [ ] Implementar retry con backoff
- [ ] Tests con mocks
- [ ] Documentar uso

### 2. Templates de Email
- [ ] Crear directorio `templates/emails/`
- [ ] Diseñar template `welcome_student.html`
  - [ ] Agregar variables: StudentName, UnitName, TeacherName, AppURL
  - [ ] Diseño responsive
- [ ] Diseñar template `low_score_alert.html`
  - [ ] Agregar variables: StudentName, MaterialTitle, Score, AttemptNumber
  - [ ] Diseño responsive
- [ ] Implementar sistema de interpolación de variables
- [ ] Tests de renderizado de templates

### 3. Servicio de Push Notifications (Firebase)
- [ ] Obtener credenciales Firebase
- [ ] Crear interface `PushService`
- [ ] Implementar `SendToUser(userID, notification)`
- [ ] Implementar `SendToUsers(userIDs, notification)`
- [ ] Implementar `SendToTopic(topic, notification)`
- [ ] Agregar configuración Firebase en config.yaml
- [ ] Implementar manejo de tokens expirados
- [ ] Tests con mocks
- [ ] Documentar uso

### 4. AssessmentAttemptProcessor
- [ ] Crear `internal/application/processor/assessment_attempt_processor.go`
- [ ] Implementar lógica de cálculo de score
- [ ] Implementar detección de score < 60%
- [ ] Implementar `notifyTeacher()`
  - [ ] Obtener docente de la unidad
  - [ ] Obtener datos del estudiante
  - [ ] Enviar email con template
  - [ ] Enviar push notification
- [ ] Implementar `updateMaterialStats()`
- [ ] Implementar `recordAnalyticsEvent()`
- [ ] Agregar logging detallado
- [ ] Tests unitarios
- [ ] Tests con mocks de servicios

### 5. StudentEnrolledProcessor
- [ ] Crear `internal/application/processor/student_enrolled_processor.go`
- [ ] Implementar obtención de datos (estudiante, unidad, docente)
- [ ] Implementar envío de email de bienvenida
- [ ] Implementar `initializeProgress()`
- [ ] Implementar notificación a docente
- [ ] Agregar logging detallado
- [ ] Tests unitarios
- [ ] Tests con mocks de servicios

### 6. Integración con Event System
- [ ] Registrar AssessmentAttemptProcessor en ProcessorRegistry
- [ ] Registrar StudentEnrolledProcessor en ProcessorRegistry
- [ ] Configurar routing de eventos
- [ ] Tests de integración end-to-end

### 7. Configuración y Documentación
- [ ] Actualizar `config/config.yaml`
- [ ] Documentar variables de entorno
- [ ] Actualizar `.env.example`
- [ ] Actualizar README con instrucciones
- [ ] Documentar costos de servicios
- [ ] Documentar rate limits
- [ ] Documentar políticas de retry

### 8. Tests y Calidad
- [ ] Tests unitarios para EmailService
- [ ] Tests unitarios para PushService
- [ ] Tests para AssessmentAttemptProcessor
- [ ] Tests para StudentEnrolledProcessor
- [ ] Validar cobertura >70%
- [ ] Ejecutar `make lint`
- [ ] Ejecutar `make build`
- [ ] Ejecutar `make test`

---

## 🔄 Orden de Ejecución Recomendado

1. **Semana 1:** EmailService + Templates
2. **Semana 2:** PushService + Firebase setup
3. **Semana 3:** AssessmentAttemptProcessor
4. **Semana 4:** StudentEnrolledProcessor + Tests de integración

---

## ✅ Checklist Final

Antes de crear PR:

- [ ] Todos los tests unitarios pasan
- [ ] Tests de integración pasan
- [ ] Cobertura >70%
- [ ] `make build` exitoso
- [ ] `make lint` sin errores
- [ ] Documentación actualizada
- [ ] Variables de entorno documentadas
- [ ] Credenciales Firebase configuradas
- [ ] Templates de email validados
- [ ] Commits atómicos y bien descritos
- [ ] Branch actualizado con `dev`

---

**Última actualización:** 2025-12-23
