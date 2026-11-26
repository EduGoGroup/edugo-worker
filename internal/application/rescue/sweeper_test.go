package rescue

import (
	"context"
	"errors"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/EduGoGroup/edugo-shared/logger"
	"github.com/EduGoGroup/edugo-worker/internal/client/m2m"
	"github.com/EduGoGroup/edugo-worker/internal/config"
)

type nopLogger struct{}

func (nopLogger) Debug(string, ...interface{})        {}
func (nopLogger) Info(string, ...interface{})         {}
func (nopLogger) Warn(string, ...interface{})         {}
func (nopLogger) Error(string, ...interface{})        {}
func (nopLogger) Fatal(string, ...interface{})        {}
func (nopLogger) Sync() error                         { return nil }
func (l nopLogger) With(...interface{}) logger.Logger { return l }

// fakeTicker es el reloj doble: el test decide cuándo hay tic.
type fakeTicker struct {
	ch      chan time.Time
	mu      sync.Mutex
	stopped bool
	period  time.Duration
}

func (t *fakeTicker) C() <-chan time.Time { return t.ch }
func (t *fakeTicker) Stop() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.stopped = true
}

// fakeSweeps guarda cada barrido y avisa por un canal.
type fakeSweeps struct {
	mu    sync.Mutex
	reqs  []m2m.SweepRequest
	errs  []error // el i-ésimo barrido devuelve errs[i] si existe
	calls chan struct{}
}

func (f *fakeSweeps) Sweep(_ context.Context, req m2m.SweepRequest) (m2m.SweepResponse, error) {
	f.mu.Lock()
	i := len(f.reqs)
	f.reqs = append(f.reqs, req)
	var err error
	if i < len(f.errs) {
		err = f.errs[i]
	}
	f.mu.Unlock()
	f.calls <- struct{}{}
	if err != nil {
		return m2m.SweepResponse{}, err
	}
	return m2m.SweepResponse{Republished: 1}, nil
}

func testSettings(enabled bool) config.AIRescueSettings {
	return config.AIRescueSettings{
		Enabled:              enabled,
		SweepInterval:        5 * time.Minute,
		MinAge:               15 * time.Minute,
		MaxRetries:           5,
		RetryBackoff:         []time.Duration{2 * time.Minute, 10 * time.Minute, 30 * time.Minute, 2 * time.Hour, 6 * time.Hour},
		JobPendingStaleAfter: 6 * time.Hour,
	}
}

func waitCall(t *testing.T, calls chan struct{}) {
	t.Helper()
	select {
	case <-calls:
	case <-time.After(2 * time.Second):
		t.Fatal("el barrido no llegó tras el tic")
	}
}

// TestSweeper_TicksAndStops: un barrido por tic con los números de la configuración; un
// error no detiene el reloj; el contexto cancelado lo para; apagado no arranca.
func TestSweeper_TicksAndStops(t *testing.T) {
	ticker := &fakeTicker{ch: make(chan time.Time)}
	sweeps := &fakeSweeps{calls: make(chan struct{}, 4), errs: []error{nil, errors.New("learning caído")}}
	s := NewSweeper(sweeps, testSettings(true), nopLogger{}).
		WithTicker(func(d time.Duration) Ticker { ticker.period = d; return ticker })

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done, started := s.Start(ctx)
	if !started {
		t.Fatal("con enabled=true debe arrancar")
	}

	for i := 0; i < 3; i++ {
		ticker.ch <- time.Now()
		waitCall(t, sweeps.calls)
	}
	if ticker.period != 5*time.Minute {
		t.Fatalf("el reloj usa sweep_interval: got %v", ticker.period)
	}

	want := m2m.SweepRequest{
		MaxRetries:             5,
		RetryBackoffSeconds:    []int64{120, 600, 1800, 7200, 21600},
		MinAgeSeconds:          900,
		JobPendingStaleSeconds: 21600,
	}
	sweeps.mu.Lock()
	if len(sweeps.reqs) != 3 {
		t.Fatalf("esperaba 3 barridos (el segundo falló y el reloj siguió), hubo %d", len(sweeps.reqs))
	}
	for i, got := range sweeps.reqs {
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("barrido %d con números inesperados: got %+v, want %+v", i, got, want)
		}
	}
	sweeps.mu.Unlock()

	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("el contexto cancelado debe parar el planificador")
	}
	ticker.mu.Lock()
	stopped := ticker.stopped
	ticker.mu.Unlock()
	if !stopped {
		t.Fatal("al parar se suelta el reloj")
	}

	// Apagado: no arranca y no barre.
	off := &fakeSweeps{calls: make(chan struct{}, 1)}
	built := false
	s = NewSweeper(off, testSettings(false), nopLogger{}).
		WithTicker(func(time.Duration) Ticker { built = true; return &fakeTicker{ch: make(chan time.Time)} })
	if _, started := s.Start(context.Background()); started {
		t.Fatal("con enabled=false no arranca")
	}
	if built || len(off.reqs) != 0 {
		t.Fatalf("apagado no crea reloj ni barre: reloj=%v barridos=%d", built, len(off.reqs))
	}
}
