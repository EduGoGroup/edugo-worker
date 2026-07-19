package reduce

import (
	"context"
	"fmt"

	"github.com/EduGoGroup/edugo-shared/logger"
	"github.com/EduGoGroup/edugo-worker/internal/assessmentimport"
	"github.com/EduGoGroup/edugo-worker/internal/client/m2m"
	"github.com/EduGoGroup/edugo-worker/internal/materialpipeline"
)

// QualityPass ejecuta la pasada 3 del reduce (calidad determinista, D-044.3, gratis): valida
// el payload de cada candidata viva contra el contrato import v1 + los límites anti-abuso
// del 038 y descarta las que no pasan. NO gasta LLM. Aislada del processor (cableado en F3c).
type QualityPass struct {
	store  candidateStore
	logger logger.Logger
}

// NewQualityPass construye la pasada.
func NewQualityPass(store candidateStore, log logger.Logger) *QualityPass {
	return &QualityPass{store: store, logger: log}
}

// QualityReport resume lo que hizo la pasada sobre un job (logs/harness/observabilidad).
type QualityReport struct {
	Candidates     int // total de candidatas leídas del job
	Processed      int // candidatas status=candidate evaluadas
	Valid          int // candidatas que pasaron la validación (siguen en candidate)
	DroppedInvalid int // candidatas descartadas → dropped_irrelevant
}

// Run corre la pasada 3 sobre un job: valida cada candidata viva (status=candidate) contra
// el contrato import v1 (materialpipeline.ValidateCandidatePayload — tipos, opciones
// completas y ≥2 en mc/ms, correct_answer presente en opciones, obligatorio salvo
// open_ended) más el techo anti-abuso de opciones del 038 (≤ MaxOptionsPerQ). La que no
// valida → dropped_irrelevant (contrato §4: no hay estado nuevo, «no apta» ⊂ «no entra al
// draft»). Idempotente: las terminales se saltan; nada se borra.
//
// Nota de alcance: el contrato import v1 (038) NO define un tope de longitud para
// question_text/explanation (solo título ≤255 y el JSON ≤1 MiB, verificados al importar);
// aquí «dentro de límites» = las cardinalidades y el tope de opciones que 038 sí fija +
// no-vacío. Un tope de longitud de texto sería un cambio de contrato (no se inventa aquí).
func (q *QualityPass) Run(ctx context.Context, jobID string) (QualityReport, error) {
	records, err := q.store.ListCandidates(ctx, jobID)
	if err != nil {
		return QualityReport{}, fmt.Errorf("listando candidatas del job %s: %w", jobID, err)
	}
	report := QualityReport{Candidates: len(records)}

	var updates []m2m.CandidateUpdate
	for i := range records {
		rec := records[i]
		if rec.Status != statusCandidate {
			continue // terminal: absorbente (idempotencia por status)
		}
		report.Processed++

		if reason := validateCandidateQuality(rec.Payload); reason != "" {
			dropped := statusDroppedIrrelevant
			updates = append(updates, m2m.CandidateUpdate{ID: rec.ID, Status: &dropped})
			report.DroppedInvalid++
			q.logger.Warn("candidata descartada por calidad (no valida el contrato import)",
				"job_id", jobID, "candidate_id", rec.ID, "motivo", reason)
			continue
		}
		report.Valid++
	}

	if len(updates) > 0 {
		if _, err := q.store.UpdateCandidates(ctx, updates); err != nil {
			return report, fmt.Errorf("persistiendo descartes de calidad del job %s: %w", jobID, err)
		}
	}

	q.logger.Info("pasada 3 de calidad completa",
		"job_id", jobID,
		"candidatas", report.Candidates,
		"procesadas", report.Processed,
		"validas", report.Valid,
		"dropped_invalid", report.DroppedInvalid)
	return report, nil
}

// validateCandidateQuality valida el payload contra el contrato import v1 (reusa
// materialpipeline.ValidateCandidatePayload — no se duplican reglas) más el techo de
// opciones anti-abuso del 038 (que el validador de candidata no cubre: solo verifica el
// mínimo por tipo, no el máximo). Devuelve "" si es válida, o un motivo corto para el log.
func validateCandidateQuality(raw []byte) string {
	payload, err := materialpipeline.ValidateCandidatePayload(raw)
	if err != nil {
		return err.Error()
	}
	if len(payload.Options) > assessmentimport.DefaultMaxOptionsPerQ {
		return fmt.Sprintf("%d opciones exceden el máximo de %d", len(payload.Options), assessmentimport.DefaultMaxOptionsPerQ)
	}
	return ""
}
