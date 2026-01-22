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

	"github.com/garyjia/ai-reimbursement/internal/config"
	"github.com/garyjia/ai-reimbursement/internal/services"
	"github.com/garyjia/ai-reimbursement/internal/worker"
	"github.com/garyjia/ai-reimbursement/pkg/utils"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func main() {
	// ========================================================================
	// STEP 1: Configuration & Logger
	// ========================================================================
	configPath := findConfigFile()
	cfg, err := config.Load(configPath)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

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

	// ========================================================================
	// STEP 2: Infrastructure Layer (Databases, Repositories, Lark, HTTP)
	// ========================================================================
	infrastructure, err := services.NewInfrastructure(services.InfrastructureConfig{
		DatabasePath:       cfg.Database.Path,
		MaxOpenConns:       cfg.Database.MaxOpenConns,
		MaxIdleConns:       cfg.Database.MaxIdleConns,
		ConnMaxLifetime:    cfg.Database.ConnMaxLifetime,
		MigrationsDir:      cfg.Database.MigrationsDir,
		LarkAppID:          cfg.Lark.AppID,
		LarkAppSecret:      cfg.Lark.AppSecret,
		LarkApprovalCode:   cfg.Lark.ApprovalCode,
		ServerHost:         cfg.Server.Host,
		ServerPort:         cfg.Server.Port,
		ServerReadTimeout:  cfg.Server.ReadTimeout,
		ServerWriteTimeout: cfg.Server.WriteTimeout,
	}, logger)
	if err != nil {
		logger.Fatal("Failed to initialize infrastructure", zap.Error(err))
	}

	// ========================================================================
	// STEP 3: Service Layer (AI, Notifications, Forms, Storage)
	// ========================================================================
	serviceContainer, err := services.NewContainer(services.ServiceConfig{
		OpenAIAPIKey:     cfg.OpenAI.APIKey,
		OpenAIModel:      cfg.OpenAI.Model,
		CompanyName:      cfg.Voucher.CompanyName,
		CompanyTaxID:     cfg.Voucher.CompanyTaxID,
		PriceDeviation:   cfg.Voucher.PriceDeviation,
		AttachmentDir:    cfg.Voucher.AttachmentDir,
		FormTemplatePath: "templates/报销单模板.xlsx",
		LarkApprovalCode: cfg.Lark.ApprovalCode,
	}, infrastructure, logger)
	if err != nil {
		logger.Fatal("Failed to initialize service container", zap.Error(err))
	}

	// Initialize workflow engine (depends on both infrastructure and services)
	infrastructure.InitializeWorkflowEngine(
		serviceContainer.AttachmentHandler,
		serviceContainer.ApprovalAPI,
	)

	// ========================================================================
	// STEP 4: Worker Layer (Background Processing)
	// ========================================================================
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create workers
	downloadWorker := worker.NewAsyncDownloadWorker(
		infrastructure.Repositories.Attachment,
		serviceContainer.AttachmentHandler,
		infrastructure.LarkClient,
		logger,
	)
	downloadWorker.SetFormPackager(serviceContainer.FormPackager)
	downloadWorker.SetFolderManager(serviceContainer.FolderManager)
	downloadWorker.SetFileStorage(serviceContainer.FileStorage)

	invoiceProcessor := worker.NewInvoiceProcessor(
		infrastructure.Repositories.Attachment,
		infrastructure.Repositories.Item,
		infrastructure.Repositories.Invoice,
		serviceContainer.PDFReader,
		serviceContainer.InvoiceAuditor,
		cfg.Voucher.AttachmentDir,
		logger,
	)
	invoiceProcessor.SetAuditNotifier(serviceContainer.AuditNotifier)
	invoiceProcessor.SetFormPackager(serviceContainer.FormPackager)

	statusPoller := worker.NewStatusPoller(
		infrastructure.Repositories.Instance,
		serviceContainer.ApprovalAPI,
		infrastructure.Engine,
		logger,
	)

	// Start all workers
	workerManager := worker.NewManager(logger)
	workerManager.Register(downloadWorker)
	workerManager.Register(invoiceProcessor)
	workerManager.Register(statusPoller)

	if err := workerManager.StartAll(ctx); err != nil {
		logger.Fatal("Failed to start workers", zap.Error(err))
	}

	// ========================================================================
	// STEP 5: HTTP Routes Setup
	// ========================================================================
	setupHTTPRoutes(infrastructure.Router, downloadWorker, invoiceProcessor, logger)

	// Start HTTP server
	infrastructure.StartHTTPServer()

	// ========================================================================
	// STEP 6: Graceful Shutdown
	// ========================================================================
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down server...")

	// Stop workers first
	workerManager.StopAll()

	// Shutdown infrastructure (HTTP + Database)
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := infrastructure.Shutdown(shutdownCtx); err != nil {
		logger.Error("Infrastructure shutdown error", zap.Error(err))
	}

	logger.Info("Server stopped")
}

// findConfigFile searches for config.yaml in current and parent directories
func findConfigFile() string {
	configPath := "configs/config.yaml"

	// If not found in current dir, try parent directories
	if _, err := os.Stat(configPath); err != nil {
		// Try going up one directory (for running from cmd/server)
		if _, err := os.Stat("../../configs/config.yaml"); err == nil {
			if err := os.Chdir("../../"); err != nil {
				log.Fatalf("Could not change to project root: %v", err)
			}
			configPath = "configs/config.yaml"
		} else {
			// Try one more level up
			if err := os.Chdir("../../../"); err == nil {
				if _, err := os.Stat("configs/config.yaml"); err == nil {
					configPath = "configs/config.yaml"
				}
			}
		}
	}

	return configPath
}

// setupHTTPRoutes configures all HTTP endpoints
func setupHTTPRoutes(router *gin.Engine, downloadWorker *worker.AsyncDownloadWorker, invoiceProcessor *worker.InvoiceProcessor, logger *zap.Logger) {
	// Middleware to log all incoming requests
	router.Use(func(c *gin.Context) {
		logger.Info("Incoming HTTP request",
			zap.String("method", c.Request.Method),
			zap.String("path", c.Request.URL.Path),
			zap.String("remote_addr", c.Request.RemoteAddr),
			zap.String("user_agent", c.Request.UserAgent()))
		c.Next()
	})

	// Catch-all route for debugging
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

	// Worker status endpoints
	router.GET("/health/worker", func(c *gin.Context) {
		status := downloadWorker.GetStatus()
		c.JSON(http.StatusOK, status)
	})

	router.GET("/health/invoice-processor", func(c *gin.Context) {
		status := invoiceProcessor.GetStatus()
		c.JSON(http.StatusOK, status)
	})
}
