package worker

import (
	"context"
	"sync"

	"go.uber.org/zap"
)

// Worker defines the common contract for all background workers
type Worker interface {
	Start(ctx context.Context) error
	Stop()
	Name() string
}

// Manager manages the lifecycle of all background workers
type Manager struct {
	workers []Worker
	logger  *zap.Logger
	mu      sync.RWMutex
}

// NewManager creates a new worker manager
func NewManager(logger *zap.Logger) *Manager {
	return &Manager{
		workers: make([]Worker, 0),
		logger:  logger,
	}
}

// Register adds a worker to be managed
func (m *Manager) Register(w Worker) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.workers = append(m.workers, w)
}

// StartAll starts all registered workers
func (m *Manager) StartAll(ctx context.Context) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, w := range m.workers {
		if err := w.Start(ctx); err != nil {
			m.logger.Error("Failed to start worker",
				zap.String("name", w.Name()),
				zap.Error(err))
			return err
		}
		m.logger.Info("Worker started", zap.String("name", w.Name()))
	}
	return nil
}

// StopAll stops all registered workers in reverse order
func (m *Manager) StopAll() {
	m.mu.RLock()
	workers := make([]Worker, len(m.workers))
	copy(workers, m.workers)
	m.mu.RUnlock()

	// Stop in reverse order (LIFO)
	for i := len(workers) - 1; i >= 0; i-- {
		w := workers[i]
		w.Stop()
		m.logger.Info("Worker stopped", zap.String("name", w.Name()))
	}
}

// Count returns the number of registered workers
func (m *Manager) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.workers)
}
