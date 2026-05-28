package fallback

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
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

func (c *SmartClient) ExtractSections(ctx context.Context, text string) ([]nlp.DocumentSection, error) {
	c.logger.Info("extrayendo secciones con SmartFallback", "textLength", len(text))

	time.Sleep(time.Duration(300+c.rng.Intn(700)) * time.Millisecond)

	text = strings.TrimSpace(text)
	if len(text) == 0 {
		return nil, nil
	}

	sections := detectSections(text)

	// Enforce limits: min 1, max 50
	if len(sections) > 50 {
		sections = mergeToLimit(sections, 50)
	}

	c.logger.Info("secciones extraídas", "count", len(sections))
	return sections, nil
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

// titleHit represents a detected title and its paragraph index.
type titleHit struct {
	paraIdx int
	title   string
}

// detectSections intenta detectar secciones en el texto usando heurísticas:
// 1. Títulos en MAYÚSCULAS
// 2. Títulos numerados (1., 2., I., II., Chapter, Capítulo, etc.)
// 3. Líneas cortas seguidas de párrafos largos
// Si no se detectan títulos, divide el texto en secciones genéricas.
func detectSections(text string) []nlp.DocumentSection {
	paragraphs := splitParagraphs(text)
	if len(paragraphs) == 0 {
		return []nlp.DocumentSection{{
			Index:   0,
			Title:   "Sección 1",
			Content: text,
			Preview: generatePreview(text),
		}}
	}

	// Try to detect titled sections
	var hits []titleHit

	for i, p := range paragraphs {
		firstLine := firstLineOf(p)
		if isTitleLine(firstLine) {
			hits = append(hits, titleHit{paraIdx: i, title: strings.TrimSpace(firstLine)})
		}
	}

	if len(hits) >= 2 {
		return buildSectionsFromTitles(paragraphs, hits)
	}

	// No titles detected: divide into ~500-800 word chunks
	return buildGenericSections(text)
}

// splitParagraphs splits text by double newlines into paragraphs.
func splitParagraphs(text string) []string {
	// Normalize line endings
	text = strings.ReplaceAll(text, "\r\n", "\n")
	raw := strings.Split(text, "\n\n")
	var paragraphs []string
	for _, p := range raw {
		trimmed := strings.TrimSpace(p)
		if len(trimmed) > 0 {
			paragraphs = append(paragraphs, trimmed)
		}
	}
	return paragraphs
}

// firstLineOf returns the first non-empty line of a paragraph.
func firstLineOf(paragraph string) string {
	lines := strings.SplitN(paragraph, "\n", 2)
	if len(lines) == 0 {
		return ""
	}
	return strings.TrimSpace(lines[0])
}

// isTitleLine checks if a line looks like a section title.
func isTitleLine(line string) bool {
	if len(line) == 0 || len(line) >= 100 {
		return false
	}

	// ALL UPPERCASE and has at least 3 word characters
	if isAllUpper(line) && len(strings.Fields(line)) >= 1 && countLetters(line) >= 3 {
		return true
	}

	// Numbered patterns
	numberedPrefixes := []string{
		"1.", "2.", "3.", "4.", "5.", "6.", "7.", "8.", "9.",
		"I.", "II.", "III.", "IV.", "V.", "VI.", "VII.", "VIII.", "IX.", "X.",
	}
	for _, prefix := range numberedPrefixes {
		if strings.HasPrefix(line, prefix) {
			return true
		}
	}

	// Keyword prefixes (case-insensitive)
	lower := strings.ToLower(line)
	keywordPrefixes := []string{
		"chapter ", "capítulo ", "capitulo ",
		"sección ", "seccion ",
		"tema ", "unidad ",
	}
	for _, kw := range keywordPrefixes {
		if strings.HasPrefix(lower, kw) {
			return true
		}
	}

	// Short line (< 80 chars) that doesn't end with a period — heuristic for heading
	if len(line) < 80 && !strings.HasSuffix(strings.TrimSpace(line), ".") {
		// Only count as title if it's relatively short compared to a sentence
		words := strings.Fields(line)
		if len(words) >= 1 && len(words) <= 10 {
			return true
		}
	}

	return false
}

// isAllUpper checks if all letter characters in the string are uppercase.
func isAllUpper(s string) bool {
	hasLetter := false
	for _, r := range s {
		if r >= 'a' && r <= 'z' {
			return false
		}
		if r >= 'A' && r <= 'Z' {
			hasLetter = true
		}
		// Accented lowercase
		if r >= 'à' && r <= 'ÿ' && r != '×' && r != '÷' {
			return false
		}
	}
	return hasLetter
}

// countLetters counts letter characters in a string.
func countLetters(s string) int {
	count := 0
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= 'À' && r <= 'ÿ') {
			count++
		}
	}
	return count
}

