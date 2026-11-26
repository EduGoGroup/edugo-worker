package processor

// Tests de material_pipeline_job_state.go (HU-IA.5): los motivos de last_error, el tope
// de bytes, la entrega sin seleccionadas, el cierre en el último reintento, tomar el job
// y el latido. Los dobles del carril viven en material_pipeline_processor_test.go.

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/EduGoGroup/edugo-shared/messaging/rabbit"
	"github.com/EduGoGroup/edugo-worker/internal/client/m2m"
	pdfErrors "github.com/EduGoGroup/edugo-worker/internal/infrastructure/pdf"
	"github.com/EduGoGroup/edugo-worker/internal/materialpipeline/reduce"
)

// --- los motivos ---

func TestJobReasons_TextsOfTheFicha(t *testing.T) {
	// Los textos son contrato con la docente (KMP los enseña tal cual): la tabla de la
	// ficha, literal.
	want := map[string]string{
		jobReasonPipelineOff:   "La generación con IA está apagada en esta escuela.",
		jobReasonUnreadablePDF: "No se pudo leer el PDF.",
		jobReasonNoSelected:    "Ninguna pregunta generada pasó los filtros: no hay borrador.",
		jobReasonDraftRejected: "El borrador no se pudo guardar.",
		jobReasonPermanent:     "No se pudo generar la evaluación.",
		jobReasonServiceDown:   "El servicio de IA no respondió tras varios intentos.",
	}
	for got, w := range want {
		if got != w {
			t.Errorf("motivo %q, la ficha dice %q", got, w)
		}
	}
	if jobReasonSeparator != " · " || maxJobLastErrorBytes != 1024 {
		t.Errorf("separador %q y tope %d (la ficha: « · » y 1024)", jobReasonSeparator, maxJobLastErrorBytes)
	}
}

func TestJobFailureReason_ByCause(t *testing.T) {
	noSelected := fmt.Errorf("%w: job job-1 (422)", m2m.ErrNoSelectedCandidates)
	rejected := fmt.Errorf("%w: pipeline status 422: import rechazado", m2m.ErrLearningPermanent)
	cases := []struct {
		name  string
		phase int16
		err   error
		want  string
	}{
		// runPhase2 envuelve la entrega con errDraftDelivery: «sin seleccionadas» gana.
		{"sin seleccionadas en la entrega", 2, fmt.Errorf("reduce: entrega del job job-1: %w: %w", errDraftDelivery, noSelected), jobReasonNoSelected},
		{"sin seleccionadas sin envolver", 2, noSelected, jobReasonNoSelected},
		{"import rechazado", 2, fmt.Errorf("reduce: entrega del job job-1: %w: %w", errDraftDelivery, rejected), jobReasonDraftRejected},
		{"PDF dañado", 0, fmt.Errorf("fase 0: %w", pdfErrors.ErrPDFCorrupt), jobReasonUnreadablePDF},
		{"PDF escaneado", 0, fmt.Errorf("fase 0: %w", pdfErrors.ErrPDFScanned), jobReasonUnreadablePDF},
		{"PDF demasiado grande", 0, pdfErrors.ErrPDFTooLarge, jobReasonUnreadablePDF},
		{"descarga que pasa del tope de tamaño", 0, m2m.ErrFileTooLarge, jobReasonUnreadablePDF},
		{"PDF vacío", 0, pdfErrors.ErrPDFEmpty, jobReasonUnreadablePDF},
		{"error de PDF fuera de la fase 0", 1, pdfErrors.ErrPDFScanned, jobReasonPermanent},
		{"otro permanente de learning", 1, fmt.Errorf("leyendo el siguiente chunk: %w", m2m.ErrLearningPermanent), jobReasonPermanent},
		{"evento malformado", 0, ErrMalformedEvent, jobReasonPermanent},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := permanentReasonFor(tc.phase, tc.err); got != tc.want {
				t.Fatalf("permanentReasonFor = %q, se esperaba %q", got, tc.want)
			}
			if got, want := jobFailureReason(tc.phase, tc.err), tc.want+" · "+tc.err.Error(); got != want {
				t.Fatalf("jobFailureReason = %q, se esperaba %q (motivo, separador y detalle)", got, want)
			}
		})
	}
}

