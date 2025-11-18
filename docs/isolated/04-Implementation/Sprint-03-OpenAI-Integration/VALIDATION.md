# Validación Sprint 03

```bash
cd /Users/jhoanmedina/source/EduGo/repos-separados/edugo-worker

# Tests de AIService
go test ./internal/services -v -run TestAIService

# Test integración con OpenAI (requiere API key)
OPENAI_API_KEY=sk-... go test ./internal/services -v -run TestAIService_Integration -tags=integration
```

## Criterios
- [ ] AIService funcional
- [ ] Rate limiting implementado
- [ ] Retry logic funcional
- [ ] Tests con mocks pasando