// buildSectionsFromTitles groups paragraphs between detected titles into sections.
func buildSectionsFromTitles(paragraphs []string, hits []titleHit) []nlp.DocumentSection {
	var sections []nlp.DocumentSection

	// Content before the first title goes into a preamble section if non-empty
	if hits[0].paraIdx > 0 {
		preambleContent := joinParagraphs(paragraphs[:hits[0].paraIdx])
		if len(strings.TrimSpace(preambleContent)) > 0 {
			sections = append(sections, nlp.DocumentSection{
				Index:   0,
				Title:   "Introducción",
				Content: preambleContent,
				Preview: generatePreview(preambleContent),
			})
		}
	}

	for i, hit := range hits {
		start := hit.paraIdx
		var end int
		if i+1 < len(hits) {
			end = hits[i+1].paraIdx
		} else {
			end = len(paragraphs)
		}

		// The content is the paragraph(s) between this title and the next
		// Remove the title line from the first paragraph if it's only the title
		firstPara := paragraphs[start]
		lines := strings.SplitN(firstPara, "\n", 2)
		var contentParts []string
		if len(lines) > 1 {
			rest := strings.TrimSpace(lines[1])
			if len(rest) > 0 {
				contentParts = append(contentParts, rest)
			}
		}
		for j := start + 1; j < end; j++ {
			contentParts = append(contentParts, paragraphs[j])
		}

		content := strings.Join(contentParts, "\n\n")
		if len(strings.TrimSpace(content)) == 0 {
			content = hit.title // Use title as content if section body is empty
		}

		sections = append(sections, nlp.DocumentSection{
			Index:   len(sections),
			Title:   hit.title,
			Content: content,
			Preview: generatePreview(content),
		})
	}

	return sections
}

// buildGenericSections divides text evenly into sections of ~500-800 words.
func buildGenericSections(text string) []nlp.DocumentSection {
	words := strings.Fields(text)
	totalWords := len(words)

	if totalWords == 0 {
		return nil
	}

	// Target ~600 words per section (midpoint of 500-800)
	targetWordsPerSection := 600
	numSections := totalWords / targetWordsPerSection
	if numSections < 1 {
		numSections = 1
	}
	if numSections > 50 {
		numSections = 50
	}

	wordsPerSection := totalWords / numSections
	sections := make([]nlp.DocumentSection, 0, numSections)

	for i := 0; i < numSections; i++ {
		start := i * wordsPerSection
		end := start + wordsPerSection
		if i == numSections-1 {
			end = totalWords // Last section gets remaining words
		}
		if start >= totalWords {
			break
		}

		content := strings.Join(words[start:end], " ")
		sections = append(sections, nlp.DocumentSection{
			Index:   i,
			Title:   fmt.Sprintf("Sección %d", i+1),
			Content: content,
			Preview: generatePreview(content),
		})
	}

	return sections
}

// generatePreview returns the first 200 words or first 3 sentences, whichever is shorter.
func generatePreview(text string) string {
	if len(text) == 0 {
		return ""
	}

	// Strategy 1: first 3 sentences
	sentences := splitPreviewSentences(text)
	var threeS string
	if len(sentences) <= 3 {
		threeS = strings.Join(sentences, ". ")
		if !strings.HasSuffix(threeS, ".") {
			threeS += "."
		}
	} else {
		threeS = strings.Join(sentences[:3], ". ") + "."
	}

	// Strategy 2: first 200 words
	words := strings.Fields(text)
	var twoHundred string
	if len(words) <= 200 {
		twoHundred = strings.Join(words, " ")
	} else {
		twoHundred = strings.Join(words[:200], " ") + "..."
	}

	// Return whichever is shorter
	if len(threeS) <= len(twoHundred) {
		return threeS
	}
	return twoHundred
}

// splitPreviewSentences splits text into sentences for preview generation.
func splitPreviewSentences(text string) []string {
	// Simple sentence splitting by period followed by space or end
	text = strings.ReplaceAll(text, "\n", " ")
	parts := strings.Split(text, ".")
	var sentences []string
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if len(trimmed) > 5 {
			sentences = append(sentences, trimmed)
		}
	}
	return sentences
}

// joinParagraphs joins paragraphs with double newlines.
func joinParagraphs(paragraphs []string) string {
	return strings.Join(paragraphs, "\n\n")
}

// mergeToLimit reduces sections to maxCount by merging the shortest adjacent pairs.
func mergeToLimit(sections []nlp.DocumentSection, maxCount int) []nlp.DocumentSection {
	for len(sections) > maxCount {
		// Find the pair of adjacent sections with the smallest combined content
		minIdx := 0
		minLen := len(sections[0].Content) + len(sections[1].Content)
		for i := 1; i < len(sections)-1; i++ {
			combined := len(sections[i].Content) + len(sections[i+1].Content)
			if combined < minLen {
				minLen = combined
				minIdx = i
			}
		}

		// Merge sections[minIdx] and sections[minIdx+1]
		merged := nlp.DocumentSection{
			Index:   sections[minIdx].Index,
			Title:   sections[minIdx].Title,
			Content: sections[minIdx].Content + "\n\n" + sections[minIdx+1].Content,
		}
		merged.Preview = generatePreview(merged.Content)

		// Replace the pair with the merged section
		newSections := make([]nlp.DocumentSection, 0, len(sections)-1)
		newSections = append(newSections, sections[:minIdx]...)
		newSections = append(newSections, merged)
		newSections = append(newSections, sections[minIdx+2:]...)

		// Re-index
		for i := range newSections {
			newSections[i].Index = i
		}
		sections = newSections
	}
	return sections
}

var _ nlp.Client = (*SmartClient)(nil)
