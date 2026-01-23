package worker

import (
	"context"
	"fmt"
	"sync"

	"go.uber.org/zap"
)

// Worker defines the interface for background workers
type Worker interface {
	Start(ctx context.Context) error
	Stop() error
	Name() string
}

// WorkerManager manages lifecycle of multiple workers
// ARCH-124: Worker lifecycle management with graceful shutdown
type WorkerManager struct {
	workers []Worker
	logger  *zap.Logger

	mu        sync.RWMutex
	isRunning bool
	ctx       context.Context
	cancel    context.CancelFunc
}

// NewWorkerManager creates a new worker manager
func NewWorkerManager(logger *zap.Logger) *WorkerManager {
	return &WorkerManager{
		workers: make([]Worker, 0),
		logger:  logger,
	}
}

// Register adds a worker to be managed
func (m *WorkerManager) Register(worker Worker) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.workers = append(m.workers, worker)
	m.logger.Info("Worker registered",
		zap.String("worker_name", worker.Name()),
		zap.Int("total_workers", len(m.workers)))
}

// StartAll starts all registered workers
func (m *WorkerManager) StartAll(ctx context.Context) error {
	m.mu.Lock()
	if m.isRunning {
		m.mu.Unlock()
		return fmt.Errorf("workers already running")
	}

	m.ctx, m.cancel = context.WithCancel(ctx)
	m.isRunning = true
	m.mu.Unlock()

	m.logger.Info("Starting all workers", zap.Int("count", len(m.workers)))

	// Start all workers
	for _, worker := range m.workers {
		if err := worker.Start(m.ctx); err != nil {
			m.logger.Error("Failed to start worker",
				zap.String("worker_name", worker.Name()),
				zap.Error(err))
			// Continue starting other workers even if one fails
			continue
		}
		m.logger.Info("Worker started", zap.String("worker_name", worker.Name()))
	}

	return nil
}

// StopAll gracefully stops all workers
func (m *WorkerManager) StopAll() error {
	m.mu.Lock()
	if !m.isRunning {
		m.mu.Unlock()
		m.logger.Warn("Workers not running, nothing to stop")
		return nil
	}

	m.isRunning = false
	m.mu.Unlock()

	m.logger.Info("Stopping all workers", zap.Int("count", len(m.workers)))

	// Cancel context to signal all workers to stop
	if m.cancel != nil {
		m.cancel()
	}

	// Stop all workers
	var errors []error
	for _, worker := range m.workers {
		if err := worker.Stop(); err != nil {
			m.logger.Error("Failed to stop worker",
				zap.String("worker_name", worker.Name()),
				zap.Error(err))
			errors = append(errors, err)
		} else {
			m.logger.Info("Worker stopped", zap.String("worker_name", worker.Name()))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("failed to stop %d workers", len(errors))
	}

	m.logger.Info("All workers stopped successfully")
	return nil
}

// GetWorkerCount returns the number of registered workers
func (m *WorkerManager) GetWorkerCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.workers)
}

// IsRunning returns whether workers are running
func (m *WorkerManager) IsRunning() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.isRunning
}
