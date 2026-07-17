package processor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/EduGoGroup/edugo-shared/logger"
	"github.com/EduGoGroup/edugo-shared/messaging/events"
	"github.com/EduGoGroup/edugo-worker/internal/client/m2m"
	"github.com/EduGoGroup/edugo-worker/internal/llm"
	"github.com/EduGoGroup/edugo-worker/internal/openended"
	"github.com/EduGoGroup/edugo-worker/internal/questionprep"
	"github.com/EduGoGroup/edugo-worker/internal/shortanswer"
)

// EventTypeAttemptReviewRequested es el event_type que enruta el registry hacia
// este processor. Debe coincidir con la routing key del binding en setupRabbitMQ
// y con el event_type que publica learning.
const EventTypeAttemptReviewRequested = "attempt.review_requested"

// Claves de política por escuela leídas vía SettingsClient (design 039/040 §rieles).
const (
	settingKeyReviewMode = "llm.review.mode" // local | api | off
	settingKeyReviewFlow = "llm.review.flow" // direct | teacher
)

// Valores del carril de revisión (design 040 §rieles).
const (
	// mode
	reviewModeOff   = "off"   // revisión apagada (default de plataforma)
	reviewModeLocal = "local" // provider LLM local (Ollama)
	reviewModeAPI   = "api"   // provider LLM por API (Anthropic)
	// flow
	reviewFlowTeacher = "teacher" // deja el intento ai_reviewed para el profesor (default)
	reviewFlowDirect  = "direct"  // finaliza el intento tras revisar (sin docente)
)

// reviewLanguage es el idioma que se pide al LLM para el feedback. El carril es
// español (regla global del ecosistema).
const reviewLanguage = "es"

// ErrMalformedEvent marca un evento que no se puede decodificar/validar. El
// clasificador de retry lo trata como permanente: reintentar no lo arregla, va al
// DLQ. Ver classifyError en retry.go.
var ErrMalformedEvent = errors.New("evento attempt.review_requested malformado")

// ErrInvalidVerdict marca una respuesta del LLM cuyo verdict no es uno de los
// válidos (p.ej. el `{}` que devuelve qwen3 con thinking + format:json, que
// deserializa a verdict=""). Se trata como fallo del LLM (transitorio, como un
// error del provider): NO se postea una review IA de 0 puntos «propuesta por IA»
// —sería un juicio falso—; la answer queda sin review para que el profesor la
// corrija a mano. classifyError no lo lista como permanente, así que reusa el
// carril de retry existente de un fallo de provider.
var ErrInvalidVerdict = errors.New("veredicto del LLM inválido")

// SchoolSettingsReader es la porción del SettingsClient M2M que usa el processor.
// Se define como interfaz para poder mockearla en tests. *m2m.SettingsClient la
// satisface.
type SchoolSettingsReader interface {
	GetSettings(ctx context.Context, schoolID string) (m2m.SchoolSettings, error)
}

// LearningReviewClient es la porción del LearningClient M2M que usa el processor
// para orquestar la revisión. Se define como interfaz para mockearla en tests.
// *m2m.LearningClient la satisface.
type LearningReviewClient interface {
	// Claim reclama el candado «en revisión por IA» del intento antes de procesar.
	// Un m2m.ErrClaimConflict (409) obliga a abstenerse (no es fallo).
	Claim(ctx context.Context, attemptID string) error
	// ReleaseClaim libera el candado al terminar el lote cuando NO se finaliza
	// (flujo teacher / short_answer). Idempotente.
	ReleaseClaim(ctx context.Context, attemptID string) error
	GetPendingAnswers(ctx context.Context, attemptID string) (m2m.PendingAnswersResponse, error)
	PostAnswerReview(ctx context.Context, attemptID, answerID string, review m2m.AnswerReviewRequest) (m2m.AnswerReviewResponse, error)
	FinalizeAttempt(ctx context.Context, attemptID string) (m2m.FinalizeResponse, error)
}

// AttemptReviewProcessor consume attempt.review_requested y orquesta la revisión
// asistida por LLM de un intento entregado.
//
// Flujo (F2): decodifica/valida el evento, lee la política de la escuela y, si la
// revisión está activa (mode=local|api), lee las respuestas pendientes de learning
// (M2M), las corrige con el LLMProvider del mode y escribe cada review de vuelta.
// Con flow=direct finaliza el intento; con flow=teacher lo deja ai_reviewed para el
// profesor (F3).
type AttemptReviewProcessor struct {
	settings  SchoolSettingsReader
	learning  LearningReviewClient
	providers map[string]llm.LLMProvider
	logger    logger.Logger
}