func TestTruncateUTF8_RuneBoundary(t *testing.T) {
	if got := truncateUTF8("corto", 1024); got != "corto" {
		t.Fatalf("lo que cabe no se toca: %q", got)
	}
	// «á» ocupa 2 bytes: con una «a» delante el byte 1024 cae a mitad de runa.
	odd := "a" + strings.Repeat("á", 600)
	if got := truncateUTF8(odd, 1024); len(got) != 1023 || !utf8.ValidString(got) {
		t.Fatalf("corte a mitad de runa: %d bytes, válido=%v (se esperan 1023, válido)", len(got), utf8.ValidString(got))
	}
	// Mezcla de runas de 1 a 4 bytes, todos los topes pequeños: siempre un prefijo
	// válido, ≤ tope, al que no le falta más de una runa.
	mixed := strings.Repeat("aé€😀", 5)
	for limit := 0; limit <= len(mixed)+1; limit++ {
		got := truncateUTF8(mixed, limit)
		if len(got) > limit || !utf8.ValidString(got) || !strings.HasPrefix(mixed, got) || len(got) < min(limit, len(mixed))-3 {
			t.Fatalf("tope %d: %q (%d bytes)", limit, got, len(got))
		}
	}
	// El last_error completo respeta el tope aunque el detalle sea enorme.
	reason := jobFailureReason(1, fmt.Errorf("%w: %s", m2m.ErrLearningPermanent, strings.Repeat("ñ€", 1000)))
	if len(reason) > 1024 || !utf8.ValidString(reason) || !strings.HasPrefix(reason, jobReasonPermanent+" · ") {
		t.Fatalf("last_error de %d bytes, válido=%v, empieza por el motivo=%v", len(reason), utf8.ValidString(reason), strings.HasPrefix(reason, jobReasonPermanent))
	}
}

// --- la entrega y tomar el job ---

func TestMaterialProcess_NoSelected_FailsWithReason(t *testing.T) {
	// Deuda 171: el 422 «sin seleccionadas» deja el job failed/2 de verdad, con el motivo
	// primero; ningún done.
	pipe := &mockMaterialPipeline{job: phase1Job(), deliverErr: fmt.Errorf("%w: job job-1 (422)", m2m.ErrNoSelectedCandidates)}
	err := newMaterialProcessor(onSettings(), pipe, &mockMaterialProvider{}).Process(context.Background(), materialEventJSON("job-1", "mat-1", "school-1"))
	if err == nil || classifyError(err) != ErrorTypePermanent || !errors.Is(err, m2m.ErrNoSelectedCandidates) {
		t.Fatalf("sin seleccionadas debe subir permanente y tipado, got %v", err)
	}
	if len(pipe.updateCalls) == 0 {
		t.Fatal("no hubo ningún PATCH")
	}
	last := pipe.updateCalls[len(pipe.updateCalls)-1]
	if last.status != jobStatusFailed || last.phase != 2 || !strings.HasPrefix(deref(last.lastError), "Ninguna pregunta generada pasó los filtros: no hay borrador. · ") {
		t.Fatalf("se esperaba failed/2 con el motivo de la deuda 171, got %+v (%q)", last, deref(last.lastError))
	}
	for _, u := range pipe.updateCalls {
		if u.status == jobStatusDone {
			t.Fatalf("ningún PATCH done: %+v", pipe.updateCalls)
		}
	}
}

