# Validación - Fase 6: Sistemas de Notificaciones

> **Objetivo:** Validar que todos los sistemas de notificación funcionen correctamente antes de mergear a `dev`

---

## 🧪 Tests Unitarios

### Email Service
```bash
go test -v ./internal/infrastructure/email/... -run TestEmailService
```

**Verificar:**
- [ ] SendEmail envía correctamente
- [ ] SendTemplatedEmail interpola variables
- [ ] Templates se renderizan sin errores
- [ ] Retry funciona con errores transitorios
- [ ] Errores de SendGrid manejados

### Push Service
```bash
go test -v ./internal/infrastructure/push/... -run TestPushService
```

**Verificar:**
- [ ] SendToUser funciona
- [ ] SendToUsers funciona (batch)
- [ ] SendToTopic funciona
- [ ] Tokens expirados manejados
- [ ] Errores de Firebase manejados

### AssessmentAttemptProcessor
```bash
go test -v ./internal/application/processor/... -run TestAssessmentAttemptProcessor
```

**Verificar:**
- [ ] Score calculado correctamente
- [ ] Detección de score < 60% funciona
- [ ] Email enviado cuando score < 60%
- [ ] Push enviado cuando score < 60%
- [ ] NO envía notificaciones cuando score >= 60%
- [ ] Estadísticas actualizadas
- [ ] Analytics registrado

### StudentEnrolledProcessor
```bash
go test -v ./internal/application/processor/... -run TestStudentEnrolledProcessor
```

**Verificar:**
- [ ] Email de bienvenida enviado
- [ ] Variables correctas en template
- [ ] Progreso inicializado
- [ ] Notificación a docente enviada
- [ ] Errores manejados sin fallar

---

## 🔗 Tests de Integración

### Flujo Assessment con Score Bajo
```bash
go test -v ./internal/application/processor/... -run TestAssessmentAttemptProcessor_Integration_LowScore
```

**Verificar:**
1. [ ] Evento `assessment_attempt.completed` procesado
2. [ ] Score < 60% detectado
3. [ ] Email enviado al docente
4. [ ] Push notification enviada
5. [ ] Datos correctos en notificaciones

### Flujo Student Enrollment
```bash
go test -v ./internal/application/processor/... -run TestStudentEnrolledProcessor_Integration
```

**Verificar:**
1. [ ] Evento `student.enrolled` procesado
2. [ ] Email de bienvenida enviado
3. [ ] Progreso inicializado en DB
4. [ ] Docente notificado

### Test Manual (Opcional con Servicios Reales)

**SendGrid:**
1. Configurar API key real
2. Enviar email de prueba
3. Verificar recepción
4. Verificar formato HTML

**Firebase:**
1. Configurar credenciales reales
2. Registrar dispositivo de prueba
3. Enviar notificación
4. Verificar recepción en dispositivo

---

## 📊 Cobertura de Tests

```bash
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

**Criterio:** Cobertura >70%

**Verificar áreas críticas:**
- [ ] EmailService: >75%
- [ ] PushService: >75%
- [ ] AssessmentAttemptProcessor: >80%
- [ ] StudentEnrolledProcessor: >80%

---

## 🏗️ Build y Lint

### Build
```bash
make build
```

**Verificar:**
- [ ] Compilación sin errores
- [ ] Binario generado correctamente
- [ ] Sin warnings

### Lint
```bash
make lint
```

**Verificar:**
- [ ] Sin errores de golangci-lint
- [ ] Sin errores de gofmt
- [ ] Sin imports circulares

---

## 🔒 Validación de Seguridad

### Credenciales
- [ ] No hay API keys hardcoded
- [ ] Variables de entorno documentadas
- [ ] Credenciales Firebase en archivo separado
- [ ] Secrets no en commits
- [ ] `.env.example` creado

### Templates de Email
- [ ] No hay XSS en interpolación
- [ ] Variables sanitizadas
- [ ] Links validados

---

## 📧 Validación de Templates

### welcome_student.html
- [ ] Todas las variables se interpolan
- [ ] Diseño responsive funciona
- [ ] Links funcionan
- [ ] Renderiza en diferentes clientes de email

### low_score_alert.html
- [ ] Todas las variables se interpolan
- [ ] Formato de score correcto (%)
- [ ] Diseño claro y profesional
- [ ] Renderiza en diferentes clientes de email

---

## 📝 Documentación

### Código
- [ ] Comentarios en interfaces públicas
- [ ] Ejemplos de uso documentados
- [ ] Errores documentados

### README
- [ ] Configuración de SendGrid documentada
- [ ] Configuración de Firebase documentada
- [ ] Variables de entorno listadas
- [ ] Costos estimados documentados
- [ ] Rate limits documentados
- [ ] Templates documentados

---

## 🚀 Pre-PR Checklist

Antes de crear el Pull Request:

- [ ] Todos los tests pasan
- [ ] Cobertura >70%
- [ ] Build exitoso
- [ ] Lint limpio
- [ ] Documentación actualizada
- [ ] Commits bien escritos
- [ ] Branch sincronizado con `dev`
- [ ] No hay conflictos
- [ ] Variables de entorno documentadas
- [ ] Templates validados
- [ ] Plan técnico revisado

---

## ✅ Criterios de Aceptación Final

La fase está lista para merge cuando:

1. ✅ EmailService funciona con SendGrid
2. ✅ PushService funciona con Firebase
3. ✅ Templates renderizan correctamente
4. ✅ AssessmentAttemptProcessor notifica apropiadamente
5. ✅ StudentEnrolledProcessor envía bienvenidas
6. ✅ Todos los tests pasan
7. ✅ Cobertura >70%
8. ✅ Build y lint exitosos
9. ✅ Documentación completa
10. ✅ Code review aprobado
11. ✅ CI/CD verde

---

**Última actualización:** 2025-12-23