// NewAttemptReviewProcessor construye el processor. providers mapea el mode
// ("local"/"api") al LLMProvider correspondiente; un mode sin provider disponible
// se trata como configuración inválida al procesar.
func NewAttemptReviewProcessor(
	settings SchoolSettingsReader,
	learning LearningReviewClient,
	providers map[string]llm.LLMProvider,
	log logger.Logger,
) *AttemptReviewProcessor {
	return &AttemptReviewProcessor{
		settings:  settings,
		learning:  learning,
		providers: providers,
		logger:    log,
	}
}

// EventType satisface processor.Processor.
func (p *AttemptReviewProcessor) EventType() string { return EventTypeAttemptReviewRequested }

// Process decodifica el evento, aplica la política de la escuela y (F1) se detiene
// antes de orquestar. Errores:
//   - evento malformado (decode/validación) → ErrMalformedEvent (permanente → DLQ).
//   - settings inaccesible → error transitorio (el consumer reintenta con backoff).
func (p *AttemptReviewProcessor) Process(ctx context.Context, payload []byte) error {
	var evt events.AttemptReviewRequestedEvent
	if err := json.Unmarshal(payload, &evt); err != nil {
		return fmt.Errorf("%w: decode: %v", ErrMalformedEvent, err)
	}

	if err := validateReviewEvent(evt); err != nil {
		return fmt.Errorf("%w: %v", ErrMalformedEvent, err)
	}

	// Leer política de la escuela. Un fallo aquí es transitorio (academic caído,
	// timeout): se devuelve tal cual para que el consumer reintente.
	settings, err := p.settings.GetSettings(ctx, evt.Payload.SchoolID)
	if err != nil {
		return fmt.Errorf("leyendo settings de escuela %s: %w", evt.Payload.SchoolID, err)
	}

	mode := settingValueOr(settings, settingKeyReviewMode, reviewModeOff)
	flow := settingValueOr(settings, settingKeyReviewFlow, reviewFlowTeacher)

	// Corto-circuito: revisión apagada para esta escuela.
	if mode == reviewModeOff {
		p.logger.Info("review apagado para la escuela, se ignora (ACK)",
			"attempt_id", evt.Payload.AttemptID,
			"school_id", evt.Payload.SchoolID,
			"mode", mode,
		)
		return nil
	}

	return p.orchestrate(ctx, evt.Payload.AttemptID, mode, flow, evt.Payload.Answers)
}

