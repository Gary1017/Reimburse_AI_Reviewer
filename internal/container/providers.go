// Package container provides dependency injection and lifecycle management
// for the AI Reimbursement system following Clean Architecture principles.
package container

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/garyjia/ai-reimbursement/internal/application/dispatcher"
	"github.com/garyjia/ai-reimbursement/internal/application/port"
	"github.com/garyjia/ai-reimbursement/internal/application/service"
	"github.com/garyjia/ai-reimbursement/internal/application/workflow"
	"github.com/garyjia/ai-reimbursement/internal/domain/event"
	infraLark "github.com/garyjia/ai-reimbursement/internal/infrastructure/external/lark"
	"github.com/garyjia/ai-reimbursement/internal/infrastructure/external/openai"
	"github.com/garyjia/ai-reimbursement/internal/infrastructure/persistence/repository"
	"github.com/garyjia/ai-reimbursement/internal/infrastructure/persistence/sqlite"
	"github.com/garyjia/ai-reimbursement/internal/infrastructure/storage"
	"github.com/garyjia/ai-reimbursement/internal/infrastructure/worker"
	"github.com/garyjia/ai-reimbursement/pkg/database"
	"go.uber.org/zap"

	_ "github.com/mattn/go-sqlite3"
)

// DatabaseBundle holds database-related components.
type DatabaseBundle struct {
	SqlDB            *sql.DB
	TransactionMgr   *sqlite.DB
}

// LarkBundle holds all Lark-related components.
type LarkBundle struct {
	Client     *infraLark.SDKClient
	Adapter    port.LarkClient
	Downloader port.LarkAttachmentDownloader
	Messenger  port.LarkMessageSender
}

// StorageBundle holds storage-related components.
type StorageBundle struct {
	FileStorage   port.FileStorage
	FolderManager port.FolderManager
}

