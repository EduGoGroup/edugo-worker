package fallback

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"math/rand"
	"strings"
	"time"

	"github.com/EduGoGroup/edugo-shared/logger"
	"github.com/EduGoGroup/edugo-worker/internal/infrastructure/nlp"
)

type SmartClient struct {
	logger logger.Logger
	rng    *rand.Rand
}

func NewSmartClient(log logger.Logger) nlp.Client {
	return &SmartClient{
		logger: log,
		rng:    rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

func (c *SmartClient) GenerateSummary(ctx context.Context, text string) (*nlp.Summary, error) {
	c.logger.Info("generando resumen con SmartFallback", "textLength", len(text))

	time.Sleep(time.Duration(500+c.rng.Intn(1000)) * time.Millisecond)

	sentences := splitSentences(text)
	words := strings.Fields(text)

	mainIdeas := extractMainIdeas(sentences, 3)
	keyConcepts := extractKeyConcepts(text)
	sections := createSections(sentences)

	c.logger.Info("resumen generado", "mainIdeas", len(mainIdeas), "concepts", len(keyConcepts))

	return &nlp.Summary{
		MainIdeas:   mainIdeas,
		KeyConcepts: keyConcepts,
		Sections:    sections,
		Glossary:    make(map[string]string),
		WordCount:   len(words),
		GeneratedAt: time.Now(),
	}, nil
}

func (c *SmartClient) GenerateQuiz(ctx context.Context, text string, questionCount int) (*nlp.Quiz, error) {
	c.logger.Info("generando quiz con SmartFallback", "questionCount", questionCount)

	time.Sleep(time.Duration(800+c.rng.Intn(1200)) * time.Millisecond)

	sentences := splitSentences(text)
	questions := make([]nlp.Question, 0, questionCount)

	for i := 0; i < questionCount && i < len(sentences); i++ {
		sentence := strings.TrimSpace(sentences[i])
		if len(sentence) < 20 {
			continue
		}

		hash := sha256.Sum256([]byte(sentence))
		qID := "q_" + hex.EncodeToString(hash[:4])

		questions = append(questions, nlp.Question{
			ID:            qID,
			QuestionText:  "¿Cuál es la idea principal de: \"" + truncate(sentence, 50) + "...\"?",
			QuestionType:  "multiple_choice",
			Options:       []string{"Opción A", "Opción B", "Opción C", "Opción D"},
			CorrectAnswer: "Opción A",
			Explanation:   "La respuesta correcta se basa en el contenido del texto.",
			Difficulty:    getDifficulty(i),
			Points:        10,
		})
	}

	c.logger.Info("quiz generado", "questions", len(questions))

	return &nlp.Quiz{
		Questions:   questions,
		GeneratedAt: time.Now(),
	}, nil
}

func (c *SmartClient) HealthCheck(ctx context.Context) error {
	c.logger.Debug("health check SmartFallback: OK")
	return nil
}

func splitSentences(text string) []string {
	text = strings.ReplaceAll(text, "\n", " ")
	parts := strings.Split(text, ".")
	var sentences []string
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if len(trimmed) > 10 {
			sentences = append(sentences, trimmed)
		}
	}
	return sentences
}

func extractMainIdeas(sentences []string, count int) []string {
	ideas := make([]string, 0, count)
	for i := 0; i < count && i < len(sentences); i++ {
		idea := strings.TrimSpace(sentences[i])
		if len(idea) > 200 {
			idea = idea[:200] + "..."
		}
		ideas = append(ideas, idea)
	}
	return ideas
}

func extractKeyConcepts(text string) map[string]string {
	concepts := make(map[string]string)
	words := strings.Fields(text)

	wordCount := make(map[string]int)
	for _, word := range words {
		word = strings.ToLower(strings.Trim(word, ".,;:!?\"'()[]{}"))
		if len(word) > 5 {
			wordCount[word]++
		}
	}

	count := 0
	for word, freq := range wordCount {
		if freq > 2 && count < 5 {
			concepts[word] = "Concepto frecuente en el texto"
			count++
		}
	}

	return concepts
}

func createSections(sentences []string) []nlp.Section {
	if len(sentences) == 0 {
		return nil
	}

	sectionSize := len(sentences) / 3
	if sectionSize == 0 {
		sectionSize = 1
	}

	sections := []nlp.Section{
		{Title: "Introducción", Content: joinSentences(sentences, 0, sectionSize)},
		{Title: "Desarrollo", Content: joinSentences(sentences, sectionSize, sectionSize*2)},
		{Title: "Conclusión", Content: joinSentences(sentences, sectionSize*2, len(sentences))},
	}

	return sections
}

func joinSentences(sentences []string, start, end int) string {
	if start >= len(sentences) {
		return ""
	}
	if end > len(sentences) {
		end = len(sentences)
	}
	return strings.Join(sentences[start:end], ". ") + "."
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen]
}

func getDifficulty(index int) string {
	switch index % 3 {
	case 0:
		return "easy"
	case 1:
		return "medium"
	default:
		return "hard"
	}
}

var _ nlp.Client = (*SmartClient)(nil)
