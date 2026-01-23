package dispatcher

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/garyjia/ai-reimbursement/internal/domain/event"
)

// mockLogger implements Logger for testing
type mockLogger struct {
	mu      sync.Mutex
	infos   []string
	errors  []string
	entries []map[string]interface{}
}

func (m *mockLogger) Info(msg string, keysAndValues ...interface{}) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.infos = append(m.infos, msg)

	entry := map[string]interface{}{"msg": msg}
	for i := 0; i < len(keysAndValues); i += 2 {
		if i+1 < len(keysAndValues) {
			entry[fmt.Sprint(keysAndValues[i])] = keysAndValues[i+1]
		}
	}
	m.entries = append(m.entries, entry)
}

func (m *mockLogger) Error(msg string, keysAndValues ...interface{}) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.errors = append(m.errors, msg)

	entry := map[string]interface{}{"msg": msg, "level": "error"}
	for i := 0; i < len(keysAndValues); i += 2 {
		if i+1 < len(keysAndValues) {
			entry[fmt.Sprint(keysAndValues[i])] = keysAndValues[i+1]
		}
	}
	m.entries = append(m.entries, entry)
}

func (m *mockLogger) InfoCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.infos)
}

func (m *mockLogger) ErrorCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.errors)
}

func (m *mockLogger) HasInfo(msg string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, info := range m.infos {
		if info == msg {
			return true
		}
	}
	return false
}

func TestNewDispatcher(t *testing.T) {
	t.Run("creates dispatcher without logger", func(t *testing.T) {
		d := NewDispatcher()
		if d == nil {
			t.Fatal("expected non-nil dispatcher")
		}
	})

	t.Run("creates dispatcher with logger", func(t *testing.T) {
		logger := &mockLogger{}
		d := NewDispatcher(WithLogger(logger))
		if d == nil {
			t.Fatal("expected non-nil dispatcher")
		}
	})
}

func TestSubscribe(t *testing.T) {
	t.Run("subscribes handler with auto-generated name", func(t *testing.T) {
		d := NewDispatcher()
		called := false
		handler := func(ctx context.Context, evt *event.Event) error {
			called = true
			return nil
		}

		d.Subscribe(event.TypeInstanceCreated, handler)

		evt := event.NewEvent(event.TypeInstanceCreated, 1, "lark-123", nil)
		if err := d.Dispatch(context.Background(), evt); err != nil {
			t.Fatalf("dispatch failed: %v", err)
		}

		if !called {
			t.Error("expected handler to be called")
		}
	})

	t.Run("subscribes multiple handlers to same event type", func(t *testing.T) {
		d := NewDispatcher()
		called1, called2 := false, false

		d.Subscribe(event.TypeInstanceCreated, func(ctx context.Context, evt *event.Event) error {
			called1 = true
			return nil
		})
		d.Subscribe(event.TypeInstanceCreated, func(ctx context.Context, evt *event.Event) error {
			called2 = true
			return nil
		})

		evt := event.NewEvent(event.TypeInstanceCreated, 1, "lark-123", nil)
		if err := d.Dispatch(context.Background(), evt); err != nil {
			t.Fatalf("dispatch failed: %v", err)
		}

		if !called1 || !called2 {
			t.Error("expected both handlers to be called")
		}
	})
}

func TestSubscribeNamed(t *testing.T) {
	t.Run("subscribes handler with custom name", func(t *testing.T) {
		logger := &mockLogger{}
		d := NewDispatcher(WithLogger(logger))

		handler := func(ctx context.Context, evt *event.Event) error {
			return nil
		}

		d.SubscribeNamed(event.TypeInstanceCreated, "test-handler", handler)

		if !logger.HasInfo("Handler registered") {
			t.Error("expected registration to be logged")
		}
	})

	t.Run("lists handlers by name", func(t *testing.T) {
		d := NewDispatcher()

		d.SubscribeNamed(event.TypeInstanceCreated, "handler-1", func(ctx context.Context, evt *event.Event) error {
			return nil
		})
		d.SubscribeNamed(event.TypeInstanceCreated, "handler-2", func(ctx context.Context, evt *event.Event) error {
			return nil
		})

		handlers := d.ListHandlers(event.TypeInstanceCreated)
		if len(handlers) != 2 {
			t.Fatalf("expected 2 handlers, got %d", len(handlers))
		}

		names := map[string]bool{}
		for _, h := range handlers {
			names[h.Name] = true
		}

		if !names["handler-1"] || !names["handler-2"] {
			t.Error("expected both handlers to be listed")
		}
	})
}