func TestMaterialProcess_TakeJob(t *testing.T) {
	// 409 al tomar el job: se cerró entre la lectura y el PATCH → ACK sin trabajo.
	pipe := &mockMaterialPipeline{job: processingJob(), pending: []*m2m.NextChunk{pendingChunk("c1")}, updateErr: m2m.ErrPipelineConflict}
	if err := newMaterialProcessor(onSettings(), pipe, &mockMaterialProvider{}).Process(context.Background(), materialEventJSON("job-1", "mat-1", "school-1")); err != nil {
		t.Fatalf("un 409 al tomar el job es ACK, got %v", err)
	}
	if len(pipe.updateCalls) != 1 || pipe.updateCalls[0].status != jobStatusProcessing || pipe.nextIdx != 0 || pipe.deliverCalls != 0 {
		t.Fatalf("tras el 409 no se procesa nada: PATCH %+v, trozos %d, entregas %d", pipe.updateCalls, pipe.nextIdx, pipe.deliverCalls)
	}

	// Un transitorio al tomarlo sube transitorio y no marca failed.
	pipe = &mockMaterialPipeline{job: processingJob(), updateErr: errors.New("learning 503")}
	err := newMaterialProcessor(onSettings(), pipe, &mockMaterialProvider{}).Process(context.Background(), materialEventJSON("job-1", "mat-1", "school-1"))
	if err == nil || classifyError(err) != ErrorTypeTransient || len(pipe.updateCalls) != 1 || pipe.nextIdx != 0 {
		t.Fatalf("transitorio al tomar el job: err %v, PATCH %+v", err, pipe.updateCalls)
	}
}

func TestMaterialProcess_PipelineOff_OtherErrorPropagates(t *testing.T) {
	// Con el pipeline apagado, un error del PATCH failed que no es 409 sube (el consumer
	// reintenta) y no se manda nada más.
	pipe := &mockMaterialPipeline{job: processingJob(), updateErr: errors.New("learning 503")}
	settings := &mockSettingsReader{settings: settingsWith(settingKeyPipelineMode, pipelineModeOff)}
	err := newMaterialProcessor(settings, pipe, &mockMaterialProvider{}).Process(context.Background(), materialEventJSON("job-1", "mat-1", "school-1"))
	if err == nil || classifyError(err) != ErrorTypeTransient {
		t.Fatalf("un 503 al cerrar el job apagado debe subir transitorio, got %v", err)
	}
	if len(pipe.updateCalls) != 1 || pipe.updateCalls[0].status != jobStatusFailed || pipe.getJobCalls != 0 {
		t.Fatalf("un solo PATCH failed y nada más: %+v (GetJob %d)", pipe.updateCalls, pipe.getJobCalls)
	}
}

// --- el último reintento ---

// deliveryCtx es un contexto con los datos de la entrega que ConsumeWithDLQ deja al
// handler. rabbit no exporta cómo meterlos (withDeliveryInfo y su clave son privados),
// así que el doble responde a la clave por su tipo. Si shared renombra la clave,
// TestDeliveryCtx_IsReadByRabbit falla primero y dice por qué.
type deliveryCtx struct {
	context.Context
	info rabbit.DeliveryInfo
}

func (c deliveryCtx) Value(key any) any {
	if tp := reflect.TypeOf(key); tp != nil && tp.PkgPath() == "github.com/EduGoGroup/edugo-shared/messaging/rabbit" && tp.Name() == "deliveryInfoKey" {
		return c.info
	}
	return c.Context.Value(key)
}

func lastAttemptInfo() rabbit.DeliveryInfo {
	return rabbit.DeliveryInfo{Queue: "material", Retry: 3, MaxRetries: 3, DeadLetterEnabled: true}
}

func TestDeliveryCtx_IsReadByRabbit(t *testing.T) {
	info, ok := rabbit.DeliveryFromContext(deliveryCtx{Context: context.Background(), info: lastAttemptInfo()})
	if !ok || !info.IsLastAttempt() {
		t.Fatalf("rabbit.DeliveryFromContext no lee el doble (ok=%v, %+v): ¿cambió la clave privada de rabbit?", ok, info)
	}
}

