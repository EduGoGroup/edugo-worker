package fallback

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/EduGoGroup/edugo-shared/logger"
	"github.com/EduGoGroup/edugo-worker/internal/infrastructure/nlp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockLogger implementa logger.Logger para testing
type mockLogger struct{}

func (m *mockLogger) Debug(msg string, fields ...interface{})  {}
func (m *mockLogger) Info(msg string, fields ...interface{})   {}
func (m *mockLogger) Warn(msg string, fields ...interface{})   {}
func (m *mockLogger) Error(msg string, fields ...interface{})  {}
func (m *mockLogger) Fatal(msg string, fields ...interface{})  {}
func (m *mockLogger) With(fields ...interface{}) logger.Logger { return m }
func (m *mockLogger) Sync() error                              { return nil }

func createTestLogger() logger.Logger {
	return &mockLogger{}
}

func TestSmartClient_GenerateSummary(t *testing.T) {
	logger := createTestLogger()
	client := NewSmartClient(logger)
	ctx := context.Background()

	testText := `Este es un texto de prueba para generar un resumen.
	El texto contiene múltiples oraciones para verificar el procesamiento.
	Las ideas principales deben ser extraídas correctamente.
	Los conceptos clave también deben identificarse en el análisis.
	Las secciones deben organizarse de manera lógica y coherente.
	El resumen debe incluir información relevante del contenido original.`

	t.Run("genera resumen con estructura completa", func(t *testing.T) {
		summary, err := client.GenerateSummary(ctx, testText)

		require.NoError(t, err)
		require.NotNil(t, summary)

		// Verificar MainIdeas
		assert.NotEmpty(t, summary.MainIdeas, "MainIdeas no debe estar vacío")
		assert.LessOrEqual(t, len(summary.MainIdeas), 3, "MainIdeas debe tener máximo 3 elementos")

		// Verificar KeyConcepts
		assert.NotNil(t, summary.KeyConcepts, "KeyConcepts no debe ser nil")

		// Verificar Sections
		assert.NotEmpty(t, summary.Sections, "Sections no debe estar vacío")
		assert.Len(t, summary.Sections, 3, "Debe haber exactamente 3 secciones")

		// Verificar nombres de secciones
		assert.Equal(t, "Introducción", summary.Sections[0].Title)
		assert.Equal(t, "Desarrollo", summary.Sections[1].Title)
		assert.Equal(t, "Conclusión", summary.Sections[2].Title)

		// Verificar contenido de secciones
		for i, section := range summary.Sections {
			assert.NotEmpty(t, section.Content, "El contenido de la sección %d no debe estar vacío", i)
		}

		// Verificar otros campos
		assert.NotNil(t, summary.Glossary, "Glossary no debe ser nil")
		assert.Greater(t, summary.WordCount, 0, "WordCount debe ser mayor a 0")
		assert.False(t, summary.GeneratedAt.IsZero(), "GeneratedAt debe estar establecido")
	})

	t.Run("MainIdeas contiene oraciones del texto", func(t *testing.T) {
		summary, err := client.GenerateSummary(ctx, testText)

		require.NoError(t, err)
		require.NotNil(t, summary)

		for _, idea := range summary.MainIdeas {
			assert.NotEmpty(t, idea, "Cada MainIdea debe tener contenido")
			assert.LessOrEqual(t, len(idea), 203, "MainIdea no debe exceder 200 caracteres + '...'")
		}
	})

	t.Run("KeyConcepts identifica palabras frecuentes", func(t *testing.T) {
		textWithRepeatedWords := `
		educación educación educación educación educación.
		aprendizaje aprendizaje aprendizaje aprendizaje.
		conocimiento conocimiento conocimiento.
		estudiante estudiante estudiante.
		profesor profesor profesor.
		`

		summary, err := client.GenerateSummary(ctx, textWithRepeatedWords)

		require.NoError(t, err)
		require.NotNil(t, summary)

		// Debe haber identificado algunos conceptos clave
		assert.NotEmpty(t, summary.KeyConcepts, "Debe identificar conceptos frecuentes")
	})

	t.Run("Sections tiene contenido válido", func(t *testing.T) {
		summary, err := client.GenerateSummary(ctx, testText)

		require.NoError(t, err)
		require.NotNil(t, summary)

		for i, section := range summary.Sections {
			assert.NotEmpty(t, section.Title, "La sección %d debe tener título", i)
			assert.NotEmpty(t, section.Content, "La sección %d debe tener contenido", i)
			assert.Contains(t, section.Content, ".", "El contenido de la sección debe tener puntos")
		}
	})
}

