package utils

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// LoggerConfig holds logger configuration
type LoggerConfig struct {
	Level      string // debug, info, warn, error
	OutputPath string // stdout, stderr, or file path
	Format     string // json or console
}

// NewLogger creates a new structured logger with dual output support
// If OutputPath is a file path, logs are written to both console (info level) and timestamped file (debug level)
func NewLogger(cfg LoggerConfig) (*zap.Logger, error) {
	// Parse log level
	var level zapcore.Level
	if err := level.UnmarshalText([]byte(cfg.Level)); err != nil {
		level = zapcore.InfoLevel
	}

	var cores []zapcore.Core

	// If output is a file path, create dual output (console + timestamped file)
	if cfg.OutputPath != "stdout" && cfg.OutputPath != "stderr" && cfg.OutputPath != "" {
		// 1. Console output core (info level, console format, colorized)
		consoleEncoderConfig := zap.NewDevelopmentEncoderConfig()
		consoleEncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
		consoleEncoderConfig.TimeKey = "timestamp"
		consoleEncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

		consoleEncoder := zapcore.NewConsoleEncoder(consoleEncoderConfig)
		consoleCore := zapcore.NewCore(
			consoleEncoder,
			zapcore.AddSync(os.Stdout),
			zapcore.InfoLevel, // Console shows info and above
		)
		cores = append(cores, consoleCore)

		// 2. File output core (all levels, json format, timestamped filename)
		fileEncoderConfig := zap.NewProductionEncoderConfig()
		fileEncoderConfig.TimeKey = "timestamp"
		fileEncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

		// Generate timestamped filename
		timestamp := time.Now().Format("2006-01-02_15-04-05")
		dir := filepath.Dir(cfg.OutputPath)
		ext := filepath.Ext(cfg.OutputPath)
		base := filepath.Base(cfg.OutputPath)
		baseWithoutExt := base[:len(base)-len(ext)]
		timestampedPath := filepath.Join(dir, fmt.Sprintf("%s_%s%s", baseWithoutExt, timestamp, ext))

		// Create directory if it doesn't exist
		if dir != "." {
			if err := os.MkdirAll(dir, 0755); err != nil {
				return nil, err
			}
		}

		file, err := os.OpenFile(timestampedPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return nil, err
		}

		fileEncoder := zapcore.NewJSONEncoder(fileEncoderConfig)
		fileCore := zapcore.NewCore(
			fileEncoder,
			zapcore.AddSync(file),
			level, // File gets configured level (typically debug)
		)
		cores = append(cores, fileCore)
	} else {
		// Single output to stdout/stderr
		var encoderConfig zapcore.EncoderConfig
		if cfg.Format == "json" {
			encoderConfig = zap.NewProductionEncoderConfig()
		} else {
			encoderConfig = zap.NewDevelopmentEncoderConfig()
			encoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
		}

		encoderConfig.TimeKey = "timestamp"
		encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

		var writeSyncer zapcore.WriteSyncer
		switch cfg.OutputPath {
		case "stderr":
			writeSyncer = zapcore.AddSync(os.Stderr)
		default: // stdout or empty
			writeSyncer = zapcore.AddSync(os.Stdout)
		}

		var encoder zapcore.Encoder
		if cfg.Format == "json" {
			encoder = zapcore.NewJSONEncoder(encoderConfig)
		} else {
			encoder = zapcore.NewConsoleEncoder(encoderConfig)
		}

		core := zapcore.NewCore(encoder, writeSyncer, level)
		cores = append(cores, core)
	}

	// Combine all cores
	combinedCore := zapcore.NewTee(cores...)

	// Create logger with caller information
	logger := zap.New(combinedCore, zap.AddCaller(), zap.AddCallerSkip(0), zap.AddStacktrace(zapcore.ErrorLevel))

	return logger, nil
}

// NewDevelopmentLogger creates a logger suitable for development
// Deprecated: Use NewLogger with explicit config instead
func NewDevelopmentLogger() (*zap.Logger, error) {
	return NewLogger(LoggerConfig{
		Level:      "debug",
		OutputPath: "stdout",
		Format:     "console",
	})
}

// NewProductionLogger creates a logger suitable for production
// Deprecated: Use NewLogger with explicit config instead
func NewProductionLogger() (*zap.Logger, error) {
	return NewLogger(LoggerConfig{
		Level:      "info",
		OutputPath: "logs/server.log",
		Format:     "json",
	})
}
