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
	// BlankByRule: la respuesta estaba en blanco y se resolvió por regla, sin modelo.
	BlankByRule bool
}

// ReviewAnswerLane hace, para una respuesta, lo que el bucle de Process hace antes
// de escribir, por el mismo sitio (reviewAnswer): resuelve la respuesta en blanco
// por regla, elige el carril según el prep (reviewOne), llama al modelo, revalida
// el veredicto y deriva los puntos. No escribe nada, no toma el candado y no lee
// settings.
//
// Es la entrada exportada que usa el arnés del worker para medir el carril real
// con el banco etiquetado (AR-INF.1, decisión 3): llama al código de producción,
// no a una copia. reviewOne solo usa el logger del procesador, así que basta un
// procesador con logger. El bucle de producción no la usa y no cambia.
//
// Errores:
//   - un error del proveedor (red, timeout) se devuelve tal cual;
//   - una salida inválida en el juicio global (que no parsea o fuera de esquema),
//     tras su único reintento, devuelve un error que envuelve ErrInvalidVerdict,
//     junto con el último resultado crudo del modelo, para que quien mide lo pueda
//     contar aparte;
//   - una sub-llamada (par o criterio) que sigue inválida tras su reintento devuelve
//     un error que envuelve llm.ErrInvalidOutput.
func ReviewAnswerLane(ctx context.Context, provider llm.LLMProvider, ans m2m.PendingAnswer, log logger.Logger) (ReviewLaneResult, error) {
	p := &AttemptReviewProcessor{logger: log}
	return p.reviewAnswer(ctx, provider, ans)
}