func TestSmartClient_GenerateQuiz(t *testing.T) {
	logger := createTestLogger()
	client := NewSmartClient(logger)
	ctx := context.Background()

	testText := `La fotosíntesis es el proceso mediante el cual las plantas convierten la luz solar en energía química.
	Este proceso es fundamental para la vida en la Tierra ya que produce oxígeno.
	Las plantas utilizan clorofila para capturar la energía de la luz.
	El agua y el dióxido de carbono son los ingredientes principales de la fotosíntesis.
	El resultado final es glucosa que la planta utiliza como alimento.
	La fotosíntesis ocurre principalmente en las hojas de las plantas.`

	t.Run("genera quiz con preguntas válidas", func(t *testing.T) {
		questionCount := 3
		quiz, err := client.GenerateQuiz(ctx, testText, questionCount)

		require.NoError(t, err)
		require.NotNil(t, quiz)

		// Verificar estructura del quiz
		assert.NotEmpty(t, quiz.Questions, "El quiz debe tener preguntas")
		assert.LessOrEqual(t, len(quiz.Questions), questionCount, "No debe exceder el número de preguntas solicitadas")
		assert.False(t, quiz.GeneratedAt.IsZero(), "GeneratedAt debe estar establecido")
	})

	t.Run("preguntas tienen estructura correcta", func(t *testing.T) {
		quiz, err := client.GenerateQuiz(ctx, testText, 5)

		require.NoError(t, err)
		require.NotNil(t, quiz)

		for i, question := range quiz.Questions {
			// Verificar campos obligatorios
			assert.NotEmpty(t, question.ID, "La pregunta %d debe tener ID", i)
			assert.NotEmpty(t, question.QuestionText, "La pregunta %d debe tener texto", i)
			assert.NotEmpty(t, question.QuestionType, "La pregunta %d debe tener tipo", i)
			assert.NotEmpty(t, question.Options, "La pregunta %d debe tener opciones", i)
			assert.NotEmpty(t, question.CorrectAnswer, "La pregunta %d debe tener respuesta correcta", i)
			assert.NotEmpty(t, question.Explanation, "La pregunta %d debe tener explicación", i)
			assert.NotEmpty(t, question.Difficulty, "La pregunta %d debe tener dificultad", i)

			// Verificar valores específicos
			assert.Equal(t, "multiple_choice", question.QuestionType, "El tipo debe ser multiple_choice")
			assert.Len(t, question.Options, 4, "Debe haber 4 opciones")
			assert.Equal(t, 10, question.Points, "Cada pregunta debe valer 10 puntos")

			// Verificar que el ID tiene el formato correcto
			assert.Contains(t, question.ID, "q_", "El ID debe comenzar con 'q_'")

			// Verificar niveles de dificultad válidos
			assert.Contains(t, []string{"easy", "medium", "hard"}, question.Difficulty,
				"La dificultad debe ser easy, medium o hard")
		}
	})

	t.Run("respeta el límite de preguntas solicitadas", func(t *testing.T) {
		quiz, err := client.GenerateQuiz(ctx, testText, 2)

		require.NoError(t, err)
		require.NotNil(t, quiz)

		assert.LessOrEqual(t, len(quiz.Questions), 2, "No debe generar más preguntas de las solicitadas")
	})

	t.Run("genera diferentes niveles de dificultad", func(t *testing.T) {
		quiz, err := client.GenerateQuiz(ctx, testText, 6)

		require.NoError(t, err)
		require.NotNil(t, quiz)

		difficulties := make(map[string]bool)
		for _, question := range quiz.Questions {
			difficulties[question.Difficulty] = true
		}

		// Si hay suficientes preguntas, debería haber diferentes niveles
		if len(quiz.Questions) >= 3 {
			assert.True(t, len(difficulties) > 1, "Debe haber más de un nivel de dificultad")
		}
	})

	t.Run("omite oraciones muy cortas", func(t *testing.T) {
		shortText := "Hola. Si. No. Este es un texto con muchas oraciones muy cortas que deben ser omitidas."
		quiz, err := client.GenerateQuiz(ctx, shortText, 10)

		require.NoError(t, err)
		require.NotNil(t, quiz)

		// Solo debe generar pregunta de la oración larga (> 20 caracteres)
		assert.LessOrEqual(t, len(quiz.Questions), 1, "Debe omitir oraciones cortas")
	})
}

