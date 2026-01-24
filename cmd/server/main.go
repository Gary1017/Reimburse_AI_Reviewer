package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/garyjia/ai-reimbursement/internal/config"
	"github.com/garyjia/ai-reimbursement/internal/container"
	"github.com/garyjia/ai-reimbursement/pkg/utils"
	"go.uber.org/zap"
)

func main() {
	// Load configuration
	configPath := findConfigFile()
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

	// Convert to container config
	containerCfg := cfg.ToContainerConfig()

	// Create container
	c, err := container.NewContainer(containerCfg, logger)
	if err != nil {
		logger.Fatal("Failed to create container", zap.Error(err))
	}

	// Start container
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := c.Start(ctx); err != nil {
		logger.Fatal("Failed to start container", zap.Error(err))
	}

	logger.Info("Application started successfully")

	// Wait for shutdown signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down...")

	// Close container
	if err := c.Close(); err != nil {
		logger.Error("Container shutdown error", zap.Error(err))
	}

	logger.Info("Application stopped")
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
