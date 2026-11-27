package llm

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestVerdictSchema_Validate(t *testing.T) {
	schema := VerdictSchema(BinaryVerdicts)
	cases := []struct {
		name string
		raw  string
		ok   bool
	}{
		{"válida", `{"verdict":"correct","score":1,"feedback":"bien"}`, true},
		{"veredicto en mayúsculas", `{"verdict":"INCORRECT","score":0,"feedback":"no"}`, true},
		{"score fuera de rango: no se rechaza", `{"verdict":"correct","score":5000000000000000,"feedback":"x"}`, true},
		{"clave de más: no se rechaza", `{"verdict":"correct","score":1,"feedback":"x","extra":1}`, true},
		{"falta score", `{"verdict":"correct","feedback":"x"}`, false},
		{"falta veredicto", `{"score":1,"feedback":"x"}`, false},
		{"objeto vacío", `{}`, false},
		{"veredicto fuera de la lista", `{"verdict":"partial","score":0.5,"feedback":"x"}`, false},
		{"veredicto inventado", `{"verdict":"maybe","score":0.5,"feedback":"x"}`, false},
		{"score como cadena", `{"verdict":"correct","score":"1.0","feedback":"x"}`, false},
		{"feedback como número", `{"verdict":"correct","score":1,"feedback":3}`, false},
		{"no es objeto", `["correct"]`, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := schema.Validate(json.RawMessage(c.raw))
			if c.ok && err != nil {
				t.Fatalf("se esperaba válida, salió %v", err)
			}
			if !c.ok && !errors.Is(err, ErrInvalidOutput) {
				t.Fatalf("se esperaba ErrInvalidOutput, salió %v", err)
			}
		})
	}
}

func TestReviewVerdicts_ByQuestionType(t *testing.T) {
	if got := ReviewVerdicts(QuestionTypeShortAnswer); len(got) != 2 {
		t.Fatalf("la corta es binaria, salió %v", got)
	}
	for _, qt := range []string{QuestionTypeOpenEnded, ""} {
		if got := ReviewVerdicts(qt); len(got) != 3 || got[1] != VerdictPartial {
			t.Fatalf("%q admite el parcial, salió %v", qt, got)
		}
	}
}

func TestOutputSchema_JSON(t *testing.T) {
	got := string(VerdictSchema(BinaryVerdicts).JSON())
	for _, want := range []string{`"type":"object"`, `"enum":["correct","incorrect"]`, `"required":["verdict","score","feedback"]`} {
		if !strings.Contains(got, want) {
			t.Errorf("el esquema no lleva %s: %s", want, got)
		}
	}
	if err := IdeasSchema.Validate(json.RawMessage(`{"ideas":["a",2]}`)); !errors.Is(err, ErrInvalidOutput) {
		t.Fatalf("una idea que no es cadena: se esperaba ErrInvalidOutput, salió %v", err)
	}
}

func TestValidateVerdictOutput(t *testing.T) {
	res, err := ValidateVerdictOutput(ReviewResult{Verdict: " Correct "}, BinaryVerdicts)
	if err != nil || res.Verdict != VerdictCorrect {
		t.Fatalf("sin crudo se valida el veredicto y se normaliza: %v %q", err, res.Verdict)
	}
	withRaw := ReviewResult{Verdict: VerdictCorrect, Raw: json.RawMessage(`{"verdict":"correct"}`)}
	if _, err := ValidateVerdictOutput(withRaw, BinaryVerdicts); !errors.Is(err, ErrInvalidOutput) {
		t.Fatalf("con crudo manda el esquema entero (falta score): %v", err)
	}
}

// scriptedCall devuelve, llamada a llamada, lo que se le fija, y guarda la
// temperatura con que se pidió cada una.
type scriptedCall struct {
	outs  []ReviewResult
	errs  []error
	temps []*float64
}

func (s *scriptedCall) call(_ context.Context, temperature *float64) (ReviewResult, error) {
	i := len(s.temps)
	s.temps = append(s.temps, temperature)
	var err error
	if i < len(s.errs) {
		err = s.errs[i]
	}
	if i < len(s.outs) {
		return s.outs[i], err
	}
	return ReviewResult{}, err
}

func TestCallVerdict(t *testing.T) {
	valid := ReviewResult{Verdict: VerdictCorrect, Score: 1}

	t.Run("válida a la primera: una llamada, a la temperatura del proveedor", func(t *testing.T) {
		s := &scriptedCall{outs: []ReviewResult{valid}}
		if _, err := CallVerdict(context.Background(), BinaryVerdicts, s.call); err != nil {
			t.Fatal(err)
		}
		if len(s.temps) != 1 || s.temps[0] != nil {
			t.Fatalf("se esperaba una llamada sin temperatura propia, hubo %d", len(s.temps))
		}
	})

	t.Run("inválida y luego válida: un reintento con otra temperatura", func(t *testing.T) {
		s := &scriptedCall{outs: []ReviewResult{{Verdict: "maybe"}, valid}}
		res, err := CallVerdict(context.Background(), BinaryVerdicts, s.call)
		if err != nil || res.Verdict != VerdictCorrect {
			t.Fatalf("el reintento válido sigue: %v %q", err, res.Verdict)
		}
		if len(s.temps) != 2 || s.temps[1] == nil || *s.temps[1] != InvalidOutputRetryTemperature {
			t.Fatalf("se esperaba un reintento a %.1f, hubo %d llamadas", InvalidOutputRetryTemperature, len(s.temps))
		}
	})

	t.Run("no parsea dos veces: error, nunca un «no»", func(t *testing.T) {
		quality := ErrLLMQuality
		s := &scriptedCall{errs: []error{quality, quality}}
		_, err := CallVerdict(context.Background(), BinaryVerdicts, s.call)
		if !errors.Is(err, ErrInvalidOutput) || len(s.temps) != 2 {
			t.Fatalf("se esperaba ErrInvalidOutput tras 2 llamadas: %v (%d)", err, len(s.temps))
		}
	})

	t.Run("error de transporte: se propaga sin reintento", func(t *testing.T) {
		boom := errors.New("connection refused")
		s := &scriptedCall{errs: []error{boom}}
		_, err := CallVerdict(context.Background(), BinaryVerdicts, s.call)
		if !errors.Is(err, boom) || errors.Is(err, ErrLLMQuality) || len(s.temps) != 1 {
			t.Fatalf("se esperaba el error tal cual y una sola llamada: %v (%d)", err, len(s.temps))
		}
	})
}

func TestDecodeVerdictOutput(t *testing.T) {
	res, err := DecodeVerdictOutput("```json\n{\"verdict\":\"Partial\",\"score\":0.5,\"feedback\":\"x\"}\n```", "corrección")
	if err != nil || res.Verdict != "Partial" || len(res.Raw) == 0 {
		t.Fatalf("decodifica sin validar y adjunta el crudo: %v %q %s", err, res.Verdict, res.Raw)
	}
	for _, out := range []string{"", "sin json", `{"verdict":"correct","score":"1"}`} {
		_, err := DecodeVerdictOutput(out, "corrección")
		if !errors.Is(err, ErrLLMQuality) || !strings.Contains(err.Error(), "no parseable") {
			t.Fatalf("%q: se esperaba un error de calidad «no parseable», salió %v", out, err)
		}
	}
}
