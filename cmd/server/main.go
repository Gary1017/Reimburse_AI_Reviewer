package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/garyjia/ai-reimbursement/internal/ai"
	"github.com/garyjia/ai-reimbursement/internal/config"
	"github.com/garyjia/ai-reimbursement/internal/invoice"
	"github.com/garyjia/ai-reimbursement/internal/lark"
	"github.com/garyjia/ai-reimbursement/internal/notification"
	"github.com/garyjia/ai-reimbursement/internal/repository"
	"github.com/garyjia/ai-reimbursement/internal/storage"
	"github.com/garyjia/ai-reimbursement/internal/voucher"
	"github.com/garyjia/ai-reimbursement/internal/worker"
	"github.com/garyjia/ai-reimbursement/internal/workflow"
	"github.com/garyjia/ai-reimbursement/pkg/database"
	"github.com/garyjia/ai-reimbursement/pkg/utils"
	"github.com/gin-gonic/gin"
	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
	"github.com/larksuite/oapi-sdk-go/v3/event/dispatcher"
	larkws "github.com/larksuite/oapi-sdk-go/v3/ws"
	"go.uber.org/zap"
)

func main() {
	// Find config file - try current directory and parent directories
	configPath := "configs/config.yaml"

	// If not found in current dir, try parent directories
	if _, err := os.Stat(configPath); err != nil {
		// Try going up one directory (for running from cmd/server)
		if _, err := os.Stat("../../configs/config.yaml"); err == nil {
			// Change to project root first
			if err := os.Chdir("../../"); err != nil {
				log.Fatalf("Could not change to project root: %v", err)
			}
			configPath = "configs/config.yaml"
		} else {
			// If still not found, try one more level up
			if err := os.Chdir("../../../"); err == nil {
				if _, err := os.Stat("configs/config.yaml"); err == nil {
					configPath = "configs/config.yaml"
				}
			}
		}
	}

	// Load configuration
	cfg, err := config.Load(configPath)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Initialize logger
	logger, err := utils.NewLogger(utils.LoggerConfig{
		Level:      cfg.Logger.Level,
		OutputPath: cfg.Logger.OutputPath,
		Format:     cfg.Logger.Format,
	})
	if err != nil {
		log.Fatalf("Failed to initialize logger: %v", err)
	}
	defer logger.Sync()

	logger.Info("Starting AI Reimbursement System",
		zap.String("server", fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)),
		zap.String("database", cfg.Database.Path))

	// Initialize database
	db, err := database.New(database.Config{
		Path:            cfg.Database.Path,
		MaxOpenConns:    cfg.Database.MaxOpenConns,
		MaxIdleConns:    cfg.Database.MaxIdleConns,
		ConnMaxLifetime: cfg.Database.ConnMaxLifetime,
	}, logger)
	if err != nil {
		logger.Fatal("Failed to initialize database", zap.Error(err))
	}
	defer db.Close()

	// Run migrations
	migrator := database.NewMigrator(db, logger)
	if err := migrator.RunMigrations(cfg.Database.MigrationsDir); err != nil {
		logger.Fatal("Failed to run migrations", zap.Error(err))
	}

	// Initialize repositories
	instanceRepo := repository.NewInstanceRepository(db.DB, logger)
	historyRepo := repository.NewHistoryRepository(db.DB, logger)
	itemRepo := repository.NewReimbursementItemRepository(db.DB, logger)
	attachmentRepo := repository.NewAttachmentRepository(db.DB, logger)
	invoiceRepo := repository.NewInvoiceRepository(db.DB, logger)
	notificationRepo := repository.NewNotificationRepository(db.DB, logger) // ARCH-012

	// Initialize Lark client and handlers
	larkClient := lark.NewClient(lark.Config{
		AppID:        cfg.Lark.AppID,
		AppSecret:    cfg.Lark.AppSecret,
		ApprovalCode: cfg.Lark.ApprovalCode,
	}, logger)

	approvalAPI := lark.NewApprovalAPI(larkClient, logger)
	approvalBotAPI := lark.NewApprovalBotAPI(larkClient, logger) // ARCH-012
	attachmentHandler := lark.NewAttachmentHandler(logger, cfg.Voucher.AttachmentDir)

	// Initialize workflow engine
	engine := workflow.NewEngine(
		db,
		instanceRepo,
		historyRepo,
		itemRepo,
		attachmentRepo,
		approvalAPI,
		attachmentHandler,
		logger,
	)

	// Initialize Lark WebSocket event processor for SDK-based event subscription
	// This is the recommended way for enterprise self-built apps (no webhook URL needed)
	eventProcessor := lark.NewEventProcessor(cfg.Lark.ApprovalCode, engine, logger)
	eventHandler := dispatcher.NewEventDispatcher("", "").
		OnCustomizedEvent("approval_instance", eventProcessor.HandleCustomizedEvent)

	// Create WebSocket client for event subscription
	wsClient := larkws.NewClient(cfg.Lark.AppID, cfg.Lark.AppSecret,
		larkws.WithEventHandler(eventHandler),
		larkws.WithLogLevel(larkcore.LogLevelInfo),
	)

	// Start WebSocket client in background goroutine
	go func() {
		logger.Info("Starting Lark WebSocket client for event subscription")
		if err := wsClient.Start(context.Background()); err != nil {
			logger.Fatal("Failed to start Lark WebSocket client", zap.Error(err))
		}
	}()
	logger.Info("Lark WebSocket client started - events will be received via SDK")

	// Initialize async download worker (ARCH-007)
	downloadWorker := worker.NewAsyncDownloadWorker(
		attachmentRepo,
		attachmentHandler,
		larkClient,
		logger,
	)

	// Initialize PDF reader and invoice auditor for AI processing (ARCH-011)
	// Use gpt-4o for Vision API (supports image input)
	visionModel := "gpt-4o"
	pdfReader := invoice.NewPDFReader(cfg.OpenAI.APIKey, visionModel, logger)
	invoiceAuditor := ai.NewInvoiceAuditor(
		cfg.OpenAI.APIKey,
		cfg.OpenAI.Model,
		cfg.Voucher.CompanyName,
		cfg.Voucher.CompanyTaxID,
		cfg.Voucher.PriceDeviation,
		logger,
	)

	// Initialize invoice processor worker (ARCH-011-B)
	invoiceProcessor := worker.NewInvoiceProcessor(
		attachmentRepo,
		itemRepo,
		invoiceRepo,
		pdfReader,
		invoiceAuditor,
		cfg.Voucher.AttachmentDir,
		logger,
	)

	// Initialize audit notification components (ARCH-012)
	auditAggregator := notification.NewAuditAggregator(logger)
	auditNotifier := notification.NewAuditNotifier(
		attachmentRepo,
		instanceRepo,
		notificationRepo,
		approvalAPI,
		approvalBotAPI,
		auditAggregator,
		cfg.Lark.ApprovalCode,
		logger,
	)
	invoiceProcessor.SetAuditNotifier(auditNotifier)
	logger.Info("AuditNotifier initialized and wired to InvoiceProcessor")

	// Initialize form packager components (ARCH-013)
	subjectMapper := voucher.NewAccountingSubjectMapper()
	folderManager := storage.NewFolderManager(cfg.Voucher.AttachmentDir, logger)
	formDataAggregator := voucher.NewFormDataAggregator(
		instanceRepo,
		itemRepo,
		attachmentRepo,
		subjectMapper,
		logger,
	)
	formFiller, err := voucher.NewReimbursementFormFiller(
		"templates/报销单模板.xlsx",
		logger,
	)
	if err != nil {
		logger.Warn("Failed to initialize FormFiller, form generation disabled", zap.Error(err))
		formFiller = nil
	}
	formPackager := voucher.NewFormPackager(
		formFiller,
		formDataAggregator,
		folderManager,
		attachmentRepo,
		instanceRepo,
		logger,
	)
	invoiceProcessor.SetFormPackager(formPackager)
	downloadWorker.SetFormPackager(formPackager)
	logger.Info("FormPackager initialized and wired to InvoiceProcessor and AsyncDownloadWorker")

	// Initialize status poller (fallback when webhooks unavailable)
	statusPoller := worker.NewStatusPoller(
		instanceRepo,
		approvalAPI,
		engine,
		logger,
	)

	// Start async download worker
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := downloadWorker.Start(ctx); err != nil {
		logger.Fatal("Failed to start download worker", zap.Error(err))
	}
	logger.Info("AsyncDownloadWorker started")

	// Start invoice processor worker (ARCH-011-B)
	if err := invoiceProcessor.Start(ctx); err != nil {
		logger.Fatal("Failed to start invoice processor", zap.Error(err))
	}
	logger.Info("InvoiceProcessor started (will process downloaded attachments)")

	// Start status poller (polls Lark API for status changes)
	if err := statusPoller.Start(ctx); err != nil {
		logger.Warn("Failed to start status poller", zap.Error(err))
	} else {
		logger.Info("StatusPoller started (will poll for status changes)")
	}

	// Initialize HTTP server
	router := gin.Default()

	// Add middleware to log all incoming requests
	router.Use(func(c *gin.Context) {
		logger.Info("Incoming HTTP request",
			zap.String("method", c.Request.Method),
			zap.String("path", c.Request.URL.Path),
			zap.String("remote_addr", c.Request.RemoteAddr),
			zap.String("user_agent", c.Request.UserAgent()))
		c.Next()
	})

	// Add catch-all route for debugging
	router.NoRoute(func(c *gin.Context) {
		logger.Warn("No route found for request",
			zap.String("method", c.Request.Method),
			zap.String("path", c.Request.URL.Path))
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Route not found",
			"path":  c.Request.URL.Path,
		})
	})

	// Health check endpoint
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":    "ok",
			"timestamp": time.Now(),
		})
	})

	// Worker status endpoint
	router.GET("/health/worker", func(c *gin.Context) {
		status := downloadWorker.GetStatus()
		c.JSON(http.StatusOK, status)
	})

	// Invoice processor status endpoint (ARCH-011)
	router.GET("/health/invoice-processor", func(c *gin.Context) {
		status := invoiceProcessor.GetStatus()
		c.JSON(http.StatusOK, status)
	})

	// Start HTTP server
	srv := &http.Server{
		Addr:         fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port),
		Handler:      router,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
	}

	go func() {
		logger.Info("Starting HTTP server", zap.String("address", srv.Addr))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("HTTP server error", zap.Error(err))
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down server...")

	// Stop workers
	downloadWorker.Stop()
	invoiceProcessor.Stop()
	statusPoller.Stop()

	// Shutdown HTTP server
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("Server shutdown error", zap.Error(err))
	}

	logger.Info("Server stopped")
}
