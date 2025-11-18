# TASKS Sprint-00: Integrar con infrastructure

## TASK-001: Actualizar go.mod

```bash
go get github.com/EduGoGroup/edugo-infrastructure/schemas@v0.2.0
go get github.com/EduGoGroup/edugo-shared/logger@v0.7.0
go get github.com/EduGoGroup/edugo-shared/evaluation@v0.7.0
go get github.com/EduGoGroup/edugo-shared/messaging/rabbit@v0.7.0
go get github.com/EduGoGroup/edugo-shared/database/mongodb@v0.7.0
go mod tidy
```

## TASK-002: Integrar validador de eventos

```go
import "github.com/EduGoGroup/edugo-infrastructure/schemas"

// Inicializar
validator, _ := schemas.NewValidator()

// Al consumir eventos
if err := validator.Validate(event, "material-uploaded-v1"); err != nil {
    logger.Error("invalid event", err)
    return nil // No reintentar
}

// Al publicar eventos
if err := validator.Validate(event, "assessment-generated-v1"); err != nil {
    return fmt.Errorf("invalid event: %w", err)
}
```

## TASK-003: Usar DLQ de shared

Usar Dead Letter Queue de shared/messaging/rabbit v0.7.0:

```go
import "github.com/EduGoGroup/edugo-shared/messaging/rabbit"

config := rabbit.ConsumerConfig{
    DLQ: rabbit.DLQConfig{
        Enabled: true,
        MaxRetries: 3,
    },
}
```

## TASK-004: Usar módulo evaluation

Importar modelos de shared/evaluation:

```go
import "github.com/EduGoGroup/edugo-shared/evaluation"

assessment := &evaluation.Assessment{
    Title: "Quiz Generated",
    PassingScore: 70,
}
```

## TASK-005: Actualizar README

Documentar uso de infrastructure para MongoDB schemas.

---

Estimación: 1 hora
