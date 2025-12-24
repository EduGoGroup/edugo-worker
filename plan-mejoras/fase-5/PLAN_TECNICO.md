# Plan Técnico Detallado: Fase 1 Core

> **Origen:** Plan de trabajo PT-008 de edugo_analisis
> **Integrado el:** 2025-12-23
> **Descripción:** Implementación técnica detallada de la Fase 1 con código específico y ejemplos

---

## 📋 Contexto

Este documento proporciona la **implementación técnica detallada** de la Fase 1. Complementa el README.md general con código específico, ejemplos de implementación y guías paso a paso.

**Referencia al Plan General:** [README.md](./README.md)

---

## Fase 1.1: Integración S3 Real

### Paso 1.1.1: Crear cliente S3

**Archivo:** `internal/infrastructure/storage/s3_client.go`

```go
package storage

import (
    "context"
    "io"

    "github.com/aws/aws-sdk-go-v2/config"
    "github.com/aws/aws-sdk-go-v2/service/s3"
)

type S3Client interface {
    DownloadFile(ctx context.Context, bucket, key string) (io.ReadCloser, error)
    GetFileSize(ctx context.Context, bucket, key string) (int64, error)
}

type s3ClientImpl struct {
    client *s3.Client
    logger logger.Logger
}

func NewS3Client(ctx context.Context, logger logger.Logger) (S3Client, error) {
    cfg, err := config.LoadDefaultConfig(ctx)
    if err != nil {
        return nil, err
    }

    return &s3ClientImpl{
        client: s3.NewFromConfig(cfg),
        logger: logger,
    }, nil
}

func (c *s3ClientImpl) DownloadFile(ctx context.Context, bucket, key string) (io.ReadCloser, error) {
    output, err := c.client.GetObject(ctx, &s3.GetObjectInput{
        Bucket: &bucket,
        Key:    &key,
    })
    if err != nil {
        c.logger.Error("failed to download from S3", "bucket", bucket, "key", key, "error", err)
        return nil, err
    }

    return output.Body, nil
}

func (c *s3ClientImpl) GetFileSize(ctx context.Context, bucket, key string) (int64, error) {
    output, err := c.client.HeadObject(ctx, &s3.HeadObjectInput{
        Bucket: &bucket,
        Key:    &key,
    })
    if err != nil {
        return 0, err
    }

    return *output.ContentLength, nil
}
```

### Paso 1.1.2: Configurar variables de entorno

**Archivo:** `config/config.yaml`

```yaml
aws:
  region: ${AWS_REGION:us-east-1}
  s3:
    bucket: ${S3_BUCKET:edugo-materials}
    timeout_seconds: 300
```

---

## Fase 1.2: Extracción de Texto PDF

### Paso 1.2.1: Agregar dependencia

```bash
go get github.com/pdfcpu/pdfcpu
# o alternativa:
go get github.com/unidoc/unipdf/v3
```

### Paso 1.2.2: Crear servicio de extracción

**Archivo:** `internal/domain/service/pdf_extractor.go`

```go
package service

import (
    "context"
    "io"
)

type PDFExtractor interface {
    ExtractText(ctx context.Context, reader io.Reader) (string, error)
    ExtractMetadata(ctx context.Context, reader io.Reader) (*PDFMetadata, error)
}

type PDFMetadata struct {
    Title      string
    Author     string
    PageCount  int
    WordCount  int
    Language   string
    CreatedAt  string
}
```

**Implementación:** `internal/infrastructure/pdf/pdf_extractor_impl.go`

```go
package pdf

import (
    "bytes"
    "context"
    "io"
    "strings"

    "github.com/pdfcpu/pdfcpu/pkg/api"
    "github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
)

type pdfExtractorImpl struct {
    logger logger.Logger
}

func NewPDFExtractor(logger logger.Logger) service.PDFExtractor {
    return &pdfExtractorImpl{logger: logger}
}

func (e *pdfExtractorImpl) ExtractText(ctx context.Context, reader io.Reader) (string, error) {
    // Leer todo el contenido
    buf := new(bytes.Buffer)
    if _, err := io.Copy(buf, reader); err != nil {
        return "", err
    }

    // Extraer texto de todas las páginas
    var textBuilder strings.Builder

    // Usar pdfcpu para extraer texto
    // ... implementación específica

    return textBuilder.String(), nil
}

func (e *pdfExtractorImpl) ExtractMetadata(ctx context.Context, reader io.Reader) (*service.PDFMetadata, error) {
    // Implementar extracción de metadata
    return &service.PDFMetadata{}, nil
}
```