// ProvideDatabase creates database connection and transaction manager.
// Returns DatabaseBundle containing sql.DB and TransactionManager.
// Also runs any pending database migrations automatically.
func ProvideDatabase(cfg *DatabaseConfig, logger *zap.Logger) (*DatabaseBundle, error) {
	if cfg == nil {
		return nil, fmt.Errorf("database config is required")
	}
	if logger == nil {
		return nil, fmt.Errorf("logger is required")
	}

	// Open SQLite database with WAL mode and busy timeout
	sqlDB, err := sql.Open("sqlite3", cfg.Path+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Configure connection pool
	sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(cfg.ConnMaxLifetime)

	// Verify connection
	if err := sqlDB.Ping(); err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	// Run database migrations if migrations directory is configured
	if cfg.MigrationsDir != "" {
		// Create a database.DB wrapper for the migrator
		dbWrapper := &database.DB{DB: sqlDB}
		migrator := database.NewMigrator(dbWrapper, logger)

		if err := migrator.RunMigrations(cfg.MigrationsDir); err != nil {
			sqlDB.Close()
			return nil, fmt.Errorf("failed to run migrations: %w", err)
		}
	}

	// Create transaction manager wrapper
	db := sqlite.NewDB(sqlDB, logger)

	return &DatabaseBundle{
		SqlDB:          sqlDB,
		TransactionMgr: db,
	}, nil
}

// ProvideRepositories creates all repositories from a database connection.
// Returns RepositoryBundle containing all repository implementations.
func ProvideRepositories(sqlDB *sql.DB, logger *zap.Logger) (*RepositoryBundle, error) {
	if sqlDB == nil {
		return nil, fmt.Errorf("database connection is required")
	}
	if logger == nil {
		return nil, fmt.Errorf("logger is required")
	}

	return &RepositoryBundle{
		// Existing repositories
		Instance:     repository.NewInstanceRepository(sqlDB, logger),
		Item:         repository.NewItemRepository(sqlDB, logger),
		Attachment:   repository.NewAttachmentRepository(sqlDB, logger),
		History:      repository.NewHistoryRepository(sqlDB, logger),
		Invoice:      repository.NewInvoiceRepository(sqlDB, logger),
		Voucher:      repository.NewVoucherRepository(sqlDB, logger),
		Notification: repository.NewNotificationRepository(sqlDB, logger),

		// New repositories for schema refactoring
		InvoiceList:        repository.NewInvoiceListRepository(sqlDB, logger),
		InvoiceV2:          repository.NewInvoiceV2Repository(sqlDB, logger),
		Task:               repository.NewApprovalTaskRepository(sqlDB, logger),
		ReviewNotification: repository.NewReviewNotificationRepository(sqlDB, logger),
	}, nil
}

// ProvideLarkClients creates Lark client and its adapters.
// Returns LarkBundle containing Client, Adapter, Downloader, and Messenger.
func ProvideLarkClients(cfg *LarkConfig, storageCfg *StorageConfig, logger *zap.Logger) (*LarkBundle, error) {
	if cfg == nil {
		return nil, fmt.Errorf("lark config is required")
	}
	if storageCfg == nil {
		return nil, fmt.Errorf("storage config is required")
	}
	if logger == nil {
		return nil, fmt.Errorf("logger is required")
	}

	// Create base Lark SDK client
	larkCfg := infraLark.Config{
		AppID:        cfg.AppID,
		AppSecret:    cfg.AppSecret,
		ApprovalCode: cfg.ApprovalCode,
	}
	sdkClient := infraLark.NewSDKClient(larkCfg, logger)

	// Create port adapters
	adapter := infraLark.NewClient(sdkClient, logger)
	downloader := infraLark.NewDownloader(storageCfg.AttachmentDir, logger)
	messenger := infraLark.NewMessenger(sdkClient, logger)

	return &LarkBundle{
		Client:     sdkClient,
		Adapter:    adapter,
		Downloader: downloader,
		Messenger:  messenger,
	}, nil
}

// ProvideAIAuditor creates the AI auditor using OpenAI.
// Returns port.AIAuditor implementation.
func ProvideAIAuditor(cfg *OpenAIConfig, promptsPath string, logger *zap.Logger) (port.AIAuditor, error) {
	if cfg == nil {
		return nil, fmt.Errorf("openai config is required")
	}
	if logger == nil {
		return nil, fmt.Errorf("logger is required")
	}

	// Load prompts from YAML file
	prompts, err := openai.LoadPrompts(promptsPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load prompts: %w", err)
	}

	auditor := openai.NewAuditor(
		cfg.APIKey,
		cfg.Model,
		cfg.Policies,
		cfg.PriceDeviationThreshold,
		prompts,
		logger,
	)

	return auditor, nil
}

// ProvideStorage creates file storage and folder manager.
// Returns StorageBundle containing FileStorage and FolderManager.
func ProvideStorage(cfg *StorageConfig, logger *zap.Logger) (*StorageBundle, error) {
	if cfg == nil {
		return nil, fmt.Errorf("storage config is required")
	}
	if logger == nil {
		return nil, fmt.Errorf("logger is required")
	}

	fileStorage := storage.NewLocalFileStorage(cfg.AttachmentDir, logger)
	folderManager := storage.NewLocalFolderManager(cfg.AttachmentDir, logger)

	return &StorageBundle{
		FileStorage:   fileStorage,
		FolderManager: folderManager,
	}, nil
}

// ServiceDeps holds dependencies required for creating services.
type ServiceDeps struct {
	Repos      *RepositoryBundle
	TxManager  port.TransactionManager
	AIAuditor  port.AIAuditor
	LarkClient port.LarkClient
	Messenger  port.LarkMessageSender
	Logger     *zap.Logger
}

// ProvideServices creates all application services.
// Returns ServiceBundle containing all service implementations.
func ProvideServices(deps *ServiceDeps) (*ServiceBundle, error) {
	if deps == nil {
		return nil, fmt.Errorf("service dependencies are required")
	}
	if deps.Repos == nil {
		return nil, fmt.Errorf("repositories are required")
	}
	if deps.TxManager == nil {
		return nil, fmt.Errorf("transaction manager is required")
	}
	if deps.Logger == nil {
		return nil, fmt.Errorf("logger is required")
	}

	// Create logger adapter for services
	serviceLogger := &zapLoggerAdapter{logger: deps.Logger}

	return &ServiceBundle{
		// Existing services
		Approval: service.NewApprovalService(
			deps.Repos.Instance,
			deps.Repos.Item,
			deps.Repos.History,
			deps.TxManager,
			serviceLogger,
		),
		Audit: service.NewAuditService(
			deps.Repos.Instance,
			deps.Repos.Item,
			deps.Repos.Attachment,
			deps.Repos.Invoice,
			deps.AIAuditor,
			serviceLogger,
		),
		Voucher: service.NewVoucherService(
			deps.Repos.Instance,
			deps.Repos.Item,
			deps.Repos.Attachment,
			deps.Repos.Voucher,
			deps.Repos.Invoice,
			deps.TxManager,
			serviceLogger,
		),
		Notification: service.NewNotificationService(
			deps.Repos.Instance,
			deps.Repos.Notification,
			deps.LarkClient,
			deps.Messenger,
			deps.TxManager,
			serviceLogger,
		),

		// New services for schema refactoring
		InvoiceList: service.NewInvoiceListService(
			deps.Repos.InvoiceList,
			deps.Repos.InvoiceV2,
			deps.TxManager,
			serviceLogger,
		),
		Task: service.NewTaskService(
			deps.Repos.Task,
			deps.Repos.ReviewNotification,
			deps.TxManager,
			serviceLogger,
		),
		ReviewNotification: service.NewReviewNotificationService(
			deps.Repos.Instance,
			deps.Repos.Task,
			deps.Repos.ReviewNotification,
			deps.LarkClient,
			deps.Messenger,
			deps.TxManager,
			serviceLogger,
		),
	}, nil
}

// ProvideDispatcher creates the event dispatcher.
// Returns dispatcher.Dispatcher implementation.
func ProvideDispatcher(logger *zap.Logger) (dispatcher.Dispatcher, error) {
	if logger == nil {
		return nil, fmt.Errorf("logger is required")
	}

	// Create dispatcher logger adapter
	dispatcherLogger := &dispatcherLoggerAdapter{logger: logger}

	return dispatcher.NewDispatcher(
		dispatcher.WithLogger(dispatcherLogger),
	), nil
}

// WorkflowDeps holds dependencies required for creating the workflow engine.
type WorkflowDeps struct {
	Repos      *RepositoryBundle
	TxManager  port.TransactionManager
	Dispatcher dispatcher.Dispatcher
	Services   *ServiceBundle
	Logger     *zap.Logger
}

// ProvideWorkflowEngine creates the workflow engine and registers event handlers.
// Returns workflow.WorkflowEngine implementation.
func ProvideWorkflowEngine(deps *WorkflowDeps) (workflow.WorkflowEngine, error) {
	if deps == nil {
		return nil, fmt.Errorf("workflow dependencies are required")
	}
	if deps.Repos == nil {
		return nil, fmt.Errorf("repositories are required")
	}
	if deps.TxManager == nil {
		return nil, fmt.Errorf("transaction manager is required")
	}
	if deps.Dispatcher == nil {
		return nil, fmt.Errorf("dispatcher is required")
	}

	// Create workflow engine
	engine := workflow.NewEngine(
		deps.Repos.Instance,
		deps.Repos.History,
		deps.TxManager,
		workflow.WithDispatcher(deps.Dispatcher),
	)

	// Register instance creation handler using InstanceCreationService
	if deps.Services != nil && deps.Services.InstanceCreation != nil {
		svc := deps.Services.InstanceCreation
		deps.Dispatcher.SubscribeNamed(event.TypeInstanceCreated, "instance_creator", func(ctx context.Context, evt *event.Event) error {
			return svc.HandleInstanceCreated(ctx, evt.LarkInstanceID, evt.Payload)
		})
	}

	// Register workflow engine as event handler for all relevant event types
	deps.Dispatcher.SubscribeNamed(event.TypeInstanceCreated, "workflow_engine", engine.HandleEvent)
	deps.Dispatcher.SubscribeNamed(event.TypeInstanceApproved, "workflow_engine", engine.HandleEvent)
	deps.Dispatcher.SubscribeNamed(event.TypeInstanceRejected, "workflow_engine", engine.HandleEvent)
	deps.Dispatcher.SubscribeNamed(event.TypeAuditCompleted, "workflow_engine", engine.HandleEvent)
	deps.Dispatcher.SubscribeNamed(event.TypeVoucherGenerated, "workflow_engine", engine.HandleEvent)
	deps.Dispatcher.SubscribeNamed(event.TypeDataReady, "workflow_engine", engine.HandleEvent)

	// Register matching service handler for InvoiceExtracted events
	if deps.Services != nil && deps.Services.Matching != nil {
		matchSvc := deps.Services.Matching
		deps.Dispatcher.SubscribeNamed(event.TypeInvoiceExtracted, "matching_service", func(ctx context.Context, evt *event.Event) error {
			return matchSvc.MatchInstance(ctx, evt.InstanceID)
		})
	}

	// Register task event handler for review pipeline (two-path trigger)
	if deps.Services != nil && deps.Services.TaskEventHandler != nil {
		teh := deps.Services.TaskEventHandler
		deps.Dispatcher.SubscribeNamed(event.TypeTaskCreated, "task_event_handler", func(ctx context.Context, evt *event.Event) error {
			return teh.HandleTaskEvent(ctx, evt.LarkInstanceID, evt.Payload)
		})
		deps.Dispatcher.SubscribeNamed(event.TypeDataReady, "data_ready_review", func(ctx context.Context, evt *event.Event) error {
			return teh.HandleDataReady(ctx, evt.InstanceID)
		})
	}

	return engine, nil
}

// WorkerDeps holds dependencies required for creating workers.
type WorkerDeps struct {
	Repos         *RepositoryBundle
	LarkBundle    *LarkBundle
	StorageBundle *StorageBundle
	AIAuditor     port.AIAuditor
	Dispatcher    dispatcher.Dispatcher
	WorkerCfg     *WorkerConfig
	Logger        *zap.Logger
}

// ProvideWorkers creates and registers all background workers.
// Returns *worker.WorkerManager with all workers registered but not started.
func ProvideWorkers(deps *WorkerDeps) (*worker.WorkerManager, error) {
	if deps == nil {
		return nil, fmt.Errorf("worker dependencies are required")
	}
	if deps.Repos == nil {
		return nil, fmt.Errorf("repositories are required")
	}
	if deps.LarkBundle == nil {
		return nil, fmt.Errorf("lark bundle is required")
	}
	if deps.StorageBundle == nil {
		return nil, fmt.Errorf("storage bundle is required")
	}
	if deps.WorkerCfg == nil {
		return nil, fmt.Errorf("worker config is required")
	}
	if deps.Logger == nil {
		return nil, fmt.Errorf("logger is required")
	}

	// Create worker manager
	manager := worker.NewWorkerManager(deps.Logger)

	// Create download worker
	downloadCfg := worker.DownloadWorkerConfig{
		PollInterval:    deps.WorkerCfg.DownloadPollInterval,
		BatchSize:       deps.WorkerCfg.DownloadBatchSize,
		DownloadTimeout: deps.WorkerCfg.DownloadTimeout,
	}
	downloadWorker := worker.NewDownloadWorker(
		downloadCfg,
		deps.Repos.Attachment,
		deps.Repos.Item,
		deps.LarkBundle.Downloader,
		deps.StorageBundle.FileStorage,
		deps.StorageBundle.FolderManager,
		deps.Logger,
	)
	manager.Register(downloadWorker)

	// Create invoice worker
	invoiceCfg := worker.InvoiceWorkerConfig{
		PollInterval:   deps.WorkerCfg.InvoicePollInterval,
		BatchSize:      deps.WorkerCfg.InvoiceBatchSize,
		ProcessTimeout: deps.WorkerCfg.InvoiceProcessTimeout,
	}
	invoiceWorker := worker.NewInvoiceWorker(
		invoiceCfg,
		deps.Repos.Attachment,
		deps.Repos.Item,
		deps.Repos.InvoiceV2,
		deps.StorageBundle.FileStorage,
		deps.AIAuditor,
		deps.Dispatcher,
		deps.Logger,
	)
	manager.Register(invoiceWorker)

	return manager, nil
}

