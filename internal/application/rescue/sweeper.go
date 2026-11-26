// Package rescue holds the worker's clock for the rescue of failed AI work (HU-IA.6, ADR
// 0050): every sweep_interval it asks learning to sweep. Learning owns the state
// (assessment.ai_failure) and the republishing; the worker only keeps the time.
package rescue

import (
	"context"
	"fmt"
	"time"

	"github.com/EduGoGroup/edugo-shared/logger"
	"github.com/EduGoGroup/edugo-worker/internal/client/m2m"
	"github.com/EduGoGroup/edugo-worker/internal/config"
)

// Líneas de log del planificador. Son estables: el guion R12 las busca como evidencia.
const (
	StartedLogMessage     = "planificador del rescate arrancado"
	DisabledLogMessage    = "planificador del rescate apagado (ai_rescue.enabled=false)"
	SweptLogMessage       = "barrido del rescate hecho"
	SweepFailedLogMessage = "barrido del rescate falló, se reintenta en el próximo tic"
	StoppedLogMessage     = "planificador del rescate detenido"
)

// SweepClient is the part of the rescue client the sweeper uses.
// *m2m.LearningRescueClient satisfies it.
type SweepClient interface {
	Sweep(ctx context.Context, req m2m.SweepRequest) (m2m.SweepResponse, error)
}

// Ticker is the clock the sweeper waits on; *time.Ticker is adapted by realTicker.
type Ticker interface {
	C() <-chan time.Time
	Stop()
}

type realTicker struct{ t *time.Ticker }

func (r realTicker) C() <-chan time.Time { return r.t.C }
func (r realTicker) Stop()               { r.t.Stop() }

func newRealTicker(d time.Duration) Ticker { return realTicker{t: time.NewTicker(d)} }

// Sweeper calls the sweep on every tick. A failed sweep is logged and does not stop it;
// the cancelled context does. Sweeps never overlap: the next tick waits for the current one.
type Sweeper struct {
	client    SweepClient
	settings  config.AIRescueSettings
	logger    logger.Logger
	newTicker func(time.Duration) Ticker
}

// NewSweeper builds the sweeper with the resolved ai_rescue settings.
func NewSweeper(client SweepClient, settings config.AIRescueSettings, log logger.Logger) *Sweeper {
	return &Sweeper{client: client, settings: settings, logger: log, newTicker: newRealTicker}
}

// WithTicker replaces the clock (tests).
func (s *Sweeper) WithTicker(newTicker func(time.Duration) Ticker) *Sweeper {
	s.newTicker = newTicker
	return s
}

// Start launches the loop in its own goroutine. It returns a channel that is closed once
// the loop has stopped (after the sweep in flight, if any) and whether it started: with
// enabled=false it does not start and the channel is nil.
func (s *Sweeper) Start(ctx context.Context) (<-chan struct{}, bool) {
	if !s.settings.Enabled {
		s.logger.Info(DisabledLogMessage)
		return nil, false
	}
	ticker := s.newTicker(s.settings.SweepInterval)
	done := make(chan struct{})
	s.logger.Info(StartedLogMessage,
		"sweep_interval", s.settings.SweepInterval.String(),
		"min_age", s.settings.MinAge.String(),
		"max_retries", s.settings.MaxRetries,
		"retry_backoff", fmt.Sprint(s.settings.RetryBackoff),
		"job_pending_stale_after", s.settings.JobPendingStaleAfter.String())
	go func() {
		defer close(done)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				s.logger.Info(StoppedLogMessage)
				return
			case <-ticker.C():
				s.sweep(ctx)
			}
		}
	}()
	return done, true
}

func (s *Sweeper) sweep(ctx context.Context) {
	start := time.Now()
	out, err := s.client.Sweep(ctx, s.request())
	if err != nil {
		s.logger.Warn(SweepFailedLogMessage, "error", err.Error(), "duration_ms", time.Since(start).Milliseconds())
		return
	}
	s.logger.Info(SweptLogMessage,
		"resolved", out.Resolved,
		"exhausted", out.Exhausted,
		"republished", out.Republished,
		"orphans", out.Orphans,
		"jobs_failed", out.JobsFailed,
		"duration_ms", time.Since(start).Milliseconds())
}

// request carries the configuration numbers on every call (decision 9): learning keeps
// no copy of them.
func (s *Sweeper) request() m2m.SweepRequest {
	backoff := make([]int64, len(s.settings.RetryBackoff))
	for i, d := range s.settings.RetryBackoff {
		backoff[i] = int64(d / time.Second)
	}
	return m2m.SweepRequest{
		MaxRetries:             s.settings.MaxRetries,
		RetryBackoffSeconds:    backoff,
		MinAgeSeconds:          int64(s.settings.MinAge / time.Second),
		JobPendingStaleSeconds: int64(s.settings.JobPendingStaleAfter / time.Second),
	}
}
