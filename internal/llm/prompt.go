package llm

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Los prompts viven aquí (no en cada implementación) para que ollama y api
// compartan exactamente el mismo texto: así el harness (D-039.8) mide el modelo,
// no diferencias de prompt. Todos piden SOLO JSON válido, en español.

// BuildGenerationPrompt arma el prompt de generación de evaluación. Pide
// explícitamente un objeto que cumpla el contrato assessment_import v1 y NADA más
// (sin markdown, sin explicación fuera del JSON).
func BuildGenerationPrompt(material MaterialInput, params GenerationParams) string {
	lang := params.Language
	if lang == "" {
		lang = "es"
	}
	n := params.NumQuestions
	if n <= 0 {
		n = 5
	}

	var b strings.Builder
	b.WriteString("Eres un generador de evaluaciones educativas. A partir del MATERIAL de estudio, ")
	fmt.Fprintf(&b, "genera una evaluación con aproximadamente %d preguntas en idioma %q.\n\n", n, lang)

	b.WriteString("REGLAS DE SALIDA (obligatorias):\n")
	b.WriteString("- Responde EXCLUSIVAMENTE con un objeto JSON válido. Sin texto antes ni después, sin ```.\n")
	b.WriteString("- El JSON DEBE tener esta forma exacta:\n")
	b.WriteString(generationSchemaHint)
	b.WriteString("\n- \"question_type\" ∈ {\"multiple_choice\",\"multiple_select\",\"true_false\",\"short_answer\",\"open_ended\"}.\n")
	b.WriteString("- \"options\" con ≥2 elementos para multiple_choice y multiple_select; ARREGLO VACÍO para true_false, short_answer y open_ended.\n")
	b.WriteString("- Las opciones NO llevan marca de correcta. La correcta se indica en \"correct_answer\" por TEXTO:\n")
	b.WriteString("  · multiple_choice: string con el texto EXACTO de una opción.\n")
	b.WriteString("  · multiple_select: arreglo de strings con los textos EXACTOS de las opciones correctas.\n")
	b.WriteString("  · true_false: string \"true\" o \"false\".\n")
	b.WriteString("  · short_answer: string con la respuesta esperada.\n")
	b.WriteString("  · open_ended: OMITE \"correct_answer\".\n")
	b.WriteString("- CRÍTICO: \"correct_answer\" debe COPIAR EXACTAMENTE el texto de una de las opciones (mismo texto, carácter por carácter). NUNCA uses letras (A, B, C), números ni índices para señalar la correcta.\n")
	b.WriteString(correctAnswerExample)
	b.WriteString("- \"difficulty\" ∈ {\"easy\",\"medium\",\"hard\"} o null. \"points\" ≥ 0. \"passing_score\" 0..100.\n")

	if params.Difficulty != "" {
		fmt.Fprintf(&b, "- Dificultad objetivo: %q.\n", params.Difficulty)
	}
	if len(params.QuestionTypes) > 0 {
		fmt.Fprintf(&b, "- Usa preferentemente estos tipos de pregunta: %s.\n", strings.Join(params.QuestionTypes, ", "))
	}

	b.WriteString("\nMATERIAL:\n")
	if material.Title != "" {
		b.WriteString("Título: " + material.Title + "\n")
	}
	if material.SubjectHint != "" {
		b.WriteString("Materia: " + material.SubjectHint + "\n")
	}
	b.WriteString("---\n")
	b.WriteString(material.Content)
	b.WriteString("\n---\n")
	return b.String()
}

// correctAnswerExample refuerza con un contraste correcto/incorrecto la regla de
// que correct_answer copia el TEXTO de la opción (no la letra). Endurecimiento
// para dar chance justa a modelos locales chicos (design 039 §6.1); la iteración
// fina de prompts es de los planes 040/041.
const correctAnswerExample = `  Ejemplo CORRECTO:
    {"question_type":"multiple_choice",
     "options":[{"option_text":"Clorofila","sort_order":1},{"option_text":"Hemoglobina","sort_order":2}],
     "correct_answer":"Clorofila"}
  Ejemplo INCORRECTO (NO hagas esto):
    {"correct_answer":"A"}   // ❌ es una letra, no el texto de la opción
`

// generationSchemaHint es el esqueleto del contrato que se incrusta en el prompt.
const generationSchemaHint = `{
  "format": "edugo.assessment_import",
  "version": 1,
  "assessment": {
    "title": "string",
    "description": "string",
    "passing_score": 60
  },
  "questions": [
    {
      "question_text": "string",
      "question_type": "multiple_choice",
      "options": [ {"option_text": "string", "sort_order": 1} ],
      "correct_answer": "string",
      "explanation": "string",
      "points": 1,
      "difficulty": "easy",
      "tags": ["string"]
    }
  ]
}`

// ExtractJSON aísla el primer objeto JSON de una respuesta de LLM, tolerando
// vallas de markdown (```json ... ```) y texto conversacional alrededor. Devuelve
// error si no encuentra un objeto balanceado.
func ExtractJSON(raw string) (json.RawMessage, error) {
	s := strings.TrimSpace(raw)

	// Quitar vallas ```json ... ``` o ``` ... ```.
	if strings.HasPrefix(s, "```") {
		s = strings.TrimPrefix(s, "```")
		s = strings.TrimPrefix(s, "json")
		s = strings.TrimPrefix(s, "JSON")
		if i := strings.LastIndex(s, "```"); i >= 0 {
			s = s[:i]
		}
		s = strings.TrimSpace(s)
	}

	start := strings.Index(s, "{")
	if start < 0 {
		return nil, fmt.Errorf("no se encontró objeto JSON en la respuesta del modelo")
	}

	// Buscar la llave de cierre balanceada, respetando strings y escapes.
	depth := 0
	inString := false
	escaped := false
	for i := start; i < len(s); i++ {
		c := s[i]
		switch {
		case escaped:
			escaped = false
		case c == '\\' && inString:
			escaped = true
		case c == '"':
			inString = !inString
		case c == '{' && !inString:
			depth++
		case c == '}' && !inString:
			depth--
			if depth == 0 {
				return unwrapEnvelope(json.RawMessage(s[start : i+1])), nil
			}
		}
	}
	return nil, fmt.Errorf("objeto JSON sin cierre balanceado en la respuesta del modelo")
}

// envelopeKeys son claves envolventes espurias que algunos modelos locales chicos
// (qwen3:1.7b con format:"json") anteponen de forma intermitente, dejando el
// objeto útil un nivel adentro (p.ej. {"bytes":{"verdict":...}}). Medido en 040 T2c.
var envelopeKeys = map[string]bool{
	"bytes": true, "result": true, "data": true, "response": true, "output": true, "json": true,
}

// unwrapEnvelope desenvuelve un objeto anidado UNA sola vez cuando el objeto de
// nivel superior tiene EXACTAMENTE una clave, esa clave es una envoltura espuria
// conocida y su valor es a su vez un objeto. Es conservador a propósito: los
// contratos legítimos (assessment_import con varias claves de tope; review con
// verdict/score/feedback) nunca cumplen las tres condiciones, así que no se tocan.
func unwrapEnvelope(raw json.RawMessage) json.RawMessage {
	var top map[string]json.RawMessage
	if err := json.Unmarshal(raw, &top); err != nil {
		return raw
	}
	if len(top) != 1 {
		return raw
	}
	for k, v := range top {
		if !envelopeKeys[k] {
			return raw
		}
		if inner := strings.TrimSpace(string(v)); strings.HasPrefix(inner, "{") {
			return json.RawMessage(inner)
		}
	}
	return raw
}