func TestUnsubscribe(t *testing.T) {
	t.Run("removes handler by name", func(t *testing.T) {
		d := NewDispatcher()
		called := false

		d.SubscribeNamed(event.TypeInstanceCreated, "handler-1", func(ctx context.Context, evt *event.Event) error {
			called = true
			return nil
		})

		d.Unsubscribe(event.TypeInstanceCreated, "handler-1")

		evt := event.NewEvent(event.TypeInstanceCreated, 1, "lark-123", nil)
		if err := d.Dispatch(context.Background(), evt); err != nil {
			t.Fatalf("dispatch failed: %v", err)
		}

		if called {
			t.Error("expected handler not to be called after unsubscribe")
		}
	})

	t.Run("removes only specified handler", func(t *testing.T) {
		d := NewDispatcher()
		called1, called2 := false, false

		d.SubscribeNamed(event.TypeInstanceCreated, "handler-1", func(ctx context.Context, evt *event.Event) error {
			called1 = true
			return nil
		})
		d.SubscribeNamed(event.TypeInstanceCreated, "handler-2", func(ctx context.Context, evt *event.Event) error {
			called2 = true
			return nil
		})

		d.Unsubscribe(event.TypeInstanceCreated, "handler-1")

		evt := event.NewEvent(event.TypeInstanceCreated, 1, "lark-123", nil)
		if err := d.Dispatch(context.Background(), evt); err != nil {
			t.Fatalf("dispatch failed: %v", err)
		}

		if called1 {
			t.Error("expected handler-1 not to be called")
		}
		if !called2 {
			t.Error("expected handler-2 to be called")
		}
	})
}

func TestDispatch(t *testing.T) {
	t.Run("dispatches to all handlers synchronously", func(t *testing.T) {
		d := NewDispatcher()
		order := []int{}
		var mu sync.Mutex

		d.Subscribe(event.TypeInstanceCreated, func(ctx context.Context, evt *event.Event) error {
			mu.Lock()
			order = append(order, 1)
			mu.Unlock()
			return nil
		})
		d.Subscribe(event.TypeInstanceCreated, func(ctx context.Context, evt *event.Event) error {
			mu.Lock()
			order = append(order, 2)
			mu.Unlock()
			return nil
		})

		evt := event.NewEvent(event.TypeInstanceCreated, 1, "lark-123", nil)
		if err := d.Dispatch(context.Background(), evt); err != nil {
			t.Fatalf("dispatch failed: %v", err)
		}

		if len(order) != 2 || order[0] != 1 || order[1] != 2 {
			t.Errorf("expected handlers to run in order [1, 2], got %v", order)
		}
	})

	t.Run("returns first error encountered", func(t *testing.T) {
		d := NewDispatcher()
		expectedErr := errors.New("handler error")
		called := false

		d.Subscribe(event.TypeInstanceCreated, func(ctx context.Context, evt *event.Event) error {
			return expectedErr
		})
		d.Subscribe(event.TypeInstanceCreated, func(ctx context.Context, evt *event.Event) error {
			called = true
			return nil
		})

		evt := event.NewEvent(event.TypeInstanceCreated, 1, "lark-123", nil)
		err := d.Dispatch(context.Background(), evt)

		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !errors.Is(err, expectedErr) {
			t.Errorf("expected error to wrap %v, got %v", expectedErr, err)
		}
		if called {
			t.Error("expected second handler not to be called after first error")
		}
	})

	t.Run("handles context cancellation", func(t *testing.T) {
		d := NewDispatcher()

		d.Subscribe(event.TypeInstanceCreated, func(ctx context.Context, evt *event.Event) error {
			return ctx.Err()
		})

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		evt := event.NewEvent(event.TypeInstanceCreated, 1, "lark-123", nil)
		err := d.Dispatch(ctx, evt)

		if err == nil {
			t.Fatal("expected error from cancelled context")
		}
	})

	t.Run("recovers from handler panic", func(t *testing.T) {
		logger := &mockLogger{}
		d := NewDispatcher(WithLogger(logger))

		d.Subscribe(event.TypeInstanceCreated, func(ctx context.Context, evt *event.Event) error {
			panic("test panic")
		})

		evt := event.NewEvent(event.TypeInstanceCreated, 1, "lark-123", nil)
		err := d.Dispatch(context.Background(), evt)

		if err == nil {
			t.Fatal("expected error from panic recovery")
		}
		if logger.ErrorCount() == 0 {
			t.Error("expected panic to be logged as error")
		}
	})

	t.Run("returns error when dispatcher is closed", func(t *testing.T) {
		d := NewDispatcher()
		if err := d.Close(); err != nil {
			t.Fatalf("close failed: %v", err)
		}

		evt := event.NewEvent(event.TypeInstanceCreated, 1, "lark-123", nil)
		err := d.Dispatch(context.Background(), evt)

		if err == nil {
			t.Fatal("expected error when dispatching to closed dispatcher")
		}
	})
}

