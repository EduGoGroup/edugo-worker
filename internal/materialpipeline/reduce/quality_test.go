package reduce

import (
	"context"
	"strconv"
	"testing"

	"github.com/EduGoGroup/edugo-worker/internal/client/m2m"
)

// manyOptions genera n opciones distintas.
func manyOptions(n int) []string {
	out := make([]string, n)
	for i := 0; i < n; i++ {
		out[i] = "op" + strconv.Itoa(i)
	}
	return out
}

// Una candidata válida sobrevive; cada regla principal del contrato import descarta a
// dropped_irrelevant. Un solo Run cubre todas las reglas.
func TestQuality_ValidaEInvalidaPorRegla(t *testing.T) {
	store := &fakeStore{records: []m2m.CandidateRecord{
		// válida (mc con 2 opciones y correcta presente)
		candRecord("ok", 0, "multiple_choice", "q ok", "op0", []string{"op0", "op1"}, []string{"i"}),
		// mc con 1 opción (< mínimo)
		candRecord("pocasOpc", 1, "multiple_choice", "q", "op0", []string{"op0"}, []string{"i"}),
		// correcta no presente en opciones
		candRecord("correctaFuera", 2, "multiple_choice", "q", "zzz", []string{"op0", "op1"}, []string{"i"}),
		// tipo desconocido
		candRecord("tipoMalo", 3, "trivia", "q", "op0", []string{"op0", "op1"}, []string{"i"}),
		// demasiadas opciones (> MaxOptionsPerQ = 10)
		candRecord("muchasOpc", 4, "multiple_choice", "q", "op0", manyOptions(11), []string{"i"}),
	}}
	// mc sin correct_answer (obligatorio salvo open_ended): candRecord con correct nil.
	store.records = append(store.records,
		candRecord("sinCorrecta", 5, "multiple_choice", "q", nil, []string{"op0", "op1"}, []string{"i"}))

	pass := NewQualityPass(store, &nopLogger{})
	rep, err := pass.Run(context.Background(), "job-1")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if rep.Processed != 6 {
		t.Fatalf("Processed = %d, quiero 6", rep.Processed)
	}
	if rep.Valid != 1 || rep.DroppedInvalid != 5 {
		t.Fatalf("Valid/DroppedInvalid = %d/%d, quiero 1/5", rep.Valid, rep.DroppedInvalid)
	}
	st := statusByID(store)
	if st["ok"] != statusCandidate {
		t.Fatalf("la candidata válida debe seguir en candidate, está en %q", st["ok"])
	}
	for _, id := range []string{"pocasOpc", "correctaFuera", "tipoMalo", "muchasOpc", "sinCorrecta"} {
		if st[id] != statusDroppedIrrelevant {
			t.Fatalf("%q debe caer a dropped_irrelevant, está en %q", id, st[id])
		}
	}
}

// Open_ended sin opciones ni correcta es válida (el contrato no las exige).
func TestQuality_OpenEndedValida(t *testing.T) {
	store := &fakeStore{records: []m2m.CandidateRecord{
		candRecord("oe", 0, "open_ended", "explica el proceso", nil, nil, []string{"i"}),
	}}
	pass := NewQualityPass(store, &nopLogger{})
	rep, err := pass.Run(context.Background(), "job-1")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if rep.Valid != 1 || rep.DroppedInvalid != 0 {
		t.Fatalf("open_ended válida: Valid/Dropped = %d/%d, quiero 1/0", rep.Valid, rep.DroppedInvalid)
	}
}

// Las terminales se saltan (idempotencia por status): no se re-evalúan ni cambian.
func TestQuality_SkipsTerminal(t *testing.T) {
	rec := candRecord("d", 0, "trivia", "q invalida", "x", nil, nil) // inválida, pero terminal
	rec.Status = statusDroppedDup
	store := &fakeStore{records: []m2m.CandidateRecord{rec}}

	pass := NewQualityPass(store, &nopLogger{})
	rep, err := pass.Run(context.Background(), "job-1")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if rep.Processed != 0 || rep.DroppedInvalid != 0 {
		t.Fatalf("una terminal no debe procesarse (processed=%d, dropped=%d)", rep.Processed, rep.DroppedInvalid)
	}
	if statusByID(store)["d"] != statusDroppedDup {
		t.Fatalf("la terminal no debe cambiar de estado")
	}
}
