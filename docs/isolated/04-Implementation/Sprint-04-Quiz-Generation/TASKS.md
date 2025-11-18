# Tareas Sprint 04

## TASK-04-001: QuizGenerator Service
**Estimación:** 4h

```go
// internal/services/quiz_generator.go
func (g *QuizGenerator) GenerateQuiz(ctx context.Context, text string) (*Quiz, error) {
    prompt := buildQuizPrompt(text)
    
    resp, err := g.openaiClient.CreateChatCompletion(ctx, request)
    // Parse response JSON
    // Validar: 5-10 preguntas, 4 opciones, 1 correcta
}
```

## Criterios
- [ ] 5-10 preguntas generadas
- [ ] 4 opciones por pregunta
- [ ] Distractores plausibles
- [ ] Feedback para respuestas

**Tiempo:** 4h
