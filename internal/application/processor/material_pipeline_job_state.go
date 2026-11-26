package processor

// material_pipeline_job_state.go — lo que el worker le dice a learning sobre el
// estado del job del carril material→evaluación (HU-IA.5): los motivos que quedan en
// last_error, el cierre cuando el pipeline está apagado, el cierre en el último
// reintento y el latido mientras Process tiene el job.

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/EduGoGroup/edugo-shared/messaging/rabbit"
	"github.com/EduGoGroup/edugo-worker/internal/client/m2m"
	pdfErrors "github.com/EduGoGroup/edugo-worker/internal/infrastructure/pdf"
)

// jobHeartbeatInterval es cada cuánto late el job mientras un worker lo procesa: un
// PATCH processing que solo refresca updated_at en learning. El barrido de jobs
// colgados (HU-IA.6) cierra un job processing que lleva 15 minutos sin latir.
const jobHeartbeatInterval = time.Minute

// maxJobLastErrorBytes es el tope de last_error que el worker manda a learning.
const maxJobLastErrorBytes = 1024

// jobReasonSeparator separa el motivo que ve la docente del detalle técnico.
const jobReasonSeparator = " · "

// Los motivos de un job fallado. Van PRIMERO en last_error y en español porque el
// cliente los enseña tal cual y corta a 160 caracteres; el detalle técnico va detrás
// del separador.
const (
	jobReasonPipelineOff   = "La generación con IA está apagada en esta escuela."
	jobReasonUnreadablePDF = "No se pudo leer el PDF."
	jobReasonNoSelected    = "Ninguna pregunta generada pasó los filtros: no hay borrador."
	jobReasonDraftRejected = "El borrador no se pudo guardar."
	jobReasonPermanent     = "No se pudo generar la evaluación."
	jobReasonServiceDown   = "El servicio de IA no respondió tras varios intentos."
)

// errDraftDelivery marca un fallo de la entrega del borrador (el POST deliver), para
// que el motivo distinga «el borrador se rechazó al guardarlo» de otro permanente de
// la fase 2. No cambia la clasificación del error: solo lo etiqueta.
var errDraftDelivery = errors.New("entrega del borrador")

// jobFailureReason arma el last_error de un job que falla por un error permanente: el
// motivo de la tabla de la HU-IA.5 según la causa y, detrás, el detalle técnico.
func jobFailureReason(phase int16, err error) string {
	return composeJobReason(permanentReasonFor(phase, err), err)
}

// permanentReasonFor elige el motivo que ve la docente para un error permanente.
func permanentReasonFor(phase int16, err error) string {
	switch {
	case errors.Is(err, m2m.ErrNoSelectedCandidates):
		return jobReasonNoSelected
	case errors.Is(err, errDraftDelivery):
		return jobReasonDraftRejected
	case phase == 0 && isPDFError(err):
		return jobReasonUnreadablePDF
	default:
		return jobReasonPermanent
	}
}

// isPDFError dice si err es uno de los cuatro permanentes del extractor de PDF
// (dañado, escaneado, demasiado grande o vacío).
func isPDFError(err error) bool {
	return errors.Is(err, pdfErrors.ErrPDFCorrupt) ||
		errors.Is(err, pdfErrors.ErrPDFScanned) ||
		errors.Is(err, pdfErrors.ErrPDFTooLarge) ||
		errors.Is(err, pdfErrors.ErrPDFEmpty)
}

// composeJobReason une el motivo y el detalle técnico y aplica el tope de bytes.
func composeJobReason(reason string, detail error) string {
	if detail == nil {
		return truncateUTF8(reason, maxJobLastErrorBytes)
	}
	return truncateUTF8(reason+jobReasonSeparator+detail.Error(), maxJobLastErrorBytes)
}

// truncateUTF8 corta s a lo sumo en limit bytes sin partir una runa: un UTF-8 partido
// haría que learning rechazara el PATCH y el job se quedaría sin cerrar.
func truncateUTF8(s string, limit int) string {
	if len(s) <= limit {
		return s
	}
	cut := limit
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	return s[:cut]
}

