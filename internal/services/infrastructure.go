package services

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/garyjia/ai-reimbursement/internal/lark"
	"github.com/garyjia/ai-reimbursement/internal/repository"
	"github.com/garyjia/ai-reimbursement/internal/workflow"
	"github.com/garyjia/ai-reimbursement/pkg/database"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Repositories holds all data access layer repositories
type Repositories struct {
	Instance     *repository.InstanceRepository
	History      *repository.HistoryRepository
	Item         *repository.ReimbursementItemRepository
	Attachment   *repository.AttachmentRepository
	Invoice      *repository.InvoiceRepository
	Notification *repository.NotificationRepository
}

// Infrastructure holds all singleton foundational components
type Infrastructure struct {
	Database     *database.DB
	Repositories *Repositories
	LarkClient   *lark.Client
	Engine       *workflow.Engine
	HTTPServer   *http.Server
	Router       *gin.Engine

	logger *zap.Logger
}

// InfrastructureConfig holds configuration for infrastructure initialization
type InfrastructureConfig struct {
	DatabasePath        string
	MaxOpenConns        int
	MaxIdleConns        int
	ConnMaxLifetime     time.Duration
	MigrationsDir       string
	LarkAppID           string
	LarkAppSecret       string
	LarkApprovalCode    string
	ServerHost          string
	ServerPort          int
	ServerReadTimeout   time.Duration
	ServerWriteTimeout  time.Duration
}

// NewInfrastructure creates and initializes all infrastructure components
func NewInfrastructure(cfg InfrastructureConfig, logger *zap.Logger) (*Infrastructure, error) {
	infra := &Infrastructure{
		logger: logger,
	}

	// Initialize database
	db, err := database.New(database.Config{
		Path:            cfg.DatabasePath,
		MaxOpenConns:    cfg.MaxOpenConns,
		MaxIdleConns:    cfg.MaxIdleConns,
		ConnMaxLifetime: cfg.ConnMaxLifetime,
	}, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize database: %w", err)
	}
	infra.Database = db

	// Run migrations
	migrator := database.NewMigrator(db, logger)
	if err := migrator.RunMigrations(cfg.MigrationsDir); err != nil {
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	// Initialize repositories
	infra.Repositories = &Repositories{
		Instance:     repository.NewInstanceRepository(db.DB, logger),
		History:      repository.NewHistoryRepository(db.DB, logger),
		Item:         repository.NewReimbursementItemRepository(db.DB, logger),
		Attachment:   repository.NewAttachmentRepository(db.DB, logger),
		Invoice:      repository.NewInvoiceRepository(db.DB, logger),
		Notification: repository.NewNotificationRepository(db.DB, logger),
	}

	// Initialize Lark client
	infra.LarkClient = lark.NewClient(lark.Config{
		AppID:        cfg.LarkAppID,
		AppSecret:    cfg.LarkAppSecret,
		ApprovalCode: cfg.LarkApprovalCode,
	}, logger)

	// Initialize HTTP router
	infra.Router = gin.Default()

	// Create HTTP server (not started yet)
	infra.HTTPServer = &http.Server{
		Addr:         fmt.Sprintf("%s:%d", cfg.ServerHost, cfg.ServerPort),
		Handler:      infra.Router,
		ReadTimeout:  cfg.ServerReadTimeout,
		WriteTimeout: cfg.ServerWriteTimeout,
	}

	logger.Info("Infrastructure initialized",
		zap.String("database", cfg.DatabasePath),
		zap.String("server_addr", infra.HTTPServer.Addr))

	return infra, nil
}

// InitializeWorkflowEngine creates the workflow engine after infrastructure is ready
func (i *Infrastructure) InitializeWorkflowEngine(attachmentHandler *lark.AttachmentHandler, approvalAPI *lark.ApprovalAPI) {
	i.Engine = workflow.NewEngine(
		i.Database,
		i.Repositories.Instance,
		i.Repositories.History,
		i.Repositories.Item,
		i.Repositories.Attachment,
		approvalAPI,
		attachmentHandler,
		i.logger,
	)
	i.logger.Info("Workflow engine initialized")
}

// StartHTTPServer starts the HTTP server in a goroutine
func (i *Infrastructure) StartHTTPServer() {
	go func() {
		i.logger.Info("Starting HTTP server", zap.String("address", i.HTTPServer.Addr))
		if err := i.HTTPServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			i.logger.Fatal("HTTP server error", zap.Error(err))
		}
	}()
}

// Shutdown gracefully shuts down all infrastructure components
func (i *Infrastructure) Shutdown(ctx context.Context) error {
	i.logger.Info("Shutting down infrastructure...")

	// Shutdown HTTP server
	if err := i.HTTPServer.Shutdown(ctx); err != nil {
		i.logger.Error("HTTP server shutdown error", zap.Error(err))
		return err
	}

	// Close database
	if err := i.Database.Close(); err != nil {
		i.logger.Error("Database close error", zap.Error(err))
		return err
	}

	i.logger.Info("Infrastructure shutdown complete")
	return nil
}
