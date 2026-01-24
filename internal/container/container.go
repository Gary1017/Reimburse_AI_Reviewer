package container

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/garyjia/ai-reimbursement/internal/application/dispatcher"
	"github.com/garyjia/ai-reimbursement/internal/application/port"
	"github.com/garyjia/ai-reimbursement/internal/application/service"
	"github.com/garyjia/ai-reimbursement/internal/application/workflow"
	"github.com/garyjia/ai-reimbursement/internal/infrastructure/persistence/sqlite"
	"github.com/garyjia/ai-reimbursement/internal/infrastructure/worker"
	"github.com/garyjia/ai-reimbursement/internal/lark"
	"go.uber.org/zap"
)

// Container manages all application dependencies and lifecycle.
// It follows Clean Architecture principles with ordered initialization
// and reverse-order teardown.
type Container struct {
	config *Config
	logger *zap.Logger

	// Infrastructure - Data
	sqlDB        *sql.DB
	db           *sqlite.DB
	repositories *RepositoryBundle

	// Infrastructure - External
	larkClient     *lark.Client
	larkAdapter    port.LarkClient
	larkDownloader port.LarkAttachmentDownloader
	larkMessenger  port.LarkMessageSender
	aiAuditor      port.AIAuditor

	// Infrastructure - Storage
	fileStorage   port.FileStorage
	folderManager port.FolderManager

	// Application
	dispatcher dispatcher.Dispatcher
	workflow   workflow.WorkflowEngine
	services   *ServiceBundle

	// Workers
	workers *worker.WorkerManager

	// Lifecycle
	mu      sync.RWMutex
	ctx     context.Context
	cancel  context.CancelFunc
	ready   atomic.Bool
	closed  atomic.Bool
}

// RepositoryBundle groups all repositories for convenient access.
type RepositoryBundle struct {
	Instance     port.InstanceRepository
	Item         port.ItemRepository
	Attachment   port.AttachmentRepository
	History      port.HistoryRepository
	Invoice      port.InvoiceRepository
	Voucher      port.VoucherRepository
	Notification port.NotificationRepository
}

// ServiceBundle groups all application services.
type ServiceBundle struct {
	Approval     service.ApprovalService
	Audit        service.AuditService
	Voucher      service.VoucherService
	Notification service.NotificationService
}

// HealthStatus represents the health of all components.
type HealthStatus struct {
	Overall    bool                      `json:"overall"`
	Components map[string]ComponentHealth `json:"components"`
}

// ComponentHealth represents health of a single component.
type ComponentHealth struct {
	Healthy bool   `json:"healthy"`
	Message string `json:"message,omitempty"`
}

// NewContainer creates a new container from configuration.
// It does not initialize components - call Start() to initialize.
func NewContainer(cfg *Config, logger *zap.Logger) (*Container, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is required")
	}
	if logger == nil {
		return nil, fmt.Errorf("logger is required")
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	return &Container{
		config: cfg,
		logger: logger,
	}, nil
}

// Start initializes all components and begins processing.
// Components are initialized in dependency order:
// 1. Database and repositories
// 2. External clients (Lark, OpenAI)
// 3. Storage
// 4. Application services
// 5. Event dispatcher and workflow engine
// 6. Workers
func (c *Container) Start(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed.Load() {
		return fmt.Errorf("container has been closed")
	}

	if c.ready.Load() {
		return fmt.Errorf("container already started")
	}

	c.ctx, c.cancel = context.WithCancel(ctx)
	c.logger.Info("Starting container initialization")

	// Step 1: Initialize database and repositories
	if err := c.initDatabase(); err != nil {
		return fmt.Errorf("failed to initialize database: %w", err)
	}
	c.logger.Info("Database initialized")

	// Step 2: Initialize external clients
	if err := c.initExternalClients(); err != nil {
		return fmt.Errorf("failed to initialize external clients: %w", err)
	}
	c.logger.Info("External clients initialized")

	// Step 3: Initialize storage
	if err := c.initStorage(); err != nil {
		return fmt.Errorf("failed to initialize storage: %w", err)
	}
	c.logger.Info("Storage initialized")

	// Step 4: Initialize application services
	if err := c.initServices(); err != nil {
		return fmt.Errorf("failed to initialize services: %w", err)
	}
	c.logger.Info("Application services initialized")

	// Step 5: Initialize dispatcher and workflow engine
	if err := c.initDispatcherAndWorkflow(); err != nil {
		return fmt.Errorf("failed to initialize dispatcher and workflow: %w", err)
	}
	c.logger.Info("Dispatcher and workflow engine initialized")

	// Step 6: Initialize and start workers
	if err := c.initWorkers(); err != nil {
		return fmt.Errorf("failed to initialize workers: %w", err)
	}
	c.logger.Info("Workers initialized and started")

	c.ready.Store(true)
	c.logger.Info("Container started successfully")

	return nil
}

