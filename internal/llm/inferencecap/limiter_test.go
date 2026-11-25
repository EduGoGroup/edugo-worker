package inferencecap

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

const testWait = 5 * time.Second

// waitQueued blocks until the limiter has n callers queued.
func waitQueued(t *testing.T, l *Limiter, n int) {
	t.Helper()
	deadline := time.Now().Add(testWait)
	for l.Waiting() != n {
		if time.Now().After(deadline) {
			t.Fatalf("la fila no llegó a %d (tiene %d)", n, l.Waiting())
		}
		time.Sleep(time.Millisecond)
	}
}

func TestLimiter_NeverMoreThanN(t *testing.T) {
	for _, n := range []int{1, 2} {
		l := New("local", n, nil)
		var inFlight, peak, done atomic.Int32
		var wg sync.WaitGroup
		for i := 0; i < 20; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				release, err := l.Acquire(context.Background())
				if err != nil {
					t.Errorf("Acquire: %v", err)
					return
				}
				cur := inFlight.Add(1)
				for {
					old := peak.Load()
					if cur <= old || peak.CompareAndSwap(old, cur) {
						break
					}
				}
				time.Sleep(time.Millisecond)
				inFlight.Add(-1)
				release()
				done.Add(1)
			}()
		}
		wg.Wait()
		if got := peak.Load(); got > int32(n) {
			t.Fatalf("N=%d: máximo en vuelo %d", n, got)
		}
		if done.Load() != 20 {
			t.Fatalf("N=%d: terminaron %d de 20", n, done.Load())
		}
	}
}

func TestLimiter_FIFO(t *testing.T) {
	l := New("local", 1, nil)
	first, err := l.Acquire(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	var order []int
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			release, err := l.Acquire(context.Background())
			if err != nil {
				t.Errorf("Acquire: %v", err)
				return
			}
			mu.Lock()
			order = append(order, i)
			mu.Unlock()
			release()
		}(i)
		waitQueued(t, l, i+1) // each one is queued before the next asks
	}
	first()
	wg.Wait()
	for i, got := range order {
		if got != i {
			t.Fatalf("orden de turnos %v, se esperaba el de llegada", order)
		}
	}
}

func TestLimiter_CancelledWaiterLeaves(t *testing.T) {
	l := New("local", 1, nil)
	first, _ := l.Acquire(context.Background())

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() {
		_, err := l.Acquire(ctx)
		errCh <- err
	}()
	waitQueued(t, l, 1)

	next := make(chan func(), 1)
	go func() {
		release, err := l.Acquire(context.Background())
		if err != nil {
			t.Errorf("Acquire: %v", err)
		}
		next <- release
	}()
	waitQueued(t, l, 2)

	cancel()
	if err := <-errCh; !errors.Is(err, context.Canceled) {
		t.Fatalf("se esperaba un error que envuelva context.Canceled, llegó %v", err)
	}
	waitQueued(t, l, 1) // the cancelled one left, it took no turn

	first()
	select {
	case release := <-next:
		release()
	case <-time.After(testWait):
		t.Fatal("el siguiente no recibió el turno")
	}

	// An already cancelled context never queues.
	if _, err := l.Acquire(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("contexto ya cancelado: %v", err)
	}

	// Cancel right when the turn is granted: whoever wins, the turn is never lost.
	for i := 0; i < 200; i++ {
		held, _ := l.Acquire(context.Background())
		raceCtx, raceCancel := context.WithCancel(context.Background())
		got := make(chan func(), 1)
		go func() {
			release, err := l.Acquire(raceCtx)
			if err != nil {
				release = nil
			}
			got <- release
		}()
		waitQueued(t, l, 1)
		go raceCancel()
		held()
		if release := <-got; release != nil {
			release()
		}
		probeCtx, probeCancel := context.WithTimeout(context.Background(), testWait)
		release, err := l.Acquire(probeCtx)
		probeCancel()
		if err != nil {
			t.Fatalf("vuelta %d: el turno se perdió: %v", i, err)
		}
		release()
	}
}

func TestLimiter_HoldCoversItsCalls(t *testing.T) {
	var waits []Wait
	l := New("local", 1, func(w Wait) { waits = append(waits, w) })

	held, releaseHold, err := l.Hold(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		release, err := l.Acquire(held)
		if err != nil {
			t.Fatalf("Acquire bajo Hold: %v", err)
		}
		release()
	}
	if l.inFlight != 1 {
		t.Fatalf("en vuelo %d con un Hold y sus llamadas: se contó dos veces", l.inFlight)
	}
	if len(waits) != 1 || !waits[0].Message {
		t.Fatalf("se esperaba una sola espera, la del mensaje: %+v", waits)
	}

	other := make(chan func(), 1)
	go func() {
		release, err := l.Acquire(context.Background())
		if err != nil {
			t.Errorf("Acquire: %v", err)
		}
		other <- release
	}()
	waitQueued(t, l, 1)
	select {
	case <-other:
		t.Fatal("otro contexto pasó con el Hold tomado")
	default:
	}

	releaseHold()
	releaseHold() // twice is harmless
	select {
	case release := <-other:
		release()
		release()
	case <-time.After(testWait):
		t.Fatal("al soltar el Hold, el otro no recibió turno")
	}
	if l.inFlight != 0 {
		t.Fatalf("en vuelo %d al terminar", l.inFlight)
	}

	// A nil limiter is "no cap".
	var none *Limiter
	ctx, release, err := none.Hold(context.Background())
	if err != nil || ctx == nil {
		t.Fatalf("Hold sin tope: %v", err)
	}
	release()
	if release, err = none.Acquire(context.Background()); err != nil {
		t.Fatalf("Acquire sin tope: %v", err)
	}
	release()
}

func TestLimiter_HoldIsPerLimiter(t *testing.T) {
	local := New("local", 1, nil)
	api := New("api", 1, nil)

	held, releaseHold, err := local.Hold(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer releaseHold()

	busy, _ := api.Acquire(context.Background())
	ctx, cancel := context.WithCancel(held)
	errCh := make(chan error, 1)
	go func() {
		_, err := api.Acquire(ctx)
		errCh <- err
	}()
	waitQueued(t, api, 1) // the local mark does not open the api turn
	cancel()
	if err := <-errCh; !errors.Is(err, context.Canceled) {
		t.Fatalf("se esperaba context.Canceled, llegó %v", err)
	}
	busy()
}
