package processor

import (
	"context"

	"github.com/EduGoGroup/edugo-shared/logger"
	"github.com/EduGoGroup/edugo-worker/internal/client/m2m"
	"github.com/EduGoGroup/edugo-worker/internal/llm"
)

// ReviewLaneResult es lo que el carril de corrección produce para UNA respuesta
// antes de escribirla en learning: el resultado del modelo y los puntos que se
// propondrían.
type ReviewLaneResult struct {
	Result llm.ReviewResult
	Points float64
}

// ReviewAnswerLane hace, para una respuesta, lo que el bucle de Process hace antes
// de escribir: elige el carril según el prep (reviewOne), llama al modelo, valida
// el veredicto (validateVerdict) y escala los puntos (scaledPoints). No escribe
// nada, no toma el candado y no lee settings.
//
// Es la entrada exportada que usa el arnés del worker para medir el carril real
// con el banco etiquetado (AR-INF.1, decisión 3): llama al código de producción,
// no a una copia. reviewOne solo usa el logger del procesador, así que basta un
// procesador con logger. El bucle de producción no la usa y no cambia.
//
// Errores:
//   - un error del proveedor (red, timeout, salida que no parsea) se devuelve tal
//     cual, con el resultado vacío;
//   - un veredicto inválido para el tipo de pregunta devuelve un error que envuelve
//     ErrInvalidVerdict, junto con el resultado crudo del modelo, para que quien
//     mide lo pueda contar aparte.
func ReviewAnswerLane(ctx context.Context, provider llm.LLMProvider, ans m2m.PendingAnswer, log logger.Logger) (ReviewLaneResult, error) {
	p := &AttemptReviewProcessor{logger: log}

	result, err := p.reviewOne(ctx, provider, ans)
	if err != nil {
		return ReviewLaneResult{}, err
	}
	if err := validateVerdict(ans.QuestionType, result.Verdict); err != nil {
		return ReviewLaneResult{Result: result}, err
	}
	return ReviewLaneResult{Result: result, Points: scaledPoints(result.Score, ans.Points)}, nil
}
