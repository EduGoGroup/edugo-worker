# Tareas Sprint 03 - OpenAI Integration

## TASK-03-001: Implementar AIService
**Prioridad:** HIGH | **Estimación:** 4h

#### Implementación
Archivo: `/Users/jhoanmedina/source/EduGo/repos-separados/edugo-worker/internal/services/ai_service.go`

```go
package services

import (
    "context"
    openai "github.com/sashabaranov/go-openai"
)

type AIService struct {
    client *openai.Client
}

func (s *AIService) GenerateSummary(ctx context.Context, text string) (*Summary, error) {
    prompt := buildSummaryPrompt(text)
    
    resp, err := s.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
        Model: openai.GPT4TurboPreview,
        Messages: []openai.ChatCompletionMessage{
            {Role: "system", Content: "Eres asistente educativo..."},
            {Role: "user", Content: prompt},
        },
        Temperature: 0.3,
        MaxTokens: 4000,
        ResponseFormat: &openai.ChatCompletionResponseFormat{Type: "json_object"},
    })
    
    if err != nil {
        return nil, err
    }
    
    // Parse JSON response
    var summary Summary
    json.Unmarshal([]byte(resp.Choices[0].Message.Content), &summary)
    
    return &summary, nil
}
```

#### Criterios
- [ ] Cliente OpenAI configurado
- [ ] Prompt engineering para resúmenes educativos
- [ ] Response format: JSON
- [ ] Timeout 60s
- [ ] Retry con backoff exponencial

---

## TASK-03-002: Rate Limiting
**Prioridad:** HIGH | **Estimación:** 2h

```go
type RateLimiter struct {
    limiter *rate.Limiter  // 60 req/min
}

func (l *RateLimiter) Wait(ctx context.Context) error {
    return l.limiter.Wait(ctx)
}
```

---

## TASK-03-003: Tests con Mocks
**Prioridad:** MEDIUM | **Estimación:** 3h

```go
func TestAIService_GenerateSummary(t *testing.T) {
    // Mock de OpenAI API
    mockClient := &MockOpenAIClient{}
    service := NewAIService(mockClient)
    
    summary, err := service.GenerateSummary(ctx, sampleText)
    
    require.NoError(t, err)
    assert.Len(t, summary.Sections, 5)
}
```

**Tiempo total:** 9h
