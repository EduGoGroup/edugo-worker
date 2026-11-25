package bootstrap

import (
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/EduGoGroup/edugo-shared/logger"
	"github.com/EduGoGroup/edugo-shared/messaging/rabbit"

	"github.com/EduGoGroup/edugo-worker/internal/application/processor"
)

type logLine struct {
	level  string
	msg    string
	fields map[string]any
}

// recordingLogger guarda cada línea con sus campos, para afirmar sobre ellas.
type recordingLogger struct {
	mu    sync.Mutex
	lines []logLine
}

func (l *recordingLogger) record(level, msg string, kv []any) {
	fields := make(map[string]any, len(kv)/2)
	for i := 0; i+1 < len(kv); i += 2 {
		fields[fmt.Sprint(kv[i])] = kv[i+1]
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.lines = append(l.lines, logLine{level: level, msg: msg, fields: fields})
}

func (l *recordingLogger) Debug(msg string, kv ...any) { l.record("debug", msg, kv) }
func (l *recordingLogger) Info(msg string, kv ...any)  { l.record("info", msg, kv) }
func (l *recordingLogger) Warn(msg string, kv ...any)  { l.record("warn", msg, kv) }
func (l *recordingLogger) Error(msg string, kv ...any) { l.record("error", msg, kv) }
func (l *recordingLogger) Fatal(msg string, kv ...any) { l.record("fatal", msg, kv) }
func (l *recordingLogger) Sync() error                 { return nil }
func (l *recordingLogger) With(...any) logger.Logger   { return l }

func assertFields(t *testing.T, line logLine, want map[string]any) {
	t.Helper()
	for key, value := range want {
		got, ok := line.fields[key]
		if !ok {
			t.Errorf("line %q: missing field %q", line.msg, key)
			continue
		}
		if got != value {
			t.Errorf("line %q: field %q = %v, want %v", line.msg, key, got, value)
		}
	}
}

func TestConsumerHooks_LogRetryAndDeadLetter(t *testing.T) {
	t.Parallel()
	log := &recordingLogger{}
	base := rabbit.ConsumerConfig{Name: "edugo-worker", PrefetchCount: 5, DLQ: rabbit.DLQConfig{Enabled: true, MaxRetries: 3}}

	cfg := WithFailureHooks(base, log)

	if cfg.Name != base.Name || cfg.PrefetchCount != base.PrefetchCount || cfg.DLQ != base.DLQ {
		t.Fatalf("the rest of the config must survive: %+v", cfg)
	}
	if base.IsPermanent != nil || base.OnRetry != nil || base.OnDeadLetter != nil {
		t.Fatal("the config received must not be mutated")
	}

	// El clasificador cableado es el del worker.
	if cfg.IsPermanent == nil {
		t.Fatal("IsPermanent must be wired")
	}
	if !cfg.IsPermanent(fmt.Errorf("%w: broken", processor.ErrUnroutableMessage)) {
		t.Error("an unroutable message must be permanent")
	}
	if cfg.IsPermanent(errors.New("model is down")) {
		t.Error("an unknown error must not be permanent")
	}

	cfg.OnRetry(rabbit.FailureEvent{
		Err: errors.New("model is down"), Queue: "review.queue", Retry: 1, MaxRetries: 3,
		Backoff: 10 * time.Second, BodySize: 42, BodyHash: "abc123def456",
	})
	cfg.OnDeadLetter(rabbit.FailureEvent{
		Err: fmt.Errorf("%w: broken", processor.ErrUnroutableMessage), Queue: "review.queue", Retry: 0,
		MaxRetries: 3, Reason: rabbit.DeadReasonPermanent, BodySize: 7, BodyHash: "0123456789ab",
	})

	if len(log.lines) != 2 {
		t.Fatalf("expected 2 log lines, got %d", len(log.lines))
	}

	retry := log.lines[0]
	if retry.level != "warn" || retry.msg != RetryLogMessage {
		t.Errorf("unexpected retry line: %s %q", retry.level, retry.msg)
	}
	assertFields(t, retry, map[string]any{
		"queue": "review.queue", "retry": 1, "max_retries": 3, "backoff": "10s",
		"reason": "transient", "error": "model is down", "body_size": 42, "body_hash": "abc123def456",
	})

	dead := log.lines[1]
	if dead.level != "error" || dead.msg != DeadLetterLogMessage {
		t.Errorf("unexpected dead letter line: %s %q", dead.level, dead.msg)
	}
	assertFields(t, dead, map[string]any{
		"queue": "review.queue", "retry": 0, "max_retries": 3, "reason": rabbit.DeadReasonPermanent,
		"error": "mensaje no enrutable: broken", "body_size": 7, "body_hash": "0123456789ab",
	})
}

func TestConsumerHooks_NilErrorDoesNotPanic(t *testing.T) {
	t.Parallel()
	log := &recordingLogger{}
	cfg := WithFailureHooks(rabbit.ConsumerConfig{}, log)

	cfg.OnDeadLetter(rabbit.FailureEvent{Queue: "q", Reason: rabbit.DeadReasonMaxRetries})

	if len(log.lines) != 1 || log.lines[0].fields["error"] != "" {
		t.Fatalf("unexpected lines: %+v", log.lines)
	}
}
