package reduce

import (
	"context"
	"fmt"

	"github.com/EduGoGroup/edugo-shared/logger"
	"github.com/EduGoGroup/edugo-shared/textmatch"
	"github.com/EduGoGroup/edugo-worker/internal/client/m2m"
	"github.com/EduGoGroup/edugo-worker/internal/materialpipeline"
)

// statusSelected es el estado terminal de una candidata que entra al draft (pasada 4 de
// selección final, D-044.5). Es absorbente igual que los demás terminales (§4): una vez
// `selected`, un re-Run no la re-evalúa. Las NO seleccionadas QUEDAN en `candidate` como
// rastro (contrato §4: nada se borra).
const statusSelected = "selected"

// jobIdeasResolver lee las main_ideas agregadas del material del job (la cobertura objetivo
// de la selección). El *m2m.LearningPipelineClient lo satisface vía GetJobIdeas; en test, un
// fake determinista. Interfaz mínima (ISP): no arrastra todo el cliente M2M.
type jobIdeasResolver interface {
	GetJobIdeas(ctx context.Context, jobID string) ([]string, error)
}

// SelectionPass ejecuta la pasada 4 del reduce (selección final determinista, D-044.5): de
// las supervivientes rankeadas por `score` elige `target_questions` maximizando la COBERTURA
// de las main_ideas del job (greedy) y, como criterio secundario, un mix de tipos de
// pregunta. Aislada del processor (el cableado es F3c): recibe sus colaboradores por
// constructor tras interfaces mínimas, de modo que los tests la ejercen con fakes
// deterministas sin tocar learning.
type SelectionPass struct {
	store  candidateStore
	ideas  jobIdeasResolver
	logger logger.Logger
}

// NewSelectionPass construye la pasada.
func NewSelectionPass(store candidateStore, ideas jobIdeasResolver, log logger.Logger) *SelectionPass {
	return &SelectionPass{store: store, ideas: ideas, logger: log}
}

// SelectionReport resume lo que hizo la pasada sobre un job (logs/harness/observabilidad).
type SelectionReport struct {
	Selected        int            // candidatas marcadas `selected` (entran al draft)
	PorTipo         map[string]int // seleccionadas por question_type (el mix logrado)
	IdeasCubiertas  int            // main_ideas que alguna seleccionada cubre
	IdeasSinCubrir  []string       // main_ideas que NINGUNA seleccionada cubre (ADVERTENCIA)
	TargetQuestions int            // cupo pedido (target_questions del job)
	CandidatasVivas int            // candidatas status=candidate con score no-nil, parseables
	AlreadySelected bool           // true: el job ya tenía ≥target selected → no se re-seleccionó
	IdeasFromSource bool           // true: las main_ideas se cayeron al agregado de source_ideas
}