func TestMaterialProcess_LastAttemptTransient_MarksFailed(t *testing.T) {
	// Un transitorio del modelo (la fase 1 sube el error intacto).
	run := func(ctx context.Context) (*mockMaterialPipeline, error) {
		pipe := &mockMaterialPipeline{job: processingJob(), pending: []*m2m.NextChunk{pendingChunk("c1")}}
		prov := &mockMaterialProvider{digestErr: errors.New("ollama request failed: connection refused")}
		return pipe, newMaterialProcessor(onSettings(), pipe, prov).Process(ctx, materialEventJSON("job-1", "mat-1", "school-1"))
	}
	failedOf := func(pipe *mockMaterialPipeline) []updateRecord {
		var out []updateRecord
		for _, u := range pipe.updateCalls {
			if u.status == jobStatusFailed {
				out = append(out, u)
			}
		}
		return out
	}
	_, plainErr := run(context.Background())

	t.Run("último intento: marca failed con el texto de servicio y devuelve el mismo error", func(t *testing.T) {
		pipe, err := run(deliveryCtx{Context: context.Background(), info: lastAttemptInfo()})
		if err == nil || classifyError(err) != ErrorTypeTransient || err.Error() != plainErr.Error() {
			t.Fatalf("el error debe ser el mismo transitorio (%v), got %v", plainErr, err)
		}
		failed := failedOf(pipe)
		if len(failed) != 1 {
			t.Fatalf("se esperaba un único PATCH failed, got %+v", pipe.updateCalls)
		}
		if failed[0].phase != 0 || deref(failed[0].lastError) != "El servicio de IA no respondió tras varios intentos. · "+plainErr.Error() {
			t.Fatalf("se esperaba failed/0 con el motivo de servicio, got %+v (%q)", failed[0], deref(failed[0].lastError))
		}
	})

	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	noMark := []struct {
		name string
		ctx  context.Context
	}{
		{"reintento intermedio", deliveryCtx{Context: context.Background(), info: rabbit.DeliveryInfo{Retry: 1, MaxRetries: 3, DeadLetterEnabled: true}}},
		{"sin cola muerta", deliveryCtx{Context: context.Background(), info: rabbit.DeliveryInfo{Retry: 3, MaxRetries: 3}}},
		{"sin datos de la entrega", context.Background()},
		{"contexto cancelado por el apagado", deliveryCtx{Context: cancelled, info: lastAttemptInfo()}},
	}
	for _, tc := range noMark {
		t.Run(tc.name+": no marca", func(t *testing.T) {
			pipe, err := run(tc.ctx)
			if err == nil || classifyError(err) != ErrorTypeTransient {
				t.Fatalf("el transitorio sube igual, got %v", err)
			}
			if failed := failedOf(pipe); len(failed) != 0 {
				t.Fatalf("no debe marcar failed, got %+v", failed)
			}
		})
	}

	t.Run("permanente en el último intento: un solo failed, el de su motivo", func(t *testing.T) {
		pipe := &mockMaterialPipeline{job: processingJob(), pendingErr: m2m.ErrLearningPermanent}
		err := newMaterialProcessor(onSettings(), pipe, &mockMaterialProvider{}).Process(deliveryCtx{Context: context.Background(), info: lastAttemptInfo()}, materialEventJSON("job-1", "mat-1", "school-1"))
		failed := failedOf(pipe)
		if classifyError(err) != ErrorTypePermanent || len(failed) != 1 || !strings.HasPrefix(deref(failed[0].lastError), "No se pudo generar la evaluación. · ") {
			t.Fatalf("permanente: err %v, failed %+v", err, failed)
		}
	})
}

// --- el latido ---

// beatingPipeline envuelve el doble del carril con un mutex para UpdateJobStatus, que
// el latido llama desde su gorrutina (el gate corre con -race). Cuenta como latido cada
// PATCH processing/0 (el job entra en processing/1, así que tomarlo es processing/1),
// avisa por beats y le devuelve beatErr.
type beatingPipeline struct {
	*mockMaterialPipeline
	mu      sync.Mutex
	beatErr error
	beats   chan struct{}
	nBeats  int
}

func (b *beatingPipeline) UpdateJobStatus(ctx context.Context, jobID, status string, phase int16, lastError *string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if status == jobStatusProcessing && phase == 0 {
		b.nBeats++
		b.updateCalls = append(b.updateCalls, updateRecord{status: status, phase: phase})
		select {
		case b.beats <- struct{}{}:
		default:
		}
		return b.beatErr
	}
	return b.mockMaterialPipeline.UpdateJobStatus(ctx, jobID, status, phase, lastError)
}

func (b *beatingPipeline) snapshot() (int, []updateRecord) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.nBeats, append([]updateRecord(nil), b.updateCalls...)
}

