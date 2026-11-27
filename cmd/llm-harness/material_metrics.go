package main

// material_metrics.go — las cuentas del modo material (AR-IA.3, §3 de la ficha). Todo
// aquí es determinista y sin modelo: toma lo que dejó la corrida (el almacén, las
// sondas y los informes de las pasadas) y lo cuenta. Ninguna cuenta lleva umbral ni
// veredicto: el arnés mide, no juzga (ADR 0048).

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"unicode"

	"github.com/EduGoGroup/edugo-worker/internal/materialpipeline"
	"github.com/EduGoGroup/edugo-worker/internal/materialpipeline/reduce"
)

// Indicio de si la respuesta correcta está en el trozo de la candidata (decisión 3).
const (
	supportFull          = "full"           // todas las palabras con contenido están en el trozo
	supportPartial       = "partial"        // al menos la mitad
	supportNone          = "none"           // menos de la mitad
	supportNotApplicable = "not_applicable" // verdadero o falso, abierta, o respuesta sin palabras con contenido
)

// stopwords son las palabras vacías que no cuentan como contenido de una respuesta.
var stopwords = map[string]struct{}{
	"el": {}, "la": {}, "los": {}, "las": {}, "un": {}, "una": {}, "unos": {}, "unas": {},
	"de": {}, "del": {}, "al": {}, "a": {}, "en": {}, "y": {}, "o": {}, "u": {}, "e": {},
	"que": {}, "se": {}, "su": {}, "sus": {}, "por": {}, "para": {}, "con": {}, "sin": {},
	"es": {}, "son": {}, "fue": {}, "era": {}, "como": {}, "lo": {}, "le": {}, "les": {},
	"mas": {}, "no": {}, "si": {},
}

var accentFolder = strings.NewReplacer(
	"á", "a", "é", "e", "í", "i", "ó", "o", "ú", "u", "ü", "u",
	"ﬁ", "fi", "ﬂ", "fl",
)