// markJobFailed cierra el job como failed con su motivo. Best-effort: un fallo del
// PATCH se loguea y se ignora (un 409 es que el job ya estaba cerrado).
func (p *MaterialPipelineProcessor) markJobFailed(ctx context.Context, jobID string, phase int16, lastError string) {
	if err := p.pipeline.UpdateJobStatus(ctx, jobID, jobStatusFailed, phase, &lastError); err != nil {
		p.logger.Warn("no se pudo marcar el job como failed (best-effort, se ignora)",
			"job_id", jobID, "phase", phase, "error", err.Error())
		return
	}
	p.logger.Info("job marcado como failed", "job_id", jobID, "phase", phase, "last_error", lastError)
}

// closeJobPipelineOff cierra como failed el job de una escuela con el pipeline
// apagado: pasa de pending a failed sin pasar por processing, así el material no
// cambia (P5) y la docente ve «Falló» con el motivo en vez de «En cola» sin fin. Un
// 409 es que el job ya estaba cerrado (reentrega, barrido): se ignora y ACK. Otro
// error sube para que el consumer lo reintente o lo mande a la cola muerta.
func (p *MaterialPipelineProcessor) closeJobPipelineOff(ctx context.Context, jobID string) error {
	reason := jobReasonPipelineOff
	if err := p.pipeline.UpdateJobStatus(ctx, jobID, jobStatusFailed, 0, &reason); err != nil {
		if errors.Is(err, m2m.ErrPipelineConflict) {
			p.logger.Info("el job ya estaba cerrado al marcarlo por pipeline apagado (409), ACK", "job_id", jobID)
			return nil
		}
		return fmt.Errorf("cerrando el job %s por pipeline apagado: %w", jobID, err)
	}
	return nil
}

// failOnLastAttempt cierra el job como failed cuando un fallo de SERVICIO (transitorio)
// llega en el último reintento: el mensaje va a morir en la cola muerta y nadie más
// tocaría el job. Devuelve SIEMPRE el mismo error. No marca nada si el error es nil o
// permanente (eso ya lo hizo failIfPermanent), si quedan reintentos, si no hay datos de
// la entrega (fuera del consumer con cola muerta), ni si el contexto se canceló: un
// apagado ordenado del worker no es un fallo del servicio y el mensaje se reentrega.
func (p *MaterialPipelineProcessor) failOnLastAttempt(ctx context.Context, jobID string, err error) error {
	if err == nil || classifyError(err) == ErrorTypePermanent {
		return err
	}
	if ctx.Err() != nil {
		return err
	}
	info, ok := rabbit.DeliveryFromContext(ctx)
	if !ok || !info.IsLastAttempt() {
		return err
	}
	// Fase 0: learning conserva la fase alcanzada (nunca retrocede).
	p.markJobFailed(ctx, jobID, 0, composeJobReason(jobReasonServiceDown, err))
	return err
}

// startJobHeartbeat late el job cada heartbeatInterval mientras Process lo tiene: un
// PATCH processing con fase 0, que en learning solo refresca updated_at (la fase no
// retrocede y el material no se toca). Devuelve la función que lo para y ESPERA a que
// la gorrutina termine, de modo que ningún latido sale después de que Process
// devuelva. Un fallo del latido se loguea y no cambia el resultado del procesamiento.
func (p *MaterialPipelineProcessor) startJobHeartbeat(ctx context.Context, jobID string) (stop func()) {
	interval := p.heartbeatInterval
	if interval <= 0 {
		return func() {}
	}

	beatCtx, cancel := context.WithCancel(ctx)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-beatCtx.Done():
				return
			case <-ticker.C:
				if err := p.pipeline.UpdateJobStatus(beatCtx, jobID, jobStatusProcessing, 0, nil); err != nil && beatCtx.Err() == nil {
					p.logger.Warn("el latido del job falló (se ignora)", "job_id", jobID, "error", err.Error())
				}
			}
		}
	}()

	var once sync.Once
	return func() {
		once.Do(func() {
			cancel()
			wg.Wait()
		})
	}
}
