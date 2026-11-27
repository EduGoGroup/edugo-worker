// Package inferencecap caps how many inferences are in flight against one model backend
// (HU-IA.10). The turn is asked right before the HTTP request opens, so the client clock
// only counts the inference, never the wait.
package inferencecap

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Wait describes one granted turn, for logs and metrics.
type Wait struct {
	// Limiter is the name of the limiter that granted the turn ("local", "api").
	Limiter string
	// Duration is how long the caller waited for the turn.
	Duration time.Duration
	// Message is true for a turn taken with Hold (a whole message), false for one call.
	Message bool
}

// Observer receives every granted turn. It must not block.
type Observer func(Wait)

// Limiter grants at most N turns at a time, in arrival order. A nil *Limiter is
// "no cap": Acquire and Hold return at once.
type Limiter struct {
	name     string
	capacity int
	observer Observer

	mu       sync.Mutex
	inFlight int
	queue    []*waiter
	// onQueue, when set, is told the queue length every time it changes.
	onQueue func(waiting int)
}

type waiter struct {
	ready   chan struct{}
	granted bool
}

// holdKey marks a context as already holding a turn of one specific limiter.
type holdKey struct{ limiter *Limiter }

// New builds a limiter with the given capacity. A capacity below 1 is taken as 1.
func New(name string, capacity int, observer Observer) *Limiter {
	if capacity < 1 {
		capacity = 1
	}
	return &Limiter{name: name, capacity: capacity, observer: observer}
}

// OnQueueChange registers a hook that receives the number of queued callers every time
// it changes. It runs with the limiter locked: it must not block nor call the limiter.
// Set it before the limiter is shared.
func (l *Limiter) OnQueueChange(fn func(waiting int)) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.onQueue = fn
}

// queueChanged reports the queue length to the hook. Callers hold l.mu.
func (l *Limiter) queueChanged() {
	if l.onQueue != nil {
		l.onQueue(len(l.queue))
	}
}

// Name returns the limiter name; a nil limiter has none.
func (l *Limiter) Name() string {
	if l == nil {
		return ""
	}
	return l.name
}

// Capacity returns the maximum number of turns in flight; 0 means no cap (nil limiter).
func (l *Limiter) Capacity() int {
	if l == nil {
		return 0
	}
	return l.capacity
}

// Acquire waits for one turn, for one call. The returned release is idempotent and must
// be called when the call ends. A context marked by Hold of this same limiter passes at
// once without taking a second turn. A cancelled context leaves the queue with an error
// that wraps the context error and takes no turn.
func (l *Limiter) Acquire(ctx context.Context) (release func(), err error) {
	if l == nil || ctx.Value(holdKey{l}) != nil {
		return func() {}, nil
	}
	return l.acquire(ctx, false)
}

// Hold waits for one turn for a whole message and returns a context marked with it:
// every Acquire of this limiter made with that context passes without waiting. The
// returned release is idempotent.
func (l *Limiter) Hold(ctx context.Context) (context.Context, func(), error) {
	if l == nil || ctx.Value(holdKey{l}) != nil {
		return ctx, func() {}, nil
	}
	release, err := l.acquire(ctx, true)
	if err != nil {
		return ctx, func() {}, err
	}
	return context.WithValue(ctx, holdKey{l}, true), release, nil
}

func (l *Limiter) acquire(ctx context.Context, message bool) (func(), error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("waiting for %s inference turn: %w", l.name, err)
	}
	start := time.Now()

	l.mu.Lock()
	if l.inFlight < l.capacity && len(l.queue) == 0 {
		l.inFlight++
		l.mu.Unlock()
		return l.granted(start, message), nil
	}
	w := &waiter{ready: make(chan struct{})}
	l.queue = append(l.queue, w)
	l.queueChanged()
	l.mu.Unlock()

	select {
	case <-w.ready:
		return l.granted(start, message), nil
	case <-ctx.Done():
		l.mu.Lock()
		if w.granted {
			// The turn was handed over just as the context ended: pass it on.
			l.mu.Unlock()
			l.release()
		} else {
			l.dequeue(w)
			l.mu.Unlock()
		}
		return nil, fmt.Errorf("waiting for %s inference turn: %w", l.name, ctx.Err())
	}
}

// granted reports the wait and builds the idempotent release of one turn.
func (l *Limiter) granted(start time.Time, message bool) func() {
	if l.observer != nil {
		l.observer(Wait{Limiter: l.name, Duration: time.Since(start), Message: message})
	}
	var once sync.Once
	return func() { once.Do(l.release) }
}

// release hands the turn to the first waiter, or frees it when nobody waits.
func (l *Limiter) release() {
	l.mu.Lock()
	defer l.mu.Unlock()
	if len(l.queue) == 0 {
		l.inFlight--
		return
	}
	w := l.queue[0]
	l.queue = l.queue[1:]
	l.queueChanged()
	w.granted = true
	close(w.ready)
}

// dequeue removes a waiter that gave up. Callers hold l.mu.
func (l *Limiter) dequeue(w *waiter) {
	for i, queued := range l.queue {
		if queued == w {
			l.queue = append(l.queue[:i], l.queue[i+1:]...)
			l.queueChanged()
			return
		}
	}
}

// Waiting returns how many callers are queued; a nil limiter has none.
func (l *Limiter) Waiting() int {
	if l == nil {
		return 0
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.queue)
}