func TestDispatchAsync(t *testing.T) {
	t.Run("dispatches to handlers asynchronously", func(t *testing.T) {
		d := NewDispatcher()
		var called atomic.Int32

		d.Subscribe(event.TypeInstanceCreated, func(ctx context.Context, evt *event.Event) error {
			time.Sleep(10 * time.Millisecond)
			called.Add(1)
			return nil
		})
		d.Subscribe(event.TypeInstanceCreated, func(ctx context.Context, evt *event.Event) error {
			time.Sleep(10 * time.Millisecond)
			called.Add(1)
			return nil
		})

		evt := event.NewEvent(event.TypeInstanceCreated, 1, "lark-123", nil)
		d.DispatchAsync(context.Background(), evt)

		// Should return immediately without waiting
		if called.Load() > 0 {
			t.Error("expected handlers not to have completed yet")
		}

		// Wait for handlers to complete
		if err := d.Close(); err != nil {
			t.Fatalf("close failed: %v", err)
		}

		if called.Load() != 2 {
			t.Errorf("expected 2 handlers to be called, got %d", called.Load())
		}
	})

	t.Run("does not block on handler errors", func(t *testing.T) {
		logger := &mockLogger{}
		d := NewDispatcher(WithLogger(logger))
		var called atomic.Int32

		d.Subscribe(event.TypeInstanceCreated, func(ctx context.Context, evt *event.Event) error {
			return errors.New("handler error")
		})
		d.Subscribe(event.TypeInstanceCreated, func(ctx context.Context, evt *event.Event) error {
			called.Add(1)
			return nil
		})

		evt := event.NewEvent(event.TypeInstanceCreated, 1, "lark-123", nil)
		d.DispatchAsync(context.Background(), evt)

		if err := d.Close(); err != nil {
			t.Fatalf("close failed: %v", err)
		}

		// Both handlers should have been called despite error
		if called.Load() != 1 {
			t.Errorf("expected second handler to be called, got %d calls", called.Load())
		}
		if logger.ErrorCount() == 0 {
			t.Error("expected error to be logged")
		}
	})

	t.Run("recovers from handler panic asynchronously", func(t *testing.T) {
		logger := &mockLogger{}
		d := NewDispatcher(WithLogger(logger))

		d.Subscribe(event.TypeInstanceCreated, func(ctx context.Context, evt *event.Event) error {
			panic("async panic")
		})

		evt := event.NewEvent(event.TypeInstanceCreated, 1, "lark-123", nil)
		d.DispatchAsync(context.Background(), evt)

		if err := d.Close(); err != nil {
			t.Fatalf("close failed: %v", err)
		}

		if logger.ErrorCount() == 0 {
			t.Error("expected panic to be logged as error")
		}
	})

	t.Run("does not dispatch when dispatcher is closed", func(t *testing.T) {
		logger := &mockLogger{}
		d := NewDispatcher(WithLogger(logger))
		var called atomic.Int32

		d.Subscribe(event.TypeInstanceCreated, func(ctx context.Context, evt *event.Event) error {
			called.Add(1)
			return nil
		})

		if err := d.Close(); err != nil {
			t.Fatalf("close failed: %v", err)
		}

		evt := event.NewEvent(event.TypeInstanceCreated, 1, "lark-123", nil)
		d.DispatchAsync(context.Background(), evt)

		// Give time for any goroutines to potentially start
		time.Sleep(50 * time.Millisecond)

		if called.Load() > 0 {
			t.Error("expected handler not to be called after close")
		}
		if logger.ErrorCount() == 0 {
			t.Error("expected error log for dispatching to closed dispatcher")
		}
	})
}