---

## Fase 1.3: Integración OpenAI Real

### Paso 1.3.1: Agregar dependencia

```bash
go get github.com/sashabaranov/go-openai
```

### Paso 1.3.2: Crear cliente OpenAI

**Archivo:** `internal/infrastructure/ai/openai_client.go`

```go
package ai

import (
    "context"

    openai "github.com/sashabaranov/go-openai"
)

type OpenAIClient interface {
    GenerateSummary(ctx context.Context, content string, options SummaryOptions) (*SummaryResult, error)
    GenerateQuiz(ctx context.Context, content string, options QuizOptions) (*QuizResult, error)
}

type SummaryOptions struct {
    Language    string
    MaxLength   int
    KeyPoints   int
}

type SummaryResult struct {
    Summary     string
    KeyPoints   []string
    Language    string
    WordCount   int
    Model       string
    ProcessingTimeMs int
}

type QuizOptions struct {
    NumQuestions   int
    Difficulty     string // easy, medium, hard
    QuestionTypes  []string // multiple_choice, true_false, open
}

type QuizResult struct {
    Questions        []Question
    TotalQuestions   int
    TotalPoints      int
    Model            string
    ProcessingTimeMs int
}

type Question struct {
    ID           string
    Text         string
    Type         string
    Options      []Option
    CorrectAnswer string
    Explanation  string
    Points       int
    Difficulty   string
}

type Option struct {
    ID   string
    Text string
}
```

**Implementación:** `internal/infrastructure/ai/openai_client_impl.go`

```go
package ai

import (
    "context"
    "encoding/json"
    "time"

    openai "github.com/sashabaranov/go-openai"
)

type openAIClientImpl struct {
    client *openai.Client
    model  string
    logger logger.Logger
}

func NewOpenAIClient(apiKey string, logger logger.Logger) OpenAIClient {
    return &openAIClientImpl{
        client: openai.NewClient(apiKey),
        model:  openai.GPT4,
        logger: logger,
    }
}

func (c *openAIClientImpl) GenerateSummary(ctx context.Context, content string, options SummaryOptions) (*SummaryResult, error) {
    start := time.Now()

    prompt := buildSummaryPrompt(content, options)

    resp, err := c.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
        Model: c.model,
        Messages: []openai.ChatCompletionMessage{
            {
                Role:    openai.ChatMessageRoleSystem,
                Content: "You are an educational content summarizer. Generate structured summaries with key points.",
            },
            {
                Role:    openai.ChatMessageRoleUser,
                Content: prompt,
            },
        },
        Temperature: 0.7,
    })
    if err != nil {
        c.logger.Error("OpenAI summary generation failed", "error", err)
        return nil, err
    }

    // Parsear respuesta
    result := &SummaryResult{
        Model:            c.model,
        ProcessingTimeMs: int(time.Since(start).Milliseconds()),
    }
    // ... parsear contenido de resp.Choices[0].Message.Content

    return result, nil
}

func (c *openAIClientImpl) GenerateQuiz(ctx context.Context, content string, options QuizOptions) (*QuizResult, error) {
    start := time.Now()

    prompt := buildQuizPrompt(content, options)

    resp, err := c.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
        Model: c.model,
        Messages: []openai.ChatCompletionMessage{
            {
                Role:    openai.ChatMessageRoleSystem,
                Content: "You are an educational quiz generator. Create well-structured quizzes in JSON format.",
            },
            {
                Role:    openai.ChatMessageRoleUser,
                Content: prompt,
            },
        },
        Temperature: 0.8,
        ResponseFormat: &openai.ChatCompletionResponseFormat{
            Type: openai.ChatCompletionResponseFormatTypeJSONObject,
        },
    })
    if err != nil {
        c.logger.Error("OpenAI quiz generation failed", "error", err)
        return nil, err
    }

    // Parsear respuesta JSON
    result := &QuizResult{
        Model:            c.model,
        ProcessingTimeMs: int(time.Since(start).Milliseconds()),
    }
    if err := json.Unmarshal([]byte(resp.Choices[0].Message.Content), result); err != nil {
        return nil, err
    }

    return result, nil
}

func buildSummaryPrompt(content string, options SummaryOptions) string {
    // Construir prompt para resumen
    return ""
}

func buildQuizPrompt(content string, options QuizOptions) string {
    // Construir prompt para quiz
    return ""
}
```