// blockingDedupe retiene la fase 2 hasta que el test lo suelta, para que Process dure
// varios latidos.
type blockingDedupe struct{ release chan struct{} }

func (d *blockingDedupe) Run(ctx context.Context, _ string) (reduce.DedupeReport, error) {
	select {
	case <-d.release:
		return reduce.DedupeReport{}, nil
	case <-ctx.Done():
		return reduce.DedupeReport{}, ctx.Err()
	case <-time.After(5 * time.Second):
		return reduce.DedupeReport{}, errors.New("blockingDedupe: nadie lo soltó")
	}
}

func TestJobHeartbeat_BeatsAndStops(t *testing.T) {
	const interval = 5 * time.Millisecond
	for _, tc := range []struct {
		name    string
		beatErr error
	}{
		{"latidos que pasan", nil},
		{"latidos que fallan", errors.New("learning 503")},
		{"latidos en conflicto", m2m.ErrPipelineConflict},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pipe := &beatingPipeline{
				mockMaterialPipeline: &mockMaterialPipeline{job: phase1Job(), deliverAssessmentID: "assess-1", deliverQuestions: 3},
				beatErr:              tc.beatErr,
				beats:                make(chan struct{}, 64),
			}
			deps := defaultReduceDeps()
			release := make(chan struct{})
			deps.Dedupe = &blockingDedupe{release: release}
			p := newMaterialProcessorWithReduce(onSettings(), pipe, &mockMaterialProvider{}, deps)
			p.heartbeatInterval = interval

			result := make(chan error, 1)
			go func() { result <- p.Process(context.Background(), materialEventJSON("job-1", "mat-1", "school-1")) }()

			// Late mientras Process tiene el job: tres latidos con la fase 2 retenida.
			for i := 0; i < 3; i++ {
				select {
				case <-pipe.beats:
				case <-time.After(2 * time.Second):
					t.Fatalf("solo hubo %d latidos mientras Process corría", i)
				}
			}
			close(release)
			var err error
			select {
			case err = <-result:
			case <-time.After(5 * time.Second):
				t.Fatal("Process no terminó tras soltar la fase 2")
			}
			// Un fallo del latido no cambia el resultado: se entrega y ACK.
			if err != nil || pipe.deliverCalls != 1 {
				t.Fatalf("el latido no cambia el resultado: err %v, entregas %d", err, pipe.deliverCalls)
			}

			// Para al terminar: ningún PATCH después de que Process devuelve.
			beats, calls := pipe.snapshot()
			time.Sleep(10 * interval)
			if beatsAfter, callsAfter := pipe.snapshot(); beatsAfter != beats || len(callsAfter) != len(calls) {
				t.Fatalf("hubo PATCH después de que Process devolviera: %d → %d latidos", beats, beatsAfter)
			}
			if calls[0].status != jobStatusProcessing || calls[0].phase != 1 {
				t.Fatalf("el primer PATCH es tomar el job (processing/1), got %+v", calls[0])
			}
			for _, u := range calls {
				if u.status != jobStatusProcessing {
					t.Fatalf("mientras late solo hay PATCH processing, got %+v", calls)
				}
			}
		})
	}
}

func TestJobHeartbeat_NoBeatWithoutInterval(t *testing.T) {
	// Con el intervalo apagado no arranca nada y parar es inocuo (dos veces también).
	p := newMaterialProcessor(onSettings(), &mockMaterialPipeline{}, &mockMaterialProvider{})
	if p.heartbeatInterval != time.Minute {
		t.Fatalf("el constructor deja el latido en 1 min, got %v", p.heartbeatInterval)
	}
	pipe := &beatingPipeline{mockMaterialPipeline: &mockMaterialPipeline{}, beats: make(chan struct{}, 1)}
	p = newMaterialProcessor(onSettings(), pipe, &mockMaterialProvider{})
	p.heartbeatInterval = 0
	stop := p.startJobHeartbeat(context.Background(), "job-1")
	time.Sleep(10 * time.Millisecond)
	stop()
	stop()
	if beats, _ := pipe.snapshot(); beats != 0 {
		t.Fatalf("sin intervalo no hay latidos, got %d", beats)
	}
}