// Close gracefully shuts down all components in reverse order.
func (c *Container) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed.Load() {
		return fmt.Errorf("container already closed")
	}

	c.logger.Info("Closing container")

	var errs []error

	// Cancel context to signal all goroutines
	if c.cancel != nil {
		c.cancel()
	}

	// Step 1: Stop workers (reverse of step 6)
	if c.workers != nil {
		if err := c.workers.StopAll(); err != nil {
			c.logger.Error("Failed to stop workers", zap.Error(err))
			errs = append(errs, fmt.Errorf("stop workers: %w", err))
		} else {
			c.logger.Info("Workers stopped")
		}
	}

	// Step 2: Close dispatcher (reverse of step 5)
	if c.dispatcher != nil {
		if err := c.dispatcher.Close(); err != nil {
			c.logger.Error("Failed to close dispatcher", zap.Error(err))
			errs = append(errs, fmt.Errorf("close dispatcher: %w", err))
		} else {
			c.logger.Info("Dispatcher closed")
		}
	}

	// Step 3: Services don't need explicit cleanup (reverse of step 4)
	c.logger.Info("Services cleaned up")

	// Step 4: Storage doesn't need explicit cleanup (reverse of step 3)
	c.logger.Info("Storage cleaned up")

	// Step 5: External clients don't need explicit cleanup (reverse of step 2)
	c.logger.Info("External clients cleaned up")

	// Step 6: Close database (reverse of step 1)
	if c.sqlDB != nil {
		if err := c.sqlDB.Close(); err != nil {
			c.logger.Error("Failed to close database", zap.Error(err))
			errs = append(errs, fmt.Errorf("close database: %w", err))
		} else {
			c.logger.Info("Database closed")
		}
	}

	c.closed.Store(true)
	c.ready.Store(false)

	if len(errs) > 0 {
		c.logger.Error("Container closed with errors", zap.Int("error_count", len(errs)))
		return fmt.Errorf("container closed with %d errors", len(errs))
	}

	c.logger.Info("Container closed successfully")
	return nil
}

// Ready returns true when all components are initialized.
func (c *Container) Ready() bool {
	return c.ready.Load()
}

// Health returns health status of all components.
func (c *Container) Health() *HealthStatus {
	status := &HealthStatus{
		Overall:    true,
		Components: make(map[string]ComponentHealth),
	}

	// Check database
	if c.sqlDB != nil {
		if err := c.sqlDB.Ping(); err != nil {
			status.Components["database"] = ComponentHealth{
				Healthy: false,
				Message: fmt.Sprintf("ping failed: %v", err),
			}
			status.Overall = false
		} else {
			status.Components["database"] = ComponentHealth{Healthy: true}
		}
	} else {
		status.Components["database"] = ComponentHealth{
			Healthy: false,
			Message: "not initialized",
		}
		status.Overall = false
	}

	// Check workers
	if c.workers != nil {
		status.Components["workers"] = ComponentHealth{
			Healthy: c.workers.IsRunning(),
			Message: fmt.Sprintf("worker count: %d", c.workers.GetWorkerCount()),
		}
		if !c.workers.IsRunning() {
			status.Overall = false
		}
	} else {
		status.Components["workers"] = ComponentHealth{
			Healthy: false,
			Message: "not initialized",
		}
		status.Overall = false
	}

	// Check dispatcher
	if c.dispatcher != nil {
		status.Components["dispatcher"] = ComponentHealth{Healthy: true}
	} else {
		status.Components["dispatcher"] = ComponentHealth{
			Healthy: false,
			Message: "not initialized",
		}
		status.Overall = false
	}

	// Check repositories
	if c.repositories != nil {
		status.Components["repositories"] = ComponentHealth{Healthy: true}
	} else {
		status.Components["repositories"] = ComponentHealth{
			Healthy: false,
			Message: "not initialized",
		}
		status.Overall = false
	}

	return status
}

