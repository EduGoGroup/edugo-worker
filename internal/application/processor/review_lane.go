package processor

import (
	"context"
	"time"

	"github.com/EduGoGroup/edugo-shared/logger"
	"github.com/EduGoGroup/edugo-worker/internal/client/m2m"
	"github.com/EduGoGroup/edugo-worker/internal/llm"
)

// ReviewLane es el carril por el que se resolvió una respuesta. Es el valor del
// campo `lane` del log de la revisión (HU-IA.2).
type ReviewLane string

const (
	// ReviewLaneList: corta con prep de lista (match determinista y pares).
	ReviewLaneList ReviewLane = "list"
	// ReviewLaneCriteria: larga con criterios (una llamada por criterio).
	ReviewLaneCriteria ReviewLane = "criteria"
	// ReviewLaneGlobal: juicio global del modelo.
	ReviewLaneGlobal ReviewLane = "global"
	// ReviewLaneBlankRule: respuesta en blanco resuelta por regla, sin modelo.
	ReviewLaneBlankRule ReviewLane = "blank_rule"
)

// ReviewLaneResult es lo que el carril de corrección produce para UNA respuesta
// antes de escribirla en learning: el resultado del modelo y los puntos que se
// propondrían.
type ReviewLaneResult struct {
	Result llm.ReviewResult
	Points float64
	// BlankByRule: la respuesta estaba en blanco y se resolvió por regla, sin modelo.
	BlankByRule bool
	// Lane: el carril que resolvió la respuesta. Se rellena también si hay error.
	Lane ReviewLane
	// LLMCalls: llamadas hechas al modelo para esta respuesta; cuentan las que
	// fallan y los reintentos.
	LLMCalls int
	// Duration: lo que tardó reviewAnswer, sin la escritura en learning.
	Duration time.Duration
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