### Paso 1.3.3: Configurar API Key

**Archivo:** `config/config.yaml`

```yaml
openai:
  api_key: ${OPENAI_API_KEY}
  model: ${OPENAI_MODEL:gpt-4}
  timeout_seconds: 120
  max_retries: 3
```

---

## Fase 1.4: Actualizar MaterialUploadedProcessor

### Paso 1.4.1: Refactorizar processor

**Archivo:** `internal/application/processor/material_uploaded_processor.go`

```go
package processor

type MaterialUploadedProcessor struct {
    materialRepo    repository.MaterialRepository
    summaryRepo     repository.MaterialSummaryRepository
    assessmentRepo  repository.MaterialAssessmentRepository
    s3Client        storage.S3Client
    pdfExtractor    service.PDFExtractor
    openAIClient    ai.OpenAIClient
    logger          logger.Logger
}

func NewMaterialUploadedProcessor(
    materialRepo repository.MaterialRepository,
    summaryRepo repository.MaterialSummaryRepository,
    assessmentRepo repository.MaterialAssessmentRepository,
    s3Client storage.S3Client,
    pdfExtractor service.PDFExtractor,
    openAIClient ai.OpenAIClient,
    logger logger.Logger,
) *MaterialUploadedProcessor {
    return &MaterialUploadedProcessor{
        materialRepo:    materialRepo,
        summaryRepo:     summaryRepo,
        assessmentRepo:  assessmentRepo,
        s3Client:        s3Client,
        pdfExtractor:    pdfExtractor,
        openAIClient:    openAIClient,
        logger:          logger,
    }
}

func (p *MaterialUploadedProcessor) Process(ctx context.Context, event events.MaterialUploaded) error {
    p.logger.Info("processing material upload", "material_id", event.MaterialID)

    // 1. Actualizar estado a "processing"
    if err := p.materialRepo.UpdateStatus(ctx, event.MaterialID, "processing"); err != nil {
        return err
    }

    // 2. Descargar PDF de S3
    reader, err := p.s3Client.DownloadFile(ctx, event.Bucket, event.Key)
    if err != nil {
        p.handleError(ctx, event.MaterialID, "failed to download from S3", err)
        return err
    }
    defer reader.Close()

    // 3. Extraer texto del PDF
    content, err := p.pdfExtractor.ExtractText(ctx, reader)
    if err != nil {
        p.handleError(ctx, event.MaterialID, "failed to extract PDF text", err)
        return err
    }

    // 4. Generar resumen con OpenAI
    summary, err := p.openAIClient.GenerateSummary(ctx, content, ai.SummaryOptions{
        Language:  "es",
        MaxLength: 500,
        KeyPoints: 5,
    })
    if err != nil {
        p.handleError(ctx, event.MaterialID, "failed to generate summary", err)
        return err
    }

    // 5. Guardar resumen en MongoDB
    if err := p.summaryRepo.Save(ctx, &entities.MaterialSummary{
        MaterialID:       event.MaterialID,
        Summary:          summary.Summary,
        KeyPoints:        summary.KeyPoints,
        Language:         summary.Language,
        WordCount:        summary.WordCount,
        Version:          1,
        AIModel:          summary.Model,
        ProcessingTimeMs: summary.ProcessingTimeMs,
    }); err != nil {
        p.handleError(ctx, event.MaterialID, "failed to save summary", err)
        return err
    }

    // 6. Generar quiz con OpenAI
    quiz, err := p.openAIClient.GenerateQuiz(ctx, content, ai.QuizOptions{
        NumQuestions:  10,
        Difficulty:    "medium",
        QuestionTypes: []string{"multiple_choice", "true_false"},
    })
    if err != nil {
        p.handleError(ctx, event.MaterialID, "failed to generate quiz", err)
        return err
    }

    // 7. Guardar quiz en MongoDB
    if err := p.assessmentRepo.Save(ctx, &entities.MaterialAssessment{
        MaterialID:       event.MaterialID,
        Questions:        mapQuestions(quiz.Questions),
        TotalQuestions:   quiz.TotalQuestions,
        TotalPoints:      quiz.TotalPoints,
        Version:          1,
        AIModel:          quiz.Model,
        ProcessingTimeMs: quiz.ProcessingTimeMs,
    }); err != nil {
        p.handleError(ctx, event.MaterialID, "failed to save assessment", err)
        return err
    }

    // 8. Actualizar estado a "completed"
    if err := p.materialRepo.UpdateStatus(ctx, event.MaterialID, "completed"); err != nil {
        return err
    }

    p.logger.Info("material processing completed",
        "material_id", event.MaterialID,
        "summary_words", summary.WordCount,
        "quiz_questions", quiz.TotalQuestions,
    )

    return nil
}

func (p *MaterialUploadedProcessor) handleError(ctx context.Context, materialID, message string, err error) {
    p.logger.Error(message, "material_id", materialID, "error", err)
    _ = p.materialRepo.UpdateStatus(ctx, materialID, "error")
}
```