// initDatabase initializes the database and all repositories using providers.
func (c *Container) initDatabase() error {
	// Use provider to create database bundle
	dbBundle, err := ProvideDatabase(&c.config.Database, c.logger)
	if err != nil {
		return err
	}

	c.sqlDB = dbBundle.SqlDB
	c.db = dbBundle.TransactionMgr

	// Use provider to create repositories
	repos, err := ProvideRepositories(c.sqlDB, c.logger)
	if err != nil {
		c.sqlDB.Close()
		return err
	}

	c.repositories = repos
	return nil
}

// initExternalClients initializes Lark and OpenAI clients using providers.
func (c *Container) initExternalClients() error {
	// Use provider to create Lark clients bundle
	larkBundle, err := ProvideLarkClients(&c.config.Lark, &c.config.Storage, c.logger)
	if err != nil {
		return err
	}

	c.larkClient = larkBundle.Client
	c.larkAdapter = larkBundle.Adapter
	c.larkDownloader = larkBundle.Downloader
	c.larkMessenger = larkBundle.Messenger

	// Use provider to create AI auditor
	auditor, err := ProvideAIAuditor(&c.config.OpenAI, c.logger)
	if err != nil {
		return err
	}
	c.aiAuditor = auditor

	return nil
}

// initStorage initializes file storage and folder manager using providers.
func (c *Container) initStorage() error {
	// Use provider to create storage bundle
	storageBundle, err := ProvideStorage(&c.config.Storage, c.logger)
	if err != nil {
		return err
	}

	c.fileStorage = storageBundle.FileStorage
	c.folderManager = storageBundle.FolderManager
	return nil
}

// initServices initializes all application services using providers.
func (c *Container) initServices() error {
	// Use provider to create services bundle
	services, err := ProvideServices(&ServiceDeps{
		Repos:      c.repositories,
		TxManager:  c.db,
		AIAuditor:  c.aiAuditor,
		LarkClient: c.larkAdapter,
		Messenger:  c.larkMessenger,
		Logger:     c.logger,
	})
	if err != nil {
		return err
	}

	c.services = services
	return nil
}

// initDispatcherAndWorkflow initializes the event dispatcher and workflow engine using providers.
func (c *Container) initDispatcherAndWorkflow() error {
	// Use provider to create dispatcher
	disp, err := ProvideDispatcher(c.logger)
	if err != nil {
		return err
	}
	c.dispatcher = disp

	// Use provider to create workflow engine (also registers event handlers)
	engine, err := ProvideWorkflowEngine(&WorkflowDeps{
		Repos:      c.repositories,
		TxManager:  c.db,
		Dispatcher: c.dispatcher,
		Logger:     c.logger,
	})
	if err != nil {
		return err
	}
	c.workflow = engine

	return nil
}

// initWorkers initializes and starts all background workers using providers.
func (c *Container) initWorkers() error {
	// Create storage bundle for workers
	storageBundle := &StorageBundle{
		FileStorage:   c.fileStorage,
		FolderManager: c.folderManager,
	}

	// Create lark bundle for workers
	larkBundle := &LarkBundle{
		Client:     c.larkClient,
		Adapter:    c.larkAdapter,
		Downloader: c.larkDownloader,
		Messenger:  c.larkMessenger,
	}

	// Use provider to create workers
	workers, err := ProvideWorkers(&WorkerDeps{
		Repos:         c.repositories,
		LarkBundle:    larkBundle,
		StorageBundle: storageBundle,
		AIAuditor:     c.aiAuditor,
		WorkerCfg:     &c.config.Worker,
		Logger:        c.logger,
	})
	if err != nil {
		return err
	}
	c.workers = workers

	// Start all workers
	if err := c.workers.StartAll(c.ctx); err != nil {
		return fmt.Errorf("failed to start workers: %w", err)
	}

	return nil
}