// Run corre la pasada 4 sobre un job: selecciona hasta `targetQuestions` candidatas vivas
// (status=candidate con score no-nil) maximizando la cobertura de las main_ideas del job y
// las marca `selected`; el resto queda en `candidate` como rastro. Idempotente: si el job ya
// tiene ≥target seleccionadas no re-selecciona (AlreadySelected). Nada se borra.
//
// DESVIACIÓN de firma (targetQuestions por parámetro): D-044.5 habla de `target_questions`
// «param del job», pero el DTO M2M PipelineJob (GetJob) NO lo expone hoy. Como acordó la
// tarea, la pasada lo recibe por parámetro (el caller/F3c lo obtiene de donde learning lo
// exponga) en vez de leerlo del job.
//
// Cobertura (D-044.5): greedy determinista. Insumo de ideas = GetJobIdeas; si viene vacío,
// se cae al AGREGADO de source_ideas de las candidatas (unión normalizada, mismo proxy que
// RelevancePass) con Warn y IdeasFromSource=true. Fase 1 — por cada idea AÚN sin cubrir, la
// mejor candidata (mayor score; empate → menor chunk_sequence, menor id) cuyas source_ideas
// la cubran (textmatch.SetMatcher). Al seleccionarla se marcan TODAS las ideas que cubre (no
// solo la que disparó la elección), para no gastar cupos de más. Fase 2 — con las ideas
// cubiertas (o sin candidatas que cubran alguna restante) y cupos libres, rellena por mix de
// tipos (prefiere el tipo menos representado entre las ya seleccionadas; empate → score, luego
// chunk_sequence, luego id). Las ideas sin cubrir son ADVERTENCIA (Warn + report), no error.
//
// Errores: propaga los del store/ideas tal cual (el caller los clasifica con la semántica del
// carril). Un error del comparador de textmatch se propaga.
func (s *SelectionPass) Run(ctx context.Context, jobID string, targetQuestions int) (SelectionReport, error) {
	records, err := s.store.ListCandidates(ctx, jobID)
	if err != nil {
		return SelectionReport{}, fmt.Errorf("listando candidatas del job %s: %w", jobID, err)
	}
	report := SelectionReport{TargetQuestions: targetQuestions, PorTipo: map[string]int{}}

	// Idempotencia: si ya hay ≥target seleccionadas, no se re-selecciona (los `selected`
	// son absorbentes; re-marcarlos sería un no-op o un 409). Se reporta lo ya hecho.
	alreadySelected := 0
	for i := range records {
		if records[i].Status == statusSelected {
			alreadySelected++
			if p, perr := materialpipeline.ValidateCandidatePayload(records[i].Payload); perr == nil && p != nil {
				report.PorTipo[p.QuestionType]++
			}
		}
	}

	// Candidatas vivas: status=candidate con score persistido (las que sobrevivieron a las
	// pasadas 1-3). Sin score no entran (la relevancia no las puntuó → conservador).
	live := make([]selCand, 0, len(records))
	for i := range records {
		rec := records[i]
		if rec.Status != statusCandidate || rec.Score == nil {
			continue
		}
		payload, perr := materialpipeline.ValidateCandidatePayload(rec.Payload)
		if perr != nil || payload == nil {
			// La calidad (pasada 3) ya descartó las no parseables; una que hoy no parsea es
			// una anomalía. No se puede leer su tipo/source_ideas, así que se omite (se deja
			// en candidate, no se borra ni selecciona).
			s.logger.Warn("candidata no parseable en selección, se omite",
				"job_id", jobID, "candidate_id", rec.ID)
			continue
		}
		live = append(live, selCand{record: rec, payload: *payload})
	}
	report.CandidatasVivas = len(live)

	if alreadySelected >= targetQuestions {
		report.AlreadySelected = true
		report.Selected = alreadySelected
		s.logger.Info("selección final: el job ya tenía ≥target seleccionadas, no se re-selecciona",
			"job_id", jobID, "ya_seleccionadas", alreadySelected, "target", targetQuestions)
		return report, nil
	}

	// Cobertura objetivo: las main_ideas del job. Vacías → agregado de source_ideas (proxy).
	ideas, err := s.ideas.GetJobIdeas(ctx, jobID)
	if err != nil {
		return report, fmt.Errorf("leyendo main_ideas del job %s: %w", jobID, err)
	}
	if len(ideas) == 0 {
		ideas = aggregateSourceIdeas(records)
		report.IdeasFromSource = true
		s.logger.Warn("selección: main_ideas del job vacías; se cae al agregado de source_ideas",
			"job_id", jobID, "ideas", len(ideas))
	}

	// SetMatcher con la misma escalera de letras del dedupe (Exact → Fuzzy 0.85) y política
	// lenient: una idea está cubierta si alguna source_ide de la candidata la matchea; las
	// source_ideas sobrantes no penalizan (solo interesa la cobertura de ESA idea).
	matcher := textmatch.NewSetMatcher(
		textmatch.NewCascade(textmatch.Exact{}, textmatch.NewFuzzy(fuzzyThreshold)),
		textmatch.PolicyLenient,
	)

	selected := make(map[int]bool, targetQuestions)
	covered := make([]bool, len(ideas))
	var order []int // orden de selección (determinista: dirige el lote de updates)

	record := func(i int) {
		selected[i] = true
		order = append(order, i)
		report.PorTipo[live[i].payload.QuestionType]++
	}

	// Fase 1 — cobertura. Se recorren las ideas en orden; para cada una aún sin cubrir se
	// elige la mejor candidata que la cubra y se marcan todas las ideas que esa candidata
	// cubre. Se corta al llenar el cupo.
	for e := range ideas {
		if len(selected) >= targetQuestions {
			break
		}
		if covered[e] {
			continue
		}
		best := -1
		for i := range live {
			if selected[i] {
				continue
			}
			ok, cerr := coversIdea(ctx, matcher, live[i].payload.SourceIdeas, ideas[e])
			if cerr != nil {
				return report, fmt.Errorf("cobertura de ideas en selección del job %s: %w", jobID, cerr)
			}
			if !ok {
				continue
			}
			if best == -1 || betterByScore(live[i], live[best]) {
				best = i
			}
		}
		if best == -1 {
			continue // idea sin candidata que la cubra: queda sin cubrir (advertencia)
		}
		record(best)
		if merr := markCoveredBy(ctx, matcher, live[best], ideas, covered); merr != nil {
			return report, fmt.Errorf("marcando ideas cubiertas en selección del job %s: %w", jobID, merr)
		}
	}

	// Fase 2 — relleno por mix de tipos. Con cupos libres, se rellena prefiriendo el tipo
	// menos representado entre las ya seleccionadas (empate → score, chunk_sequence, id).
	for len(selected) < targetQuestions {
		best := -1
		for i := range live {
			if selected[i] {
				continue
			}
			if best == -1 || betterForFill(live[i], live[best], report.PorTipo) {
				best = i
			}
		}
		if best == -1 {
			break // no quedan candidatas vivas (target > vivas): se seleccionaron todas
		}
		record(best)
	}

	// Resumen de cobertura.
	for e, c := range covered {
		if c {
			report.IdeasCubiertas++
		} else {
			report.IdeasSinCubrir = append(report.IdeasSinCubrir, ideas[e])
		}
	}

	updates := make([]m2m.CandidateUpdate, 0, len(order))
	for _, i := range order {
		sel := statusSelected
		updates = append(updates, m2m.CandidateUpdate{ID: live[i].record.ID, Status: &sel})
	}
	if len(updates) > 0 {
		if _, err := s.store.UpdateCandidates(ctx, updates); err != nil {
			return report, fmt.Errorf("persistiendo selección del job %s: %w", jobID, err)
		}
	}
	report.Selected = len(order)

	if len(report.IdeasSinCubrir) > 0 {
		s.logger.Warn("selección final: quedaron main_ideas sin cubrir (el profesor decide)",
			"job_id", jobID, "sin_cubrir", len(report.IdeasSinCubrir))
	}
	s.logger.Info("pasada 4 de selección final completa",
		"job_id", jobID,
		"vivas", report.CandidatasVivas,
		"seleccionadas", report.Selected,
		"target", targetQuestions,
		"ideas_cubiertas", report.IdeasCubiertas,
		"ideas_sin_cubrir", len(report.IdeasSinCubrir))
	return report, nil
}