---

## Fase 1.5: Tests de Integración

### Paso 1.5.1: Tests con mocks

**Archivo:** `internal/application/processor/material_uploaded_processor_test.go`

```go
package processor_test

func TestMaterialUploadedProcessor_Process(t *testing.T) {
    tests := []struct {
        name          string
        setupMocks    func(*mocks.MockS3Client, *mocks.MockPDFExtractor, *mocks.MockOpenAIClient)
        expectedError bool
    }{
        {
            name: "successful processing",
            setupMocks: func(s3 *mocks.MockS3Client, pdf *mocks.MockPDFExtractor, ai *mocks.MockOpenAIClient) {
                s3.EXPECT().DownloadFile(gomock.Any(), gomock.Any(), gomock.Any()).Return(io.NopCloser(strings.NewReader("pdf content")), nil)
                pdf.EXPECT().ExtractText(gomock.Any(), gomock.Any()).Return("extracted text", nil)
                ai.EXPECT().GenerateSummary(gomock.Any(), gomock.Any(), gomock.Any()).Return(&ai.SummaryResult{Summary: "test summary"}, nil)
                ai.EXPECT().GenerateQuiz(gomock.Any(), gomock.Any(), gomock.Any()).Return(&ai.QuizResult{Questions: []ai.Question{}}, nil)
            },
            expectedError: false,
        },
        {
            name: "S3 download failure",
            setupMocks: func(s3 *mocks.MockS3Client, pdf *mocks.MockPDFExtractor, ai *mocks.MockOpenAIClient) {
                s3.EXPECT().DownloadFile(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, errors.New("S3 error"))
            },
            expectedError: true,
        },
        // ... más casos
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Implementar test
        })
    }
}
```

### Paso 1.5.2: Tests de integración E2E (opcional)

```go
// Solo correr en CI con servicios reales
func TestMaterialUploadedProcessor_E2E(t *testing.T) {
    if testing.Short() {
        t.Skip("skipping E2E test")
    }
    // Test con S3 y OpenAI reales
}
```

---

## Fase 1.6: Commits y PR

### Paso 1.6.1: Commits atómicos sugeridos

