# Reporte de Coverage - Worker

## Template

| Package | Coverage | Objetivo |
|---------|----------|----------|
| services/pdf_processor | XX% | >80% |
| services/ai_service | XX% | >80% |
| services/quiz_generator | XX% | >80% |
| consumer | XX% | >70% |
| **TOTAL** | **XX%** | **>75%** |

## Comandos
```bash
go test ./... -coverprofile=coverage.out
go tool cover -html=coverage.out
```

## Gaps
[Identificar áreas sin coverage y plan de mejora]