// orchestrate ejecuta la revisión asistida de un intento con la política resuelta.
//
// Idempotencia y retry (gate de tasks.md): todo el flujo es seguro de reprocesar.
// El GET re-lee solo las respuestas AÚN pendientes y el POST review es upsert del
// lado de learning, así que un redelivery tras un fallo a mitad no duplica ni
// corrompe. Por eso los fallos transitorios (LLM o M2M 5xx/red) se devuelven como
// error para que el consumer reintente. Un 4xx permanente de learning
// (ErrLearningPermanent) o un mode sin provider se envuelven en un error que el
// clasificador marca permanente; aun así el consumer con DLQ reintenta MaxRetries
// antes de mandar el mensaje al DLQ (ConsumeWithDLQ no consulta el clasificador),
// lo cual es inofensivo porque el reproceso es idempotente.
func (p *AttemptReviewProcessor) orchestrate(ctx context.Context, attemptID, mode, flow string, answers []events.AttemptReviewAnswerRef) error {
	provider, ok := p.providers[mode]
	if !ok || provider == nil {
		// mode desconocido o provider no disponible: es config errónea, no un fallo
		// transitorio. Permanente para no reintentar en vano.
		return fmt.Errorf("%w: no hay LLMProvider para mode=%q", ErrMalformedEvent, mode)
	}

	// Candado «en revisión por IA»: reclamar ANTES de tocar nada, para que profe y
	// worker no choquen (plan 040 F4). Un 409 (candado ajeno vigente o intento ya
	// completado por el profe) NO es un fallo: el worker se abstiene y ACKea.
	if err := p.learning.Claim(ctx, attemptID); err != nil {
		if errors.Is(err, m2m.ErrClaimConflict) {
			p.logger.Info("intento no reclamable (candado ajeno o ya completado), se abstiene (ACK)",
				"attempt_id", attemptID, "motivo", err.Error())
			return nil
		}
		return fmt.Errorf("reclamando candado de attempt %s: %w", attemptID, err)
	}

	// Regla de fondo (mixto open_ended + short_answer): la presencia de CUALQUIER
	// short_answer fuerza el flujo teacher (no finalizar), porque la corrección de
	// respuesta corta es una PROPUESTA que el profesor debe visar. Solo se finaliza
	// un intento 100% open_ended con flow=direct. Ver reporte del paso 1.
	hasShortAnswer := false
	for _, a := range answers {
		if a.QuestionType == llm.QuestionTypeShortAnswer {
			hasShortAnswer = true
			break
		}
	}
	finalizeAtEnd := flow == reviewFlowDirect && !hasShortAnswer

	pending, err := p.learning.GetPendingAnswers(ctx, attemptID)
	if err != nil {
		return fmt.Errorf("leyendo answers pendientes de attempt %s: %w", attemptID, err)
	}

	// Sin pendientes: nada que corregir. Puede ser un redelivery de un intento ya
	// revisado. Si toca finalizar (direct + solo open_ended) intentamos finalize
	// idempotente; si no, liberamos el candado para el profesor.
	if len(pending.Answers) == 0 {
		if finalizeAtEnd {
			return p.finalize(ctx, attemptID, "sin pendientes (posible redelivery)")
		}
		p.logger.Info("review sin pendientes, flujo teacher/short_answer: release-claim",
			"attempt_id", attemptID, "mode", mode, "flow", flow, "has_short_answer", hasShortAnswer)
		p.releaseClaim(ctx, attemptID, "sin pendientes (posible redelivery)")
		return nil
	}

	for _, ans := range pending.Answers {
		result, err := p.reviewOne(ctx, provider, ans)
		if err != nil {
			// Fallo del LLM: transitorio. Reintentar es seguro (aún no escribimos esta
			// review; las ya escritas no vuelven a aparecer en el GET pending).
			return fmt.Errorf("LLM revisando answer %s (attempt %s): %w", ans.AnswerID, attemptID, err)
		}

		// Guardia anti-basura: si el verdict no es válido para el tipo de pregunta
		// (p.ej. "" por un `{}` de qwen3), el LLM no emitió un juicio usable. Se trata
		// igual que un fallo del provider (mismo carril de retry) y NO se postea una
		// review IA de 0 puntos «propuesta» — la answer queda para el profesor.
		if err := validateVerdict(ans.QuestionType, result.Verdict); err != nil {
			p.logger.Warn("veredicto del LLM inválido, se descarta la propuesta (no se postea review IA)",
				"attempt_id", attemptID,
				"answer_id", ans.AnswerID,
				"question_type", ans.QuestionType,
				"verdict", string(result.Verdict),
				"score", result.Score,
				"provider", provider.Name(),
			)
			return fmt.Errorf("revisando answer %s (attempt %s): %w", ans.AnswerID, attemptID, err)
		}

		points := scaledPoints(result.Score, ans.Points)
		if _, err := p.learning.PostAnswerReview(ctx, attemptID, ans.AnswerID, m2m.AnswerReviewRequest{
			PointsAwarded: points,
			Feedback:      result.Feedback,
		}); err != nil {
			return fmt.Errorf("escribiendo review de answer %s (attempt %s): %w", ans.AnswerID, attemptID, err)
		}

		p.logger.Info("answer revisada por LLM",
			"attempt_id", attemptID,
			"answer_id", ans.AnswerID,
			"question_type", ans.QuestionType,
			"verdict", string(result.Verdict),
			"score", result.Score,
			"points_awarded", points,
			"provider", provider.Name(),
		)
	}

	// Todas las pendientes quedaron revisadas.
	if finalizeAtEnd {
		return p.finalize(ctx, attemptID, "todas las respuestas revisadas")
	}

	// Flujo teacher (o presencia de short_answer): NO finalize. Se libera el candado
	// y el intento queda ai_reviewed para que el profesor lo vise (plan 040 F4).
	p.logger.Info("review completada, flujo teacher/short_answer: release-claim (sin finalize)",
		"attempt_id", attemptID, "answers", len(pending.Answers), "has_short_answer", hasShortAnswer)
	p.releaseClaim(ctx, attemptID, "revisión completada, intento queda para el profesor")
	return nil
}

