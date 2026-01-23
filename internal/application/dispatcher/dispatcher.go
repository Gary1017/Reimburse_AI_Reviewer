package dispatcher

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/garyjia/ai-reimbursement/internal/domain/event"
)

// Dispatcher routes events to registered handlers
type Dispatcher interface {
	// Subscribe registers a handler for an event type
	Subscribe(eventType event.Type, handler Handler)

	// SubscribeNamed registers a handler with a name for debugging
	SubscribeNamed(eventType event.Type, name string, handler Handler)

	// Unsubscribe removes a handler by name
	Unsubscribe(eventType event.Type, name string)

	// Dispatch sends event to all registered handlers synchronously
	// Returns first error encountered (handlers run in order)
	Dispatch(ctx context.Context, evt *event.Event) error

	// DispatchAsync sends event to handlers asynchronously
	// Does not wait for handlers to complete
	DispatchAsync(ctx context.Context, evt *event.Event)

	// ListHandlers returns registered handlers for an event type
	ListHandlers(eventType event.Type) []HandlerInfo

	// Close shuts down the dispatcher and waits for async handlers
	Close() error
}

// Logger interface for minimal logging dependency
type Logger interface {
	Info(msg string, keysAndValues ...interface{})
	Error(msg string, keysAndValues ...interface{})
}

// eventDispatcher is the concrete implementation of Dispatcher
type eventDispatcher struct {
	mu       sync.RWMutex
	handlers map[event.Type][]HandlerInfo
	logger   Logger

	// For async dispatch
	wg     sync.WaitGroup
	closed atomic.Bool
}

// Option configures the dispatcher
type Option func(*eventDispatcher)

// WithLogger sets a logger for the dispatcher
func WithLogger(logger Logger) Option {
	return func(d *eventDispatcher) {
		d.logger = logger
	}
}

// NewDispatcher creates a new event dispatcher
func NewDispatcher(opts ...Option) Dispatcher {
	d := &eventDispatcher{
		handlers: make(map[event.Type][]HandlerInfo),
	}

	for _, opt := range opts {
		opt(d)
	}

	return d
}

// Subscribe registers a handler for an event type with an auto-generated name
func (d *eventDispatcher) Subscribe(eventType event.Type, handler Handler) {
	name := fmt.Sprintf("handler-%d", len(d.handlers[eventType]))
	d.SubscribeNamed(eventType, name, handler)
}

// SubscribeNamed registers a handler with a specific name for debugging
func (d *eventDispatcher) SubscribeNamed(eventType event.Type, name string, handler Handler) {
	d.mu.Lock()
	defer d.mu.Unlock()

	info := HandlerInfo{
		Name:      name,
		EventType: eventType,
		Handler:   handler,
	}

	d.handlers[eventType] = append(d.handlers[eventType], info)

	if d.logger != nil {
		d.logger.Info("Handler registered",
			"event_type", eventType,
			"handler_name", name,
		)
	}
}

// Unsubscribe removes a handler by name
func (d *eventDispatcher) Unsubscribe(eventType event.Type, name string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	handlers := d.handlers[eventType]
	filtered := make([]HandlerInfo, 0, len(handlers))

	for _, h := range handlers {
		if h.Name != name {
			filtered = append(filtered, h)
		}
	}

	d.handlers[eventType] = filtered

	if d.logger != nil {
		d.logger.Info("Handler unregistered",
			"event_type", eventType,
			"handler_name", name,
		)
	}
}

// Dispatch sends event to all registered handlers synchronously
func (d *eventDispatcher) Dispatch(ctx context.Context, evt *event.Event) error {
	if d.closed.Load() {
		return fmt.Errorf("dispatcher is closed")
	}

	d.mu.RLock()
	handlers := d.handlers[evt.Type]
	d.mu.RUnlock()

	if d.logger != nil {
		d.logger.Info("Dispatching event",
			"event_type", evt.Type,
			"event_id", evt.ID,
			"handler_count", len(handlers),
		)
	}

	for _, info := range handlers {
		if err := d.safeExecute(ctx, evt, info); err != nil {
			if d.logger != nil {
				d.logger.Error("Handler error",
					"event_type", evt.Type,
					"event_id", evt.ID,
					"handler_name", info.Name,
					"error", err,
				)
			}
			return fmt.Errorf("handler %s failed: %w", info.Name, err)
		}
	}

	return nil
}

// DispatchAsync sends event to handlers asynchronously
func (d *eventDispatcher) DispatchAsync(ctx context.Context, evt *event.Event) {
	if d.closed.Load() {
		if d.logger != nil {
			d.logger.Error("Cannot dispatch async event, dispatcher is closed",
				"event_type", evt.Type,
				"event_id", evt.ID,
			)
		}
		return
	}

	d.mu.RLock()
	handlers := d.handlers[evt.Type]
	d.mu.RUnlock()

	if d.logger != nil {
		d.logger.Info("Dispatching event asynchronously",
			"event_type", evt.Type,
			"event_id", evt.ID,
			"handler_count", len(handlers),
		)
	}

	for _, info := range handlers {
		d.wg.Add(1)
		go func(h HandlerInfo) {
			defer d.wg.Done()

			if err := d.safeExecute(ctx, evt, h); err != nil {
				if d.logger != nil {
					d.logger.Error("Async handler error",
						"event_type", evt.Type,
						"event_id", evt.ID,
						"handler_name", h.Name,
						"error", err,
					)
				}
			}
		}(info)
	}
}

// ListHandlers returns registered handlers for an event type
func (d *eventDispatcher) ListHandlers(eventType event.Type) []HandlerInfo {
	d.mu.RLock()
	defer d.mu.RUnlock()

	handlers := d.handlers[eventType]
	result := make([]HandlerInfo, len(handlers))

	for i, h := range handlers {
		result[i] = HandlerInfo{
			Name:        h.Name,
			EventType:   h.EventType,
			Description: h.Description,
			// Note: Handler function is not copied to avoid exposing internal details
		}
	}

	return result
}

// Close shuts down the dispatcher and waits for async handlers to complete
func (d *eventDispatcher) Close() error {
	if !d.closed.CompareAndSwap(false, true) {
		return fmt.Errorf("dispatcher already closed")
	}

	if d.logger != nil {
		d.logger.Info("Closing dispatcher, waiting for async handlers")
	}

	d.wg.Wait()

	if d.logger != nil {
		d.logger.Info("Dispatcher closed")
	}

	return nil
}

// safeExecute runs a handler with panic recovery
func (d *eventDispatcher) safeExecute(ctx context.Context, evt *event.Event, info HandlerInfo) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("handler panic: %v", r)
			if d.logger != nil {
				d.logger.Error("Handler panic recovered",
					"event_type", evt.Type,
					"event_id", evt.ID,
					"handler_name", info.Name,
					"panic", r,
				)
			}
		}
	}()

	return info.Handler(ctx, evt)
}