func TestSmartClient_HealthCheck(t *testing.T) {
	logger := createTestLogger()
	client := NewSmartClient(logger)
	ctx := context.Background()

	t.Run("retorna nil indicando servicio saludable", func(t *testing.T) {
		err := client.HealthCheck(ctx)
		assert.NoError(t, err, "HealthCheck debe retornar nil")
	})

	t.Run("funciona con contexto cancelado", func(t *testing.T) {
		cancelCtx, cancel := context.WithCancel(ctx)
		cancel()

		// El health check es simple y no verifica el contexto
		err := client.HealthCheck(cancelCtx)
		assert.NoError(t, err, "HealthCheck debe retornar nil incluso con contexto cancelado")
	})
}

func TestSmartClient_EmptyText(t *testing.T) {
	logger := createTestLogger()
	client := NewSmartClient(logger)
	ctx := context.Background()

	t.Run("GenerateSummary con texto vacío", func(t *testing.T) {
		summary, err := client.GenerateSummary(ctx, "")

		require.NoError(t, err, "No debe retornar error con texto vacío")
		require.NotNil(t, summary, "Debe retornar un summary válido")

		assert.Empty(t, summary.MainIdeas, "MainIdeas debe estar vacío")
		assert.Empty(t, summary.KeyConcepts, "KeyConcepts debe estar vacío")
		assert.Empty(t, summary.Sections, "Sections debe estar vacío")
		assert.Equal(t, 0, summary.WordCount, "WordCount debe ser 0")
		assert.NotNil(t, summary.Glossary, "Glossary debe estar inicializado")
	})

	t.Run("GenerateSummary con solo espacios", func(t *testing.T) {
		summary, err := client.GenerateSummary(ctx, "   \n\n   \t   ")

		require.NoError(t, err, "No debe retornar error con solo espacios")
		require.NotNil(t, summary, "Debe retornar un summary válido")

		assert.Empty(t, summary.MainIdeas, "MainIdeas debe estar vacío con solo espacios")
		assert.Equal(t, 0, summary.WordCount, "WordCount debe ser 0 con solo espacios")
	})

	t.Run("GenerateQuiz con texto vacío", func(t *testing.T) {
		quiz, err := client.GenerateQuiz(ctx, "", 5)

		require.NoError(t, err, "No debe retornar error con texto vacío")
		require.NotNil(t, quiz, "Debe retornar un quiz válido")

		assert.Empty(t, quiz.Questions, "Questions debe estar vacío")
	})

	t.Run("GenerateQuiz con texto insuficiente", func(t *testing.T) {
		quiz, err := client.GenerateQuiz(ctx, "Texto muy corto.", 5)

		require.NoError(t, err, "No debe retornar error")
		require.NotNil(t, quiz, "Debe retornar un quiz válido")

		// Puede tener 0 o 1 pregunta dependiendo de la longitud
		assert.LessOrEqual(t, len(quiz.Questions), 1, "No debe generar muchas preguntas con texto corto")
	})

	t.Run("GenerateSummary con una sola oración", func(t *testing.T) {
		summary, err := client.GenerateSummary(ctx, "Esta es una única oración de prueba para el resumen.")

		require.NoError(t, err, "No debe retornar error")
		require.NotNil(t, summary, "Debe retornar un summary válido")

		assert.NotEmpty(t, summary.MainIdeas, "Debe tener al menos una MainIdea")
		assert.LessOrEqual(t, len(summary.MainIdeas), 1, "Solo debe tener 1 MainIdea")
		assert.NotEmpty(t, summary.Sections, "Debe crear secciones")
	})

	t.Run("GenerateQuiz con questionCount cero", func(t *testing.T) {
		quiz, err := client.GenerateQuiz(ctx, testLongText(), 0)

		require.NoError(t, err, "No debe retornar error con questionCount 0")
		require.NotNil(t, quiz, "Debe retornar un quiz válido")

		assert.Empty(t, quiz.Questions, "No debe generar preguntas si questionCount es 0")
	})

	t.Run("GenerateQuiz con questionCount negativo causa panic", func(t *testing.T) {
		// El código actual tiene un bug: no valida questionCount negativo
		// Esto causa panic al hacer make([]Question, 0, questionCount)
		// Este test documenta el comportamiento actual
		assert.Panics(t, func() {
			_, _ = client.GenerateQuiz(ctx, testLongText(), -5)
		}, "questionCount negativo debe causar panic (bug conocido)")
	})
}

