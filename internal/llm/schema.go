package llm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// La salida de las llamadas del carril de revisión se cierra en dos sitios (HU-INF.5,
// P9 de R-047): el esquema de cada llamada viaja al modelo local, que restringe la
// forma al generar, y el mismo esquema se revalida en Go siempre, que es lo que
// manda. El proveedor de pago no manda el esquema: la versión de API del worker
// (2023-06-01) no consta en su documentación como compatible con la salida
// estructurada, así que ahí solo vale la revalidación.
//
// La validación vive en quien llama (el par, el criterio y el carril global), no en el
// proveedor: el juez del dedupe del PDF usa la misma llamada de par y para él una
// salida inválida sigue siendo «no duplicado» (decisión 7).

// ErrInvalidOutput marca una salida del modelo que no cumple el esquema de su llamada:
// un campo que falta, un tipo equivocado o un veredicto fuera de la lista. Envuelve
// ErrLLMQuality: es un fallo de calidad, no de transporte.
var ErrInvalidOutput = fmt.Errorf("%w: salida fuera de esquema", ErrLLMQuality)

// InvalidOutputRetryTemperature es la temperatura del único reintento de una salida
// inválida. A temperatura 0, repetir la misma llamada devuelve lo mismo; el pipeline
// de material usa el mismo valor en su reintento por calidad. La primera llamada
// sigue con la temperatura del proveedor (0 en producción, R-048).
const InvalidOutputRetryTemperature = 0.3

// schemaNode es el subconjunto de JSON Schema que usan las llamadas del worker: objeto
// con propiedades obligatorias, cadena (con o sin lista cerrada), número y lista de
// cadenas. Se serializa tal cual para el modelo local y se valida en Go.
type schemaNode struct {
	Type       string                 `json:"type"`
	Properties map[string]*schemaNode `json:"properties,omitempty"`
	Required   []string               `json:"required,omitempty"`
	Enum       []string               `json:"enum,omitempty"`
	Items      *schemaNode            `json:"items,omitempty"`
}

// OutputSchema es el esquema de la salida de una llamada.
type OutputSchema struct {
	root schemaNode
}

// JSON devuelve el esquema como JSON Schema, listo para el campo format de Ollama.
func (s OutputSchema) JSON() json.RawMessage {
	b, err := json.Marshal(s.root)
	if err != nil {
		// Un schemaNode siempre serializa; si no, es un error de programación.
		panic(fmt.Sprintf("serializando el esquema de salida: %v", err))
	}
	return b
}

// Validate comprueba la salida cruda contra el esquema. Las claves de más no se
// rechazan (el prompt ya las prohíbe y no cambian el resultado); el rango del score
// tampoco: la nota de los carriles globales ya no sale de él. Las cadenas de una lista
// cerrada se comparan sin distinguir mayúsculas.
func (s OutputSchema) Validate(raw json.RawMessage) error {
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return fmt.Errorf("%w: no es JSON: %v", ErrInvalidOutput, err)
	}
	return validateNode(&s.root, v, "salida")
}

func validateNode(n *schemaNode, v any, path string) error {
	switch n.Type {
	case "object":
		obj, ok := v.(map[string]any)
		if !ok {
			return fmt.Errorf("%w: %s no es un objeto", ErrInvalidOutput, path)
		}
		for _, key := range n.Required {
			if _, ok := obj[key]; !ok {
				return fmt.Errorf("%w: falta %q", ErrInvalidOutput, key)
			}
		}
		for key, child := range n.Properties {
			if val, ok := obj[key]; ok {
				if err := validateNode(child, val, key); err != nil {
					return err
				}
			}
		}
	case "string":
		str, ok := v.(string)
		if !ok {
			return fmt.Errorf("%w: %q no es una cadena", ErrInvalidOutput, path)
		}
		if len(n.Enum) > 0 && !containsFold(n.Enum, str) {
			return fmt.Errorf("%w: %q = %q no está en %v", ErrInvalidOutput, path, str, n.Enum)
		}
	case "number":
		if _, ok := v.(float64); !ok {
			return fmt.Errorf("%w: %q no es un número", ErrInvalidOutput, path)
		}
	case "array":
		list, ok := v.([]any)
		if !ok {
			return fmt.Errorf("%w: %q no es una lista", ErrInvalidOutput, path)
		}
		for i, item := range list {
			if err := validateNode(n.Items, item, fmt.Sprintf("%s[%d]", path, i)); err != nil {
				return err
			}
		}
	}
	return nil
}

func containsFold(list []string, s string) bool {
	s = strings.TrimSpace(s)
	for _, item := range list {
		if strings.EqualFold(item, s) {
			return true
		}
	}
	return false
}

// BinaryVerdicts son los veredictos de las llamadas binarias (el par y el criterio) y
// del juicio global de la corta.
var BinaryVerdicts = []Verdict{VerdictCorrect, VerdictIncorrect}