// reviewOne produce el ReviewResult de UNA answer, eligiendo el carril según el prep
// (plan 042 F3c). short_answer con prep content_kind=list ⇒ carril TRITURADO (match
// determinista + pares binarios, reemplaza el juicio global). short_answer con prep
// de otro content_kind ⇒ prompt global enriquecido con los ítems normalizados. Sin
// prep (o inválido) o cualquier otro tipo ⇒ flujo global actual intacto.
func (p *AttemptReviewProcessor) reviewOne(ctx context.Context, provider llm.LLMProvider, ans m2m.PendingAnswer) (llm.ReviewResult, error) {
	prep := p.parsePrep(ans)

	req := llm.ReviewRequest{
		QuestionType:   ans.QuestionType,
		QuestionText:   ans.QuestionText,
		ExpectedAnswer: ans.ExpectedAnswer,
		Rubric:         ans.Rubric,
		StudentAnswer:  ans.StudentAnswer,
		Language:       reviewLanguage,
	}

	if ans.QuestionType == llm.QuestionTypeShortAnswer && prep != nil {
		if prep.ContentKind == questionprep.ContentKindList {
			p.logger.Info("short_answer con prep list: carril triturado (match determinista + pares)",
				"answer_id", ans.AnswerID, "items", len(prep.Items))
			return shortanswer.Grade(ctx, provider, shortanswer.GradeInput{
				QuestionText:  ans.QuestionText,
				StudentAnswer: ans.StudentAnswer,
				Items:         prep.Items,
				ItemsVerbatim: prep.ItemsVerbatim,
				Language:      reviewLanguage,
			})
		}
		// term/number/date/free: mejora barata del prompt global (D-042.10 §short_answer,
		// punto 5) sin cambiar el contrato del POST review.
		req.ExpectedAnswer = enrichExpectedWithPrep(ans.ExpectedAnswer, prep)
	}

	if ans.QuestionType == questionprep.QuestionTypeOpenEnded && prep != nil {
		// F4b: con criterios reales (≥1 no vacío), el juicio global se reemplaza por una
		// llamada binaria por criterio + agregación determinista en Go. Filtramos criterios
		// en blanco por defensa: un prep con solo criterios vacíos NO debe entrar aquí (daría
		// incorrect/0.0 silencioso); cae al fallback F4a de rúbrica global. El validador ya
		// rechaza esos preps, pero no lo asumimos en el carril de corrección.
		criteria := nonBlankCriteria(prep.Criteria)
		if len(criteria) > 0 {
			p.logger.Info("open_ended con prep criteria: carril por criterios (una llamada por criterio)",
				"answer_id", ans.AnswerID, "criteria", len(criteria))
			return openended.Grade(ctx, provider, openended.GradeInput{
				QuestionText:   ans.QuestionText,
				ExpectedAnswer: ans.ExpectedAnswer,
				StudentAnswer:  ans.StudentAnswer,
				Criteria:       criteria,
				Language:       reviewLanguage,
			})
		}
		// F4a: sin criterios reales, se enriquece el prompt global con intención/ideas/variantes.
		req.Prep = &llm.ReviewPrep{
			QuestionIntent: prep.QuestionIntent,
			MainIdeas:      prep.MainIdeas,
			SecondaryIdeas: prep.SecondaryIdeas,
			ValidVariants:  prep.ValidVariants,
		}
	}

	return provider.ReviewAnswer(ctx, req)
}

// parsePrep decodifica y valida el llm_prep que learning adjuntó a la answer contra
// el contrato v1 (reusa el validador de F2). Ausente o inválido ⇒ nil: el carril
// degrada al flujo global (D-042.10 §4, el carril de corrección nunca espera al de
// preparación). Un prep inválido se loguea pero no es fatal.
func (p *AttemptReviewProcessor) parsePrep(ans m2m.PendingAnswer) *questionprep.Prep {
	if len(ans.LLMPrep) == 0 {
		return nil
	}
	prep, err := questionprep.Validate(ans.LLMPrep, ans.QuestionType)
	if err != nil {
		p.logger.Warn("llm_prep inválido en pending answer, se ignora (flujo global)",
			"answer_id", ans.AnswerID, "question_type", ans.QuestionType, "motivo", err.Error())
		return nil
	}
	return prep
}

// enrichExpectedWithPrep añade a la respuesta esperada los ítems normalizados del
// prep (term/number/date/free) como pista extra para el prompt global. No cambia el
// contrato del POST review: solo enriquece el texto que ya viaja en ExpectedAnswer.
func enrichExpectedWithPrep(expected string, prep *questionprep.Prep) string {
	if len(prep.Items) == 0 {
		return expected
	}
	norm := strings.Join(prep.Items, ", ")
	if strings.TrimSpace(expected) == "" {
		return norm
	}
	return expected + "\nRespuesta esperada (normalizada): " + norm
}