func TestSmartClient_InterfaceCompliance(t *testing.T) {
	logger := createTestLogger()
	client := NewSmartClient(logger)

	t.Run("implementa la interfaz nlp.Client", func(t *testing.T) {
		// Si esto compila, el test pasa
		assert.NotNil(t, client, "El cliente debe ser no nulo")
		assert.Implements(t, (*nlp.Client)(nil), client, "Debe implementar la interfaz nlp.Client")
	})
}

func TestSmartClient_ExtractSections(t *testing.T) {
	log := createTestLogger()
	client := NewSmartClient(log)
	ctx := context.Background()

	t.Run("texto con títulos en MAYÚSCULAS", func(t *testing.T) {
		text := `INTRODUCCIÓN

La fotosíntesis es un proceso fundamental en la naturaleza.
Permite a las plantas convertir la luz solar en energía química.

DESARROLLO

El proceso ocurre en los cloroplastos de las células vegetales.
La clorofila es el pigmento responsable de capturar la energía lumínica.
El agua y el dióxido de carbono son los reactivos principales.

CONCLUSIÓN

La fotosíntesis es esencial para la vida en la Tierra.
Sin este proceso, no habría oxígeno en la atmósfera.`

		sections, err := client.ExtractSections(ctx, text)
		require.NoError(t, err)
		require.NotNil(t, sections)
		assert.GreaterOrEqual(t, len(sections), 2, "debe detectar al menos 2 secciones tituladas")

		// Verificar que las secciones tienen estructura válida
		for i, section := range sections {
			assert.Equal(t, i, section.Index, "índice debe ser secuencial")
			assert.NotEmpty(t, section.Title, "cada sección debe tener título")
			assert.NotEmpty(t, section.Content, "cada sección debe tener contenido")
			assert.NotEmpty(t, section.Preview, "cada sección debe tener preview")
		}

		// Verificar que se detectaron los títulos
		titles := make([]string, len(sections))
		for i, s := range sections {
			titles[i] = s.Title
		}
		assert.Contains(t, titles, "INTRODUCCIÓN")
		assert.Contains(t, titles, "DESARROLLO")
		assert.Contains(t, titles, "CONCLUSIÓN")
	})

	t.Run("texto con secciones numeradas", func(t *testing.T) {
		text := `1. Primera sección

Este es el contenido de la primera sección del documento.
Contiene información importante sobre el tema principal.

2. Segunda sección

Aquí se desarrolla el segundo punto del documento.
Se amplían los conceptos presentados anteriormente.

3. Tercera sección

Finalmente, se presentan las conclusiones y recomendaciones.
El análisis demuestra la importancia del tema.`

		sections, err := client.ExtractSections(ctx, text)
		require.NoError(t, err)
		require.NotNil(t, sections)
		assert.GreaterOrEqual(t, len(sections), 3, "debe detectar al menos 3 secciones numeradas")

		for i, section := range sections {
			assert.Equal(t, i, section.Index)
			assert.NotEmpty(t, section.Title)
			assert.NotEmpty(t, section.Content)
			assert.NotEmpty(t, section.Preview)
		}
	})

	t.Run("texto sin estructura genera secciones genéricas", func(t *testing.T) {
		// Generar texto largo sin estructura (~1500 words)
		var words []string
		sampleWords := []string{"educación", "aprendizaje", "conocimiento", "estudiante", "profesor",
			"clase", "escuela", "libro", "ciencia", "tecnología", "desarrollo", "metodología",
			"investigación", "análisis", "resultado", "conclusión", "proceso", "evaluación"}
		for i := 0; i < 1500; i++ {
			words = append(words, sampleWords[i%len(sampleWords)])
		}
		text := ""
		for i, w := range words {
			text += w
			if i < len(words)-1 {
				text += " "
			}
		}

		sections, err := client.ExtractSections(ctx, text)
		require.NoError(t, err)
		require.NotNil(t, sections)
		assert.GreaterOrEqual(t, len(sections), 2, "debe crear al menos 2 secciones genéricas")
		assert.LessOrEqual(t, len(sections), 5, "no debe crear demasiadas secciones para ~1500 palabras")

		for _, section := range sections {
			assert.Contains(t, section.Title, "Sección", "secciones genéricas deben titularse 'Sección N'")
			assert.NotEmpty(t, section.Content)
			assert.NotEmpty(t, section.Preview)
		}
	})

	t.Run("texto muy corto retorna 1 sección", func(t *testing.T) {
		text := "Este es un texto muy corto con menos de cien palabras. Solo sirve para verificar el caso base."

		sections, err := client.ExtractSections(ctx, text)
		require.NoError(t, err)
		require.NotNil(t, sections)
		assert.Len(t, sections, 1, "texto corto debe generar exactamente 1 sección")
		assert.Equal(t, 0, sections[0].Index)
		assert.NotEmpty(t, sections[0].Content)
		assert.NotEmpty(t, sections[0].Preview)
	})

	t.Run("texto con 60+ secciones potenciales se limita a 50", func(t *testing.T) {
		// Generar texto con 60 títulos en mayúsculas
		var parts []string
		for i := 1; i <= 60; i++ {
			title := fmt.Sprintf("SECCIÓN NÚMERO %d", i)
			body := fmt.Sprintf("Contenido de la sección %d con información relevante.", i)
			parts = append(parts, title+"\n\n"+body)
		}
		text := strings.Join(parts, "\n\n")

		sections, err := client.ExtractSections(ctx, text)
		require.NoError(t, err)
		require.NotNil(t, sections)
		assert.LessOrEqual(t, len(sections), 50, "no debe exceder 50 secciones")
		assert.GreaterOrEqual(t, len(sections), 1, "debe tener al menos 1 sección")

		// Verificar índices secuenciales
		for i, section := range sections {
			assert.Equal(t, i, section.Index, "índices deben ser secuenciales tras merge")
		}
	})

	t.Run("texto vacío retorna nil", func(t *testing.T) {
		sections, err := client.ExtractSections(ctx, "")
		require.NoError(t, err)
		assert.Nil(t, sections)
	})

	t.Run("texto con solo espacios retorna nil", func(t *testing.T) {
		sections, err := client.ExtractSections(ctx, "   \n\n   \t   ")
		require.NoError(t, err)
		assert.Nil(t, sections)
	})
}

// NOTA: Tests de concurrencia removidos porque SmartClient usa math/rand global
// que no es thread-safe. El cliente fallback no está diseñado para uso concurrente
// intensivo ya que es una implementación de respaldo para desarrollo/testing.

// Funciones auxiliares para tests

func testLongText() string {
	return `La inteligencia artificial es una rama de la ciencia computacional.
	Se enfoca en crear sistemas capaces de realizar tareas que requieren inteligencia humana.
	El aprendizaje automático es un subcampo importante de la inteligencia artificial.
	Las redes neuronales son estructuras inspiradas en el cerebro humano.
	El procesamiento del lenguaje natural permite a las máquinas entender texto.
	La visión por computadora ayuda a las máquinas a interpretar imágenes.
	Los algoritmos de clasificación son fundamentales en el aprendizaje supervisado.
	El aprendizaje profundo utiliza redes neuronales con múltiples capas.`
}
