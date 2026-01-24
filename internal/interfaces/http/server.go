// Package http provides HTTP server adapter for the application layer.
// This is a thin adapter layer that translates HTTP requests to application service calls.
package http

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/garyjia/ai-reimbursement/internal/application/service"
)

// Logger interface for logging operations
type Logger interface {
	Info(msg string, keysAndValues ...interface{})
	Error(msg string, keysAndValues ...interface{})
}

// ServerConfig holds HTTP server configuration
type ServerConfig struct {
	Host         string
	Port         int
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
}

// DefaultServerConfig returns default server configuration
func DefaultServerConfig() ServerConfig {
	return ServerConfig{
		Host:         "0.0.0.0",
		Port:         8080,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}
}

// Server is the HTTP server adapter
type Server struct {
	config          ServerConfig
	httpServer      *http.Server
	router          *gin.Engine
	approvalService service.ApprovalService
	auditService    service.AuditService
	voucherService  service.VoucherService
	logger          Logger
}

// NewServer creates a new HTTP server with the given services
func NewServer(
	config ServerConfig,
	approvalService service.ApprovalService,
	auditService service.AuditService,
	voucherService service.VoucherService,
	logger Logger,
) *Server {
	// Set gin mode based on environment
	gin.SetMode(gin.ReleaseMode)

	router := gin.New()

	server := &Server{
		config:          config,
		router:          router,
		approvalService: approvalService,
		auditService:    auditService,
		voucherService:  voucherService,
		logger:          logger,
	}

	// Setup middleware
	server.setupMiddleware()

	// Setup routes
	server.setupRoutes()

	return server
}

// setupMiddleware configures middleware for the router
func (s *Server) setupMiddleware() {
	// Recovery middleware
	s.router.Use(gin.Recovery())

	// Logging middleware
	s.router.Use(s.loggingMiddleware())
}

// loggingMiddleware creates a logging middleware
func (s *Server) loggingMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		method := c.Request.Method

		// Process request
		c.Next()

		// Log request details
		latency := time.Since(start)
		status := c.Writer.Status()

		s.logger.Info("HTTP request",
			"method", method,
			"path", path,
			"status", status,
			"latency", latency.String(),
			"client_ip", c.ClientIP(),
		)
	}
}

// setupRoutes configures all HTTP routes
func (s *Server) setupRoutes() {
	handlers := NewHandlers(s.approvalService, s.auditService, s.voucherService, s.logger)

	// Health check
	s.router.GET("/health", handlers.HealthCheck)

	// API routes
	api := s.router.Group("/api")
	{
		// Instances
		api.GET("/instances", handlers.ListInstances)
		api.GET("/instances/:id", handlers.GetInstance)
		api.POST("/instances/:id/audit", handlers.TriggerAudit)
		api.POST("/instances/:id/voucher", handlers.GenerateVoucher)
	}
}

// Start starts the HTTP server
func (s *Server) Start(ctx context.Context) error {
	addr := fmt.Sprintf("%s:%d", s.config.Host, s.config.Port)

	s.httpServer = &http.Server{
		Addr:         addr,
		Handler:      s.router,
		ReadTimeout:  s.config.ReadTimeout,
		WriteTimeout: s.config.WriteTimeout,
	}

	s.logger.Info("Starting HTTP server", "address", addr)

	// Start server in goroutine
	errCh := make(chan error, 1)
	go func() {
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	// Wait for context cancellation or error
	select {
	case <-ctx.Done():
		s.logger.Info("HTTP server shutdown requested")
		return s.Stop()
	case err := <-errCh:
		s.logger.Error("HTTP server error", "error", err)
		return err
	}
}

// Stop gracefully stops the HTTP server
func (s *Server) Stop() error {
	if s.httpServer == nil {
		return nil
	}

	s.logger.Info("Stopping HTTP server")

	// Create shutdown context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := s.httpServer.Shutdown(ctx); err != nil {
		s.logger.Error("HTTP server shutdown error", "error", err)
		return err
	}

	s.logger.Info("HTTP server stopped")
	return nil
}

// Router returns the underlying gin router (for testing)
func (s *Server) Router() *gin.Engine {
	return s.router
}

// Address returns the server address
func (s *Server) Address() string {
	return fmt.Sprintf("%s:%d", s.config.Host, s.config.Port)
}