```bash
# Commit 1: S3 client
git commit -m "feat(storage): implementar cliente S3 para descarga de archivos

- Agregar S3Client interface
- Implementar DownloadFile y GetFileSize
- Configuración de AWS

Refs: PT-008, documents/mejoras/DEUDA_TECNICA.md DT-003"

# Commit 2: PDF extractor
git commit -m "feat(pdf): implementar extractor de texto PDF

- Agregar PDFExtractor interface
- Implementar ExtractText con pdfcpu
- Agregar ExtractMetadata

Refs: PT-008, documents/mejoras/DEUDA_TECNICA.md DT-003"

# Commit 3: OpenAI client
git commit -m "feat(ai): implementar cliente OpenAI real

- Agregar OpenAIClient interface
- Implementar GenerateSummary
- Implementar GenerateQuiz con respuesta JSON
- Prompts optimizados para educación

Refs: PT-008, documents/mejoras/DEUDA_TECNICA.md DT-002"

# Commit 4: Processor actualizado
git commit -m "feat(processor): actualizar MaterialUploadedProcessor con integraciones reales

- Integrar S3Client para descarga
- Integrar PDFExtractor para texto
- Integrar OpenAI para resumen y quiz
- Manejo de errores con actualización de estado

Refs: PT-008, documents/mejoras/REFACTORING.md RF-002"

# Commit 5: Tests
git commit -m "test: agregar tests para MaterialUploadedProcessor

- Tests unitarios con mocks
- Tests de integración opcionales
- Cobertura >60%

Refs: PT-008, documents/mejoras/DEUDA_TECNICA.md DT-009"
```

---

## Checklist Técnico Final

- [ ] **S3Client**
  - [ ] Interface definida
  - [ ] Implementación con aws-sdk-go-v2
  - [ ] DownloadFile funcional
  - [ ] GetFileSize funcional
  - [ ] Tests unitarios

- [ ] **PDFExtractor**
  - [ ] Interface definida
  - [ ] Implementación con pdfcpu/unidoc
  - [ ] ExtractText funcional
  - [ ] ExtractMetadata funcional
  - [ ] Tests con PDFs de ejemplo

- [ ] **OpenAIClient**
  - [ ] Interface definida
  - [ ] Implementación con go-openai
  - [ ] GenerateSummary funcional
  - [ ] GenerateQuiz funcional
  - [ ] Prompts optimizados
  - [ ] Tests con mocks

- [ ] **MaterialUploadedProcessor**
  - [ ] Refactorizado con nuevas dependencias
  - [ ] Flujo completo S3 → PDF → OpenAI → MongoDB
  - [ ] Manejo de errores por etapa
  - [ ] UpdateStatus en cada paso
  - [ ] Tests unitarios
  - [ ] Tests de integración

- [ ] **MaterialReprocessProcessor**
  - [ ] Actualizado (usa mismo flujo)

- [ ] **Configuración**
  - [ ] Variables de entorno documentadas
  - [ ] config.yaml actualizado
  - [ ] Valores por defecto razonables

- [ ] **Build y Tests**
  - [ ] `make build` exitoso
  - [ ] `make test` todos pasan
  - [ ] `make lint` limpio
  - [ ] Cobertura >60%

---

## Variables de Entorno Requeridas

```bash
# AWS
AWS_REGION=us-east-1
AWS_ACCESS_KEY_ID=xxx
AWS_SECRET_ACCESS_KEY=xxx
S3_BUCKET=edugo-materials

# OpenAI
OPENAI_API_KEY=sk-xxx
OPENAI_MODEL=gpt-4
```

---

## Costos Estimados

| Servicio | Estimado por Material |
|----------|----------------------|
| S3 Download | ~$0.0001 (1MB PDF) |
| OpenAI GPT-4 | ~$0.03-0.06 (resumen + quiz) |

**Total estimado por material:** ~$0.03-0.06

---

## Rate Limiting

- **OpenAI:** Implementar retry con backoff exponencial
- **S3:** No aplica para volumen esperado

---

## Referencias

- **Plan General:** [README.md](./README.md)
- **Tareas:** [TAREAS.md](./TAREAS.md)
- **Validación:** [VALIDACION.md](./VALIDACION.md)
- **Deuda Técnica:** [../../documents/mejoras/DEUDA_TECNICA.md](../../documents/mejoras/DEUDA_TECNICA.md)
- **Refactoring:** [../../documents/mejoras/REFACTORING.md](../../documents/mejoras/REFACTORING.md)

---

**Última actualización:** 2025-12-23
**Versión:** 1.0
**Origen:** PT-008 edugo_analisis