// nonBlankCriteria devuelve los criterios no vacíos (tras TrimSpace) conservando el
// orden. Es la frontera del carril F4b: si queda ≥1 se corrige por criterios; si queda 0
// el carril cae al fallback de rúbrica global (F4a), nunca a un incorrect/0.0 silencioso.
func nonBlankCriteria(criteria []string) []string {
	var out []string
	for _, c := range criteria {
		if strings.TrimSpace(c) != "" {
			out = append(out, c)
		}
	}
	return out
}

// releaseClaim libera el candado de mejor esfuerzo: un fallo NO es fatal (el
// candado vence por TTL del lado de learning), así que se loguea y se sigue. Las
// reviews ya quedaron escritas; reprocesar por un release fallido sería en vano.
func (p *AttemptReviewProcessor) releaseClaim(ctx context.Context, attemptID, reason string) {
	if err := p.learning.ReleaseClaim(ctx, attemptID); err != nil {
		p.logger.Warn("no se pudo liberar el candado (vencerá por TTL)",
			"attempt_id", attemptID, "motivo", err.Error(), "contexto", reason)
		return
	}
	p.logger.Info("candado liberado", "attempt_id", attemptID, "motivo", reason)
}

// finalize cierra la revisión del intento (flow direct). Es idempotente del lado
// de learning (200 no-op si ya estaba completed).
func (p *AttemptReviewProcessor) finalize(ctx context.Context, attemptID, reason string) error {
	resp, err := p.learning.FinalizeAttempt(ctx, attemptID)
	if err != nil {
		return fmt.Errorf("finalizando attempt %s: %w", attemptID, err)
	}
	p.logger.Info("attempt finalizado (flow direct)",
		"attempt_id", attemptID, "status", resp.Status, "motivo", reason)
	return nil
}

// scaledPoints escala el Score del LLM (fracción 0..1) al puntaje real de la
// pregunta y lo redondea a 2 decimales (paso mínimo razonable para puntajes
// escolares; evita colas binarias tipo 4.999999). El Score se acota a [0,1] por
// si el modelo devuelve un valor fuera de rango.
func scaledPoints(score, questionPoints float64) float64 {
	if score < 0 {
		score = 0
	}
	if score > 1 {
		score = 1
	}
	return math.Round(score*questionPoints*100) / 100
}

// validateVerdict comprueba que el veredicto del LLM sea uno de los aceptables
// para el tipo de pregunta: short_answer es binario (equivalencia correct/
// incorrect); open_ended (o tipo vacío por compatibilidad F3) admite además el
// parcial de rúbrica. Un veredicto fuera de este conjunto —incluido el vacío de
// un `{}`— es ErrInvalidVerdict.
func validateVerdict(questionType string, v llm.Verdict) error {
	switch questionType {
	case llm.QuestionTypeShortAnswer:
		if v == llm.VerdictCorrect || v == llm.VerdictIncorrect {
			return nil
		}
	default: // open_ended o vacío (compatibilidad F3)
		if v == llm.VerdictCorrect || v == llm.VerdictPartial || v == llm.VerdictIncorrect {
			return nil
		}
	}
	return fmt.Errorf("%w: %q no es válido para question_type %q", ErrInvalidVerdict, v, questionType)
}

// validateReviewEvent comprueba los campos mínimos que necesita el carril. No
// reusa el constructor del shared porque ese regenera el Timestamp.
func validateReviewEvent(evt events.AttemptReviewRequestedEvent) error {
	if evt.EventType != EventTypeAttemptReviewRequested {
		return fmt.Errorf("event_type inesperado: %q", evt.EventType)
	}
	if evt.Payload.AttemptID == "" {
		return errors.New("attempt_id vacío")
	}
	if evt.Payload.AssessmentID == "" {
		return errors.New("assessment_id vacío")
	}
	if evt.Payload.SchoolID == "" {
		return errors.New("school_id vacío")
	}
	if len(evt.Payload.Answers) == 0 {
		return errors.New("answers vacío")
	}
	return nil
}

// settingValueOr devuelve el valor resuelto de una clave o el default de plataforma.
func settingValueOr(s m2m.SchoolSettings, key, def string) string {
	if v, ok := s.Get(key); ok && v != "" {
		return v
	}
	return def
}