// selCand es una candidata viva en proceso de selección: el registro crudo + su payload
// parseado. El score no-nil está garantizado por el filtro de Run.
type selCand struct {
	record  m2m.CandidateRecord
	payload materialpipeline.CandidatePayloadV1
}

// score devuelve el score de relevancia (garantizado no-nil por el filtro de Run; el guard
// es defensivo).
func (c selCand) score() float64 {
	if c.record.Score == nil {
		return 0
	}
	return *c.record.Score
}

// coversIdea reporta si las source_ideas de una candidata cubren `idea` (SetMatcher lenient:
// basta con que una source_ide matchee la idea; las sobrantes no penalizan). Sin source_ideas
// no hay cobertura. Un error del comparador se propaga.
func coversIdea(ctx context.Context, matcher *textmatch.SetMatcher, sourceIdeas []string, idea string) (bool, error) {
	if len(sourceIdeas) == 0 {
		return false, nil
	}
	rep, err := matcher.Match(ctx, []string{idea}, sourceIdeas)
	if err != nil {
		return false, err
	}
	return rep.Covered[0], nil
}

// markCoveredBy marca en `covered` todas las ideas aún sin cubrir que la candidata cubre (no
// solo la que disparó su elección), para no gastar un cupo por idea cuando una candidata
// cubre varias. Un error del comparador se propaga.
func markCoveredBy(ctx context.Context, matcher *textmatch.SetMatcher, cand selCand, ideas []string, covered []bool) error {
	for f := range ideas {
		if covered[f] {
			continue
		}
		ok, err := coversIdea(ctx, matcher, cand.payload.SourceIdeas, ideas[f])
		if err != nil {
			return err
		}
		if ok {
			covered[f] = true
		}
	}
	return nil
}

// betterByScore decide si a debe ganar sobre b como candidata de una idea: mayor score;
// empate → menor chunk_sequence; empate → menor id (determinista).
func betterByScore(a, b selCand) bool {
	sa, sb := a.score(), b.score()
	if sa != sb {
		return sa > sb
	}
	if a.record.ChunkSequence != b.record.ChunkSequence {
		return a.record.ChunkSequence < b.record.ChunkSequence
	}
	return a.record.ID < b.record.ID
}

// betterForFill decide si a debe ganar sobre b en el relleno (fase 2): criterio primario del
// relleno = tipo menos representado entre las ya seleccionadas (mix); empate → betterByScore.
func betterForFill(a, b selCand, porTipo map[string]int) bool {
	ca, cb := porTipo[a.payload.QuestionType], porTipo[b.payload.QuestionType]
	if ca != cb {
		return ca < cb
	}
	return betterByScore(a, b)
}