// normalizedWords pasa un texto a minúsculas, sin tildes ni signos, y lo parte en
// palabras.
func normalizedWords(text string) []string {
	folded := accentFolder.Replace(strings.ToLower(text))
	return strings.FieldsFunc(folded, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
}

// contentWords son las palabras de un texto que no son vacías.
func contentWords(text string) []string {
	var out []string
	for _, w := range normalizedWords(text) {
		if _, stop := stopwords[w]; !stop {
			out = append(out, w)
		}
	}
	return out
}

// supportOf dice cuánto de una respuesta está en el conjunto de palabras del trozo.
func supportOf(answer string, chunkWords map[string]struct{}) string {
	words := contentWords(answer)
	if len(words) == 0 {
		return supportNotApplicable
	}
	found := 0
	for _, w := range words {
		if _, ok := chunkWords[w]; ok {
			found++
		}
	}
	switch {
	case found == len(words):
		return supportFull
	case found*2 >= len(words):
		return supportPartial
	default:
		return supportNone
	}
}

// answerInChunk devuelve un indicio por unidad de respuesta: uno en opción múltiple
// y en la corta, uno por opción correcta en selección múltiple, y «no aplica» en
// verdadero o falso y en la abierta. Es un indicio, no un juicio: un «none» puede
// ser una paráfrasis.
func answerInChunk(c materialpipeline.CandidatePayloadV1, chunkText string) []string {
	chunkWords := map[string]struct{}{}
	for _, w := range normalizedWords(chunkText) {
		chunkWords[w] = struct{}{}
	}
	switch c.QuestionType {
	case "multiple_choice", "short_answer":
		var answer string
		if err := json.Unmarshal(c.CorrectAnswer, &answer); err != nil {
			return []string{supportNotApplicable}
		}
		return []string{supportOf(answer, chunkWords)}
	case "multiple_select":
		var answers []string
		if err := json.Unmarshal(c.CorrectAnswer, &answers); err != nil || len(answers) == 0 {
			return []string{supportNotApplicable}
		}
		out := make([]string, 0, len(answers))
		for _, a := range answers {
			out = append(out, supportOf(a, chunkWords))
		}
		return out
	default:
		return []string{supportNotApplicable}
	}
}

// normalizeIdea deja una idea comparable: minúsculas, sin tildes ni signos.
func normalizeIdea(s string) string { return strings.Join(normalizedWords(s), " ") }

// sourceIdeasGiven cuenta cuántas source_ideas de una candidata son, de verdad,
// ideas que se le dieron al modelo (las principales y secundarias de su trozo).
func sourceIdeasGiven(sourceIdeas, givenIdeas []string) (total, given int) {
	set := make(map[string]struct{}, len(givenIdeas))
	for _, g := range givenIdeas {
		set[normalizeIdea(g)] = struct{}{}
	}
	for _, s := range sourceIdeas {
		total++
		if _, ok := set[normalizeIdea(s)]; ok {
			given++
		}
	}
	return total, given
}

// foldText colapsa los espacios y deshace las ligaduras del PDF, para buscar un
// título de sección en el texto extraído.
func foldText(s string) string {
	return strings.NewReplacer("ﬁ", "fi", "ﬂ", "fl").Replace(strings.Join(strings.Fields(s), " "))
}

// sectionOffsets busca cada título, en orden, en el texto extraído y devuelve dónde
// empieza. Si uno no aparece devuelve error: el arnés no puede medir la cobertura.
func sectionOffsets(text string, titles []string) ([]int, error) {
	folded := foldText(text)
	offsets := make([]int, 0, len(titles))
	from := 0
	for _, title := range titles {
		idx := strings.Index(folded[from:], foldText(title))
		if idx < 0 {
			return nil, fmt.Errorf("la sección %q no aparece en el texto extraído (o no en ese orden)", title)
		}
		offsets = append(offsets, from+idx)
		from += idx + 1
	}
	return offsets, nil
}

// chunkOffsets devuelve dónde empieza cada trozo en el texto extraído (-1 si no se
// encuentra). Los trozos van en orden y no se solapan.
func chunkOffsets(text string, chunks []string) []int {
	folded := foldText(text)
	out := make([]int, len(chunks))
	from := 0
	for i, c := range chunks {
		head := []rune(foldText(c))
		if len(head) > 60 {
			head = head[:60]
		}
		idx := strings.Index(folded[from:], string(head))
		if idx < 0 {
			out[i] = -1
			continue
		}
		out[i] = from + idx
		from += idx + 1
	}
	return out
}

// sectionOfOffset dice a qué sección pertenece lo que empieza en offset: la última
// cuyo título queda antes o en ese punto. Lo que empieza antes del primer título (el
// título del documento) cuenta en la primera sección. -1 si no hay secciones o el
// trozo no se encontró.
func sectionOfOffset(offset int, sections []int) int {
	if len(sections) == 0 || offset < 0 {
		return -1
	}
	idx := 0
	for i, start := range sections {
		if start <= offset {
			idx = i
		}
	}
	return idx
}

// distribution resume un reparto de enteros: mínimo, mediana, máximo y el reparto
// entero (cuántos trozos dieron cada cantidad).
type distribution struct {
	Values    []int       `json:"values"`
	Min       int         `json:"min"`
	Median    float64     `json:"median"`
	Max       int         `json:"max"`
	Histogram map[int]int `json:"histogram"`
}

func newDistribution(values []int) distribution {
	d := distribution{Values: values, Histogram: map[int]int{}}
	if len(values) == 0 {
		return d
	}
	sorted := append([]int{}, values...)
	sort.Ints(sorted)
	d.Min, d.Max = sorted[0], sorted[len(sorted)-1]
	mid := len(sorted) / 2
	if len(sorted)%2 == 1 {
		d.Median = float64(sorted[mid])
	} else {
		d.Median = float64(sorted[mid-1]+sorted[mid]) / 2
	}
	for _, v := range values {
		d.Histogram[v]++
	}
	return d
}

// chunkRun es un trozo de la corrida, con lo que el modelo dijo de él.
type chunkRun struct {
	Seq            int            `json:"seq"`
	Words          int            `json:"words"`
	Section        int            `json:"section"`
	Status         string         `json:"status"`
	FailReason     string         `json:"fail_reason,omitempty"`
	SummaryWords   int            `json:"summary_words"`
	Topic          string         `json:"chunk_topic,omitempty"`
	MainIdeas      []string       `json:"main_ideas"`
	SecondaryIdeas []string       `json:"secondary_ideas,omitempty"`
	Proposed       int            `json:"proposed"`
	ProposedTypes  map[string]int `json:"proposed_types"`
	ProposeError   string         `json:"propose_error_kind,omitempty"`
}

// candidateRun es una candidata guardada, entera, con su estado final.
type candidateRun struct {
	ID            string          `json:"id"`
	ChunkSeq      int             `json:"chunk_seq"`
	Section       int             `json:"section"`
	Status        string          `json:"status"`
	Score         *float64        `json:"score"`
	DedupeGroup   *string         `json:"dedupe_group"`
	Payload       json.RawMessage `json:"payload"`
	AnswerInChunk []string        `json:"answer_in_chunk"`
	SourceIdeas   int             `json:"source_ideas"`
	SourceGiven   int             `json:"source_ideas_given"`
}

// discardedRun es una candidata que el modelo propuso y el contrato descartó.
type discardedRun struct {
	ChunkSeq int      `json:"chunk_seq"`
	Type     string   `json:"question_type"`
	Issues   []string `json:"issues"`
}

// selectedQuestion es una pregunta del borrador, para leerla.
type selectedQuestion struct {
	ChunkSeq int    `json:"chunk_seq"`
	Section  int    `json:"section"`
	Type     string `json:"question_type"`
	Text     string `json:"question_text"`
}

// caseMetrics son los cinco bloques de cuentas de un caso.
type caseMetrics struct {
	Generation struct {
		Chunks                 int            `json:"chunks"`
		ChunksFailed           int            `json:"chunks_failed"`
		ProposedPerChunk       distribution   `json:"proposed_per_chunk"`
		ProposedTotal          int            `json:"proposed_total"`
		ProposedTypes          map[string]int `json:"proposed_types"`
		ChunksWithoutOpenEnded int            `json:"chunks_without_open_ended"`
		DiscardedByContract    []discardedRun `json:"discarded_by_contract"`
		DeicticStatements      int            `json:"deictic_statements"`
	} `json:"generation"`
	Support struct {
		// ByType: por tipo de pregunta, cuántas unidades de respuesta hay en cada indicio.
		ByType            map[string]map[string]int `json:"answer_in_chunk_by_type"`
		SourceIdeas       int                       `json:"source_ideas"`
		SourceIdeasGiven  int                       `json:"source_ideas_given"`
		CandidatesAllTrue int                       `json:"candidates_all_source_ideas_given"`
		Candidates        int                       `json:"candidates"`
	} `json:"support"`
	Reduction struct {
		DedupeRan         bool           `json:"dedupe_ran"`
		PairsText         int            `json:"pairs_text"`
		PairsEmbed        int            `json:"pairs_embed"`
		PairsLLM          int            `json:"pairs_llm"`
		Clusters          int            `json:"clusters"`
		DroppedDup        int            `json:"dropped_dup"`
		RelevanceRan      bool           `json:"relevance_ran"`
		Scored            int            `json:"scored"`
		DroppedIrrelevant int            `json:"dropped_irrelevant"`
		Unscored          int            `json:"unscored"`
		QualityRan        bool           `json:"quality_ran"`
		DroppedInvalid    int            `json:"dropped_invalid"`
		FinalStatus       map[string]int `json:"final_status"`
	} `json:"reduction"`
	Draft struct {
		Selected             int                `json:"selected"`
		Target               int                `json:"target"`
		SelectedTypes        map[string]int     `json:"selected_types"`
		SelectionRan         bool               `json:"selection_ran"`
		IdeasCovered         int                `json:"ideas_covered"`
		IdeasUncovered       int                `json:"ideas_uncovered"`
		Sections             int                `json:"sections"`
		SectionsWithSelected int                `json:"sections_with_selected"`
		Questions            []selectedQuestion `json:"selected_questions"`
	} `json:"draft"`
	Cost struct {
		Digest       callStats  `json:"digest"`
		Propose      callStats  `json:"propose"`
		Judge        callStats  `json:"judge"`
		Relevance    callStats  `json:"relevance"`
		Embed        embedStats `json:"embed"`
		Phase0MS     int64      `json:"phase0_ms"`
		Phase1MS     int64      `json:"phase1_ms"`
		DedupeMS     int64      `json:"dedupe_ms"`
		RelevanceMS  int64      `json:"relevance_ms"`
		QualityMS    int64      `json:"quality_ms"`
		SelectionMS  int64      `json:"selection_ms"`
		TotalMS      int64      `json:"total_ms"`
		Redeliveries int        `json:"redeliveries"`
	} `json:"cost"`
}

// caseDetail es lo que las cuentas devuelven además de los conteos: los trozos y
// las candidatas enteras, para que el «después» pueda decir qué pregunta cambió.
type caseDetail struct {
	Chunks     []chunkRun
	Candidates []candidateRun
}

// computeCaseMetrics cuenta una corrida. text es el texto extraído del PDF y
// sections dónde empieza cada sección declarada (vacío si el caso no declara).
func computeCaseMetrics(res *railResult, text string, sections []int, target int) (caseMetrics, caseDetail) {
	var m caseMetrics
	var d caseDetail
	store := res.store

	texts := make([]string, len(store.chunks))
	for i, c := range store.chunks {
		texts[i] = c.text
	}
	offsets := chunkOffsets(text, texts)
	finals := res.model.finalProposals()

	m.Generation.ProposedTypes = map[string]int{}
	m.Generation.DiscardedByContract = []discardedRun{}
	perChunk := make([]int, 0, len(store.chunks))
	chunkSection := map[int]int{}
	chunkText := map[int]string{}
	chunkIdeas := map[int][]string{}
	for i, c := range store.chunks {
		cr := chunkRun{
			Seq:           c.seq,
			Words:         len(strings.Fields(c.text)),
			Section:       sectionOfOffset(offsets[i], sections),
			Status:        c.status,
			FailReason:    c.failReason,
			MainIdeas:     []string{},
			ProposedTypes: map[string]int{},
		}
		chunkSection[c.seq] = cr.Section
		chunkText[c.seq] = c.text
		if c.summary != nil {
			cr.SummaryWords = len(strings.Fields(*c.summary))
		}
		var art materialpipeline.ChunkArtifactsV1
		if len(c.artifacts) > 0 && json.Unmarshal(c.artifacts, &art) == nil {
			cr.Topic = art.ChunkTopic
			cr.MainIdeas = append(cr.MainIdeas, art.MainIdeas...)
			cr.SecondaryIdeas = art.SecondaryIdeas
			chunkIdeas[c.seq] = append(append([]string{}, art.MainIdeas...), art.SecondaryIdeas...)
		}
		if c.status == storeStatusFailed {
			m.Generation.ChunksFailed++
		}
		if prop, ok := finals[c.text]; ok {
			cr.Proposed = len(prop.Candidates)
			cr.ProposeError = prop.ErrorKind
			for _, pc := range prop.Candidates {
				cr.ProposedTypes[pc.Type]++
				m.Generation.ProposedTypes[pc.Type]++
				if pc.Deictic != "" {
					m.Generation.DeicticStatements++
				}
				if !pc.Valid {
					m.Generation.DiscardedByContract = append(m.Generation.DiscardedByContract,
						discardedRun{ChunkSeq: c.seq, Type: pc.Type, Issues: pc.Issues})
				}
			}
		}
		if cr.ProposedTypes["open_ended"] == 0 {
			m.Generation.ChunksWithoutOpenEnded++
		}
		m.Generation.ProposedTotal += cr.Proposed
		perChunk = append(perChunk, cr.Proposed)
		d.Chunks = append(d.Chunks, cr)
	}
	m.Generation.Chunks = len(store.chunks)
	m.Generation.ProposedPerChunk = newDistribution(perChunk)

	m.Support.ByType = map[string]map[string]int{}
	m.Reduction.FinalStatus = map[string]int{}
	m.Draft.SelectedTypes = map[string]int{}
	m.Draft.Questions = []selectedQuestion{}
	m.Draft.Target = target
	m.Draft.Sections = len(sections)
	sectionsHit := map[int]struct{}{}

	records, _ := store.ListCandidates(context.Background(), store.job.JobID)
	for _, rec := range records {
		cr := candidateRun{
			ID: rec.ID, ChunkSeq: rec.ChunkSequence, Section: chunkSection[rec.ChunkSequence],
			Status: rec.Status, Score: rec.Score, DedupeGroup: rec.DedupeGroup, Payload: rec.Payload,
			AnswerInChunk: []string{},
		}
		m.Reduction.FinalStatus[rec.Status]++
		var p materialpipeline.CandidatePayloadV1
		if err := json.Unmarshal(rec.Payload, &p); err == nil {
			cr.AnswerInChunk = answerInChunk(p, chunkText[rec.ChunkSequence])
			if m.Support.ByType[p.QuestionType] == nil {
				m.Support.ByType[p.QuestionType] = map[string]int{}
			}
			for _, s := range cr.AnswerInChunk {
				m.Support.ByType[p.QuestionType][s]++
			}
			cr.SourceIdeas, cr.SourceGiven = sourceIdeasGiven(p.SourceIdeas, chunkIdeas[rec.ChunkSequence])
			m.Support.SourceIdeas += cr.SourceIdeas
			m.Support.SourceIdeasGiven += cr.SourceGiven
			if cr.SourceIdeas > 0 && cr.SourceIdeas == cr.SourceGiven {
				m.Support.CandidatesAllTrue++
			}
			if rec.Status == candStatusSelected {
				m.Draft.Selected++
				m.Draft.SelectedTypes[p.QuestionType]++
				m.Draft.Questions = append(m.Draft.Questions, selectedQuestion{
					ChunkSeq: rec.ChunkSequence, Section: cr.Section, Type: p.QuestionType, Text: p.QuestionText,
				})
				if cr.Section >= 0 {
					sectionsHit[cr.Section] = struct{}{}
				}
			}
		}
		m.Support.Candidates++
		d.Candidates = append(d.Candidates, cr)
	}
	m.Draft.SectionsWithSelected = len(sectionsHit)

	fillReduction(&m, res)
	fillCost(&m, res)
	return m, d
}

// fillReduction copia los informes de las pasadas: de cada una, su primera ejecución
// sin error (la que vio el trabajo entero).
func fillReduction(m *caseMetrics, res *railResult) {
	if rep, ok := res.dedupe.log.firstOK(); ok {
		m.Reduction.DedupeRan = true
		m.Reduction.PairsText, m.Reduction.PairsEmbed, m.Reduction.PairsLLM = rep.PairsText, rep.PairsEmbed, rep.PairsLLM
		m.Reduction.Clusters, m.Reduction.DroppedDup = rep.Clusters, rep.DroppedDup
	}
	if rep, ok := res.relevance.log.firstOK(); ok {
		m.Reduction.RelevanceRan = true
		m.Reduction.Scored, m.Reduction.DroppedIrrelevant, m.Reduction.Unscored = rep.Scored, rep.DroppedIrrelevant, rep.Unscored
	}
	if rep, ok := res.quality.log.firstOK(); ok {
		m.Reduction.QualityRan = true
		m.Reduction.DroppedInvalid = rep.DroppedInvalid
	}
	if rep, ok := res.selection.log.firstOK(); ok {
		m.Draft.SelectionRan = true
		m.Draft.IdeasCovered = rep.IdeasCubiertas
		m.Draft.IdeasUncovered = len(rep.IdeasSinCubrir)
	}
}

// fillCost copia las cuentas de las sondas y los tiempos por fase.
func fillCost(m *caseMetrics, res *railResult) {
	res.model.mu.Lock()
	m.Cost.Digest, m.Cost.Propose = res.model.digest, res.model.propose
	m.Cost.Judge, m.Cost.Relevance = res.model.judge, res.model.relevance
	res.model.mu.Unlock()
	m.Cost.Embed = res.embedder.stats
	if !res.store.chunkedAt.IsZero() {
		m.Cost.Phase0MS = res.store.chunkedAt.Sub(res.StartedAt).Milliseconds()
		if !res.store.phase1Done.IsZero() {
			m.Cost.Phase1MS = res.store.phase1Done.Sub(res.store.chunkedAt).Milliseconds()
		}
	}
	m.Cost.DedupeMS = res.dedupe.log.elapsedMS()
	m.Cost.RelevanceMS = res.relevance.log.elapsedMS()
	m.Cost.QualityMS = res.quality.log.elapsedMS()
	m.Cost.SelectionMS = res.selection.log.elapsedMS()
	m.Cost.TotalMS = res.EndedAt.Sub(res.StartedAt).Milliseconds()
	if n := len(res.Attempts); n > 1 {
		m.Cost.Redeliveries = n - 1
	}
}

// passRuns reúne las ejecuciones de las cuatro pasadas, para el JSON de la corrida.
type passRuns struct {
	Dedupe    []passRun[reduce.DedupeReport]    `json:"dedupe"`
	Relevance []passRun[reduce.RelevanceReport] `json:"relevance"`
	Quality   []passRun[reduce.QualityReport]   `json:"quality"`
	Selection []passRun[reduce.SelectionReport] `json:"selection"`
}

func collectPassRuns(res *railResult) passRuns {
	return passRuns{
		Dedupe:    res.dedupe.log.runs,
		Relevance: res.relevance.log.runs,
		Quality:   res.quality.log.runs,
		Selection: res.selection.log.runs,
	}
}