// ReviewVerdicts son los veredictos válidos del juicio global según el tipo de
// pregunta: la corta es binaria; la larga (o el tipo vacío, por compatibilidad F3)
// admite el parcial.
func ReviewVerdicts(questionType string) []Verdict {
	if questionType == QuestionTypeShortAnswer {
		return BinaryVerdicts
	}
	return []Verdict{VerdictCorrect, VerdictPartial, VerdictIncorrect}
}

// VerdictSchema es el esquema {verdict, score, feedback} con la lista de veredictos
// dada.
func VerdictSchema(allowed []Verdict) OutputSchema {
	enum := make([]string, len(allowed))
	for i, v := range allowed {
		enum[i] = string(v)
	}
	return OutputSchema{root: schemaNode{
		Type: "object",
		Properties: map[string]*schemaNode{
			"verdict":  {Type: "string", Enum: enum},
			"score":    {Type: "number"},
			"feedback": {Type: "string"},
		},
		Required: []string{"verdict", "score", "feedback"},
	}}
}

// IdeasSchema es el esquema de la extracción de ideas: {"ideas": ["…"]}.
var IdeasSchema = OutputSchema{root: schemaNode{
	Type:       "object",
	Properties: map[string]*schemaNode{"ideas": {Type: "array", Items: &schemaNode{Type: "string"}}},
	Required:   []string{"ideas"},
}}

// DecodeVerdictOutput aísla el objeto JSON de la salida del modelo y lo decodifica en
// un ReviewResult con el crudo adjunto (Raw), para que quien llama lo revalide. No
// valida el veredicto: el proveedor sigue igual de laxo que antes (decisión 7). Una
// salida sin objeto JSON o que no decodifica es un error de calidad (ErrLLMQuality);
// what es el nombre de la llamada, para el mensaje.
func DecodeVerdictOutput(out, what string) (ReviewResult, error) {
	rawJSON, err := ExtractJSON(out)
	if err != nil {
		return ReviewResult{}, fmt.Errorf("%w: respuesta de %s no parseable: %v", ErrLLMQuality, what, err)
	}
	var result ReviewResult
	if err := json.Unmarshal(rawJSON, &result); err != nil {
		return ReviewResult{}, fmt.Errorf("%w: respuesta de %s no parseable: %v", ErrLLMQuality, what, err)
	}
	result.Raw = rawJSON
	return result, nil
}

// ValidateVerdictOutput revalida en Go el resultado de una llamada de veredicto: con
// el crudo (Raw), contra el esquema entero; sin él (un doble de prueba), solo el
// veredicto. Devuelve el resultado con el veredicto en minúsculas.
func ValidateVerdictOutput(res ReviewResult, allowed []Verdict) (ReviewResult, error) {
	if len(res.Raw) > 0 {
		if err := VerdictSchema(allowed).Validate(res.Raw); err != nil {
			return res, err
		}
	}
	norm := Verdict(strings.ToLower(strings.TrimSpace(string(res.Verdict))))
	for _, v := range allowed {
		if norm == v {
			res.Verdict = norm
			return res, nil
		}
	}
	return res, fmt.Errorf("%w: veredicto %q no está en %v", ErrInvalidOutput, res.Verdict, allowed)
}

// CallVerdict hace una llamada de veredicto y la revalida. Si la salida es inválida
// (no parsea o no cumple el esquema), la repite UNA vez con
// InvalidOutputRetryTemperature; si sigue inválida, devuelve un error que envuelve
// ErrInvalidOutput, junto con el último resultado crudo. Un error de transporte (que
// no envuelve ErrLLMQuality) se devuelve tal cual, sin reintento. call recibe la
// temperatura de la llamada: nil en la primera (la del proveedor).
func CallVerdict(ctx context.Context, allowed []Verdict, call func(ctx context.Context, temperature *float64) (ReviewResult, error)) (ReviewResult, error) {
	res, err := callAndValidate(ctx, allowed, call, nil)
	if err == nil || !errors.Is(err, ErrLLMQuality) {
		return res, err
	}
	retryTemp := InvalidOutputRetryTemperature
	res, retryErr := callAndValidate(ctx, allowed, call, &retryTemp)
	if retryErr == nil || !errors.Is(retryErr, ErrLLMQuality) {
		return res, retryErr
	}
	return res, fmt.Errorf("%w: tras un reintento: %v (primera: %v)", ErrInvalidOutput, retryErr, err)
}

func callAndValidate(ctx context.Context, allowed []Verdict, call func(context.Context, *float64) (ReviewResult, error), temperature *float64) (ReviewResult, error) {
	res, err := call(ctx, temperature)
	if err != nil {
		return res, err
	}
	return ValidateVerdictOutput(res, allowed)
}