func TestListHandlers(t *testing.T) {
	t.Run("returns empty list for unregistered event type", func(t *testing.T) {
		d := NewDispatcher()
		handlers := d.ListHandlers(event.TypeInstanceCreated)
		if len(handlers) != 0 {
			t.Errorf("expected 0 handlers, got %d", len(handlers))
		}
	})

	t.Run("returns handler info without exposing function", func(t *testing.T) {
		d := NewDispatcher()

		d.SubscribeNamed(event.TypeInstanceCreated, "test-handler", func(ctx context.Context, evt *event.Event) error {
			return nil
		})

		handlers := d.ListHandlers(event.TypeInstanceCreated)
		if len(handlers) != 1 {
			t.Fatalf("expected 1 handler, got %d", len(handlers))
		}

		h := handlers[0]
		if h.Name != "test-handler" {
			t.Errorf("expected name 'test-handler', got '%s'", h.Name)
		}
		if h.EventType != event.TypeInstanceCreated {
			t.Errorf("expected event type %s, got %s", event.TypeInstanceCreated, h.EventType)
		}
		if h.Handler != nil {
			t.Error("expected handler function not to be exposed")
		}
	})

	t.Run("returns all handlers for event type", func(t *testing.T) {
		d := NewDispatcher()

		d.SubscribeNamed(event.TypeInstanceCreated, "handler-1", func(ctx context.Context, evt *event.Event) error {
			return nil
		})
		d.SubscribeNamed(event.TypeInstanceCreated, "handler-2", func(ctx context.Context, evt *event.Event) error {
			return nil
		})
		d.SubscribeNamed(event.TypeInstanceApproved, "other-handler", func(ctx context.Context, evt *event.Event) error {
			return nil
		})

		handlers := d.ListHandlers(event.TypeInstanceCreated)
		if len(handlers) != 2 {
			t.Fatalf("expected 2 handlers for TypeInstanceCreated, got %d", len(handlers))
		}
	})
}

func TestClose(t *testing.T) {
	t.Run("waits for async handlers to complete", func(t *testing.T) {
		d := NewDispatcher()
		var completed atomic.Bool

		d.Subscribe(event.TypeInstanceCreated, func(ctx context.Context, evt *event.Event) error {
			time.Sleep(50 * time.Millisecond)
			completed.Store(true)
			return nil
		})

		evt := event.NewEvent(event.TypeInstanceCreated, 1, "lark-123", nil)
		d.DispatchAsync(context.Background(), evt)

		if err := d.Close(); err != nil {
			t.Fatalf("close failed: %v", err)
		}

		if !completed.Load() {
			t.Error("expected async handler to complete before Close returns")
		}
	})

	t.Run("returns error on double close", func(t *testing.T) {
		d := NewDispatcher()

		if err := d.Close(); err != nil {
			t.Fatalf("first close failed: %v", err)
		}

		err := d.Close()
		if err == nil {
			t.Fatal("expected error on second close")
		}
	})

	t.Run("prevents new async dispatches after close", func(t *testing.T) {
		d := NewDispatcher()
		var called atomic.Int32

		d.Subscribe(event.TypeInstanceCreated, func(ctx context.Context, evt *event.Event) error {
			called.Add(1)
			return nil
		})

		if err := d.Close(); err != nil {
			t.Fatalf("close failed: %v", err)
		}

		evt := event.NewEvent(event.TypeInstanceCreated, 1, "lark-123", nil)
		d.DispatchAsync(context.Background(), evt)

		time.Sleep(50 * time.Millisecond)

		if called.Load() > 0 {
			t.Error("expected no handlers to be called after close")
		}
	})
}

func TestConcurrency(t *testing.T) {
	t.Run("handles concurrent subscriptions", func(t *testing.T) {
		d := NewDispatcher()
		var wg sync.WaitGroup

		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				d.SubscribeNamed(event.TypeInstanceCreated, fmt.Sprintf("handler-%d", id), func(ctx context.Context, evt *event.Event) error {
					return nil
				})
			}(i)
		}

		wg.Wait()

		handlers := d.ListHandlers(event.TypeInstanceCreated)
		if len(handlers) != 10 {
			t.Errorf("expected 10 handlers, got %d", len(handlers))
		}
	})

	t.Run("handles concurrent dispatch", func(t *testing.T) {
		d := NewDispatcher()
		var called atomic.Int32

		d.Subscribe(event.TypeInstanceCreated, func(ctx context.Context, evt *event.Event) error {
			called.Add(1)
			return nil
		})

		var wg sync.WaitGroup
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				evt := event.NewEvent(event.TypeInstanceCreated, 1, "lark-123", nil)
				d.Dispatch(context.Background(), evt)
			}()
		}

		wg.Wait()

		if called.Load() != 10 {
			t.Errorf("expected 10 handler calls, got %d", called.Load())
		}
	})
}