// Getters for accessing container components

// DB returns the transaction manager.
func (c *Container) DB() port.TransactionManager {
	return c.db
}

// Repositories returns all repositories.
func (c *Container) Repositories() *RepositoryBundle {
	return c.repositories
}

// LarkClient returns the Lark client adapter.
func (c *Container) LarkClient() port.LarkClient {
	return c.larkAdapter
}

// LarkDownloader returns the Lark attachment downloader.
func (c *Container) LarkDownloader() port.LarkAttachmentDownloader {
	return c.larkDownloader
}

// LarkMessenger returns the Lark message sender.
func (c *Container) LarkMessenger() port.LarkMessageSender {
	return c.larkMessenger
}

// AIAuditor returns the AI auditor.
func (c *Container) AIAuditor() port.AIAuditor {
	return c.aiAuditor
}

// FileStorage returns the file storage.
func (c *Container) FileStorage() port.FileStorage {
	return c.fileStorage
}

// FolderManager returns the folder manager.
func (c *Container) FolderManager() port.FolderManager {
	return c.folderManager
}

// Dispatcher returns the event dispatcher.
func (c *Container) Dispatcher() dispatcher.Dispatcher {
	return c.dispatcher
}

// WorkflowEngine returns the workflow engine.
func (c *Container) WorkflowEngine() workflow.WorkflowEngine {
	return c.workflow
}

// Services returns all application services.
func (c *Container) Services() *ServiceBundle {
	return c.services
}

// Workers returns the worker manager.
func (c *Container) Workers() *worker.WorkerManager {
	return c.workers
}

// RawLarkClient returns the underlying Lark client for WebSocket setup.
func (c *Container) RawLarkClient() *lark.Client {
	return c.larkClient
}

// Logger returns the container's logger.
func (c *Container) Logger() *zap.Logger {
	return c.logger
}

// Config returns the container's configuration.
func (c *Container) Config() *Config {
	return c.config
}

// zapLoggerAdapter adapts zap.Logger to the service.Logger interface.
type zapLoggerAdapter struct {
	logger *zap.Logger
}

func (a *zapLoggerAdapter) Info(msg string, keysAndValues ...interface{}) {
	fields := convertToZapFields(keysAndValues...)
	a.logger.Info(msg, fields...)
}

func (a *zapLoggerAdapter) Error(msg string, keysAndValues ...interface{}) {
	fields := convertToZapFields(keysAndValues...)
	a.logger.Error(msg, fields...)
}

// dispatcherLoggerAdapter adapts zap.Logger to the dispatcher.Logger interface.
type dispatcherLoggerAdapter struct {
	logger *zap.Logger
}

func (a *dispatcherLoggerAdapter) Info(msg string, keysAndValues ...interface{}) {
	fields := convertToZapFields(keysAndValues...)
	a.logger.Info(msg, fields...)
}

func (a *dispatcherLoggerAdapter) Error(msg string, keysAndValues ...interface{}) {
	fields := convertToZapFields(keysAndValues...)
	a.logger.Error(msg, fields...)
}

// convertToZapFields converts key-value pairs to zap fields.
func convertToZapFields(keysAndValues ...interface{}) []zap.Field {
	fields := make([]zap.Field, 0, len(keysAndValues)/2)
	for i := 0; i+1 < len(keysAndValues); i += 2 {
		key, ok := keysAndValues[i].(string)
		if !ok {
			continue
		}
		fields = append(fields, zap.Any(key, keysAndValues[i+1]))
	}
	return fields
}
