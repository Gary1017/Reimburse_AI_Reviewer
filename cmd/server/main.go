package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/garyjia/ai-reimbursement/internal/ai"
	"github.com/garyjia/ai-reimbursement/internal/config"
	"github.com/garyjia/ai-reimbursement/internal/email"
	"github.com/garyjia/ai-reimbursement/internal/lark"
	"github.com/garyjia/ai-reimbursement/internal/repository"
	"github.com/garyjia/ai-reimbursement/internal/voucher"
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
	// Load configuration
	cfg, err := config.Load("configs/config.yaml")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load configuration: %v\n", err)
		os.Exit(1)
	}

	// Initialize logger
	logger, err := utils.NewLogger(utils.LoggerConfig{
		Level:      cfg.Logger.Level,
		OutputPath: cfg.Logger.OutputPath,
		Format:     cfg.Logger.Format,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}
	defer logger.Sync()

	logger.Info("Starting AI Reimbursement Workflow System",
		zap.String("version", "1.0.0"),
		zap.Int("port", cfg.Server.Port))

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
		logger.Fatal("Failed to run database migrations", zap.Error(err))
	}

	// Create necessary directories
	if err := os.MkdirAll(cfg.Voucher.OutputDir, 0755); err != nil {
		logger.Fatal("Failed to create output directory", zap.Error(err))
	}

	// Initialize repositories
	instanceRepo := repository.NewInstanceRepository(db.DB, logger)
	historyRepo := repository.NewHistoryRepository(db.DB, logger)
	voucherRepo := repository.NewVoucherRepository(db.DB, logger)
	itemRepo := repository.NewReimbursementItemRepository(db.DB, logger)

	// Initialize Lark client
	larkClient := lark.NewClient(lark.Config{
		AppID:        cfg.Lark.AppID,
		AppSecret:    cfg.Lark.AppSecret,
		ApprovalCode: cfg.Lark.ApprovalCode,
	}, logger)

	approvalAPI := lark.NewApprovalAPI(larkClient, logger)
	messageAPI := lark.NewMessageAPI(larkClient, logger)

	// Initialize AI components
	policyValidator, err := ai.NewPolicyValidator(
		cfg.OpenAI.APIKey,
		cfg.OpenAI.Model,
		cfg.OpenAI.Temperature,
		"configs/policies.json",
		logger,
	)
	if err != nil {
		logger.Fatal("Failed to initialize policy validator", zap.Error(err))
	}

	priceBenchmarker := ai.NewPriceBenchmarker(
		cfg.OpenAI.APIKey,
		cfg.OpenAI.Model,
		cfg.OpenAI.Temperature,
		cfg.Voucher.PriceDeviation,
		logger,
	)

	auditor := ai.NewAuditor(policyValidator, priceBenchmarker, logger)

	// Initialize workflow engine
	// Initialize attachment handler and repository
	attachmentHandler := lark.NewAttachmentHandler(logger, cfg.Voucher.AttachmentDir)
	attachmentRepo := repository.NewAttachmentRepository(db.DB, logger)

	workflowEngine := workflow.NewEngine(
		db,
		instanceRepo,
		historyRepo,
		itemRepo,
		attachmentRepo,
		approvalAPI,
		attachmentHandler,
		logger,
	)

	// Initialize Lark event processor and WS client
	eventProcessor := lark.NewEventProcessor(cfg.Lark.ApprovalCode, workflowEngine, logger)
	eventHandler := dispatcher.NewEventDispatcher("", "").
		OnCustomizedEvent("approval_instance", eventProcessor.HandleCustomizedEvent)

	wsClient := larkws.NewClient(cfg.Lark.AppID, cfg.Lark.AppSecret,
		larkws.WithEventHandler(eventHandler),
		larkws.WithLogLevel(larkcore.LogLevelInfo),
	)

	go func() {
		if err := wsClient.Start(context.Background()); err != nil {
			logger.Fatal("Failed to start Lark WS client", zap.Error(err))
		}
	}()

	// Initialize voucher generator
	voucherGenerator, err := voucher.NewGenerator(
		db,
		instanceRepo,
		voucherRepo,
		approvalAPI,
		voucher.Config{
			TemplatePath:    cfg.Voucher.TemplatePath,
			OutputDir:       cfg.Voucher.OutputDir,
			CompanyName:     cfg.Voucher.CompanyName,
			CompanyTaxID:    cfg.Voucher.CompanyTaxID,
			AccountantEmail: cfg.Email.AccountantEmail,
		},
		logger,
	)
	if err != nil {
		logger.Fatal("Failed to initialize voucher generator", zap.Error(err))
	}

	// Initialize email sender
	emailSender := email.NewSender(messageAPI, voucherRepo, logger)

	// Set Gin mode based on logger level
	if cfg.Logger.Level == "debug" {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	// Initialize HTTP router
	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(loggingMiddleware(logger))
	router.Use(corsMiddleware())

	// Health check endpoint
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":  "healthy",
			"service": "ai-reimbursement",
			"time":    time.Now().Format(time.RFC3339),
		})
	})

	// Admin API endpoints (for testing and monitoring)
	api := router.Group("/api/v1")
	{
		api.GET("/instances/:id", func(c *gin.Context) {
			// Get instance by ID
			// TODO: Implement admin API handlers
			c.JSON(http.StatusOK, gin.H{"message": "Admin API endpoint"})
		})

		// Test endpoint for form parsing verification
		api.POST("/test/parse-form", func(c *gin.Context) {
			var formData map[string]interface{}
			if err := c.ShouldBindJSON(&formData); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}

			// Convert to JSON string for parser
			formJSON, err := json.Marshal(formData)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to marshal form data"})
				return
			}

			// Parse using FormParser
			formParser := lark.NewFormParser(logger)
			items, err := formParser.Parse(string(formJSON))
			if err != nil {
				logger.Warn("Form parsing failed", zap.Error(err))
				c.JSON(http.StatusOK, gin.H{
					"success": false,
					"error":   err.Error(),
					"items":   []interface{}{},
				})
				return
			}

			// Convert items to JSON for response
			itemsJSON := make([]map[string]interface{}, len(items))
			for i, item := range items {
				itemsJSON[i] = map[string]interface{}{
					"id":                 item.ID,
					"item_type":          item.ItemType,
					"description":        item.Description,
					"amount":             item.Amount,
					"currency":           item.Currency,
					"expense_date":       item.ExpenseDate,
					"vendor":             item.Vendor,
					"business_purpose":   item.BusinessPurpose,
					"receipt_attachment": item.ReceiptAttachment,
				}
			}

			logger.Info("Form parsed successfully", zap.Int("item_count", len(items)))
			c.JSON(http.StatusOK, gin.H{
				"success": true,
				"items":   itemsJSON,
				"count":   len(items),
			})
		})
	}

	// Store dependencies in context for use in handlers
	appContext := &AppContext{
		DB:               db,
		WorkflowEngine:   workflowEngine,
		VoucherGenerator: voucherGenerator,
		EmailSender:      emailSender,
		Auditor:          auditor,
		Logger:           logger,
	}

	// Make context available to handlers
	router.Use(func(c *gin.Context) {
		c.Set("app_context", appContext)
		c.Next()
	})

	// Create HTTP server
	srv := &http.Server{
		Addr:         fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port),
		Handler:      router,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
	}

	// Start server in a goroutine
	go func() {
		logger.Info("HTTP server listening", zap.String("addr", srv.Addr))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("Failed to start HTTP server", zap.Error(err))
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down server...")

	// Graceful shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.Error("Server forced to shutdown", zap.Error(err))
	}

	logger.Info("Server exited successfully")
}

// AppContext holds application-wide dependencies
type AppContext struct {
	DB               *database.DB
	WorkflowEngine   *workflow.Engine
	VoucherGenerator *voucher.Generator
	EmailSender      *email.Sender
	Auditor          *ai.Auditor
	Logger           *zap.Logger
}

// loggingMiddleware logs HTTP requests
func loggingMiddleware(logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery

		c.Next()

		latency := time.Since(start)

		logger.Info("HTTP request",
			zap.String("method", c.Request.Method),
			zap.String("path", path),
			zap.String("query", query),
			zap.Int("status", c.Writer.Status()),
			zap.Duration("latency", latency),
			zap.String("ip", c.ClientIP()),
		)
	}
}

// corsMiddleware adds CORS headers
func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}
