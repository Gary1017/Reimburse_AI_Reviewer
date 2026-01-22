package utils

import (
	"os"
	"path/filepath"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// LoggerConfig holds logger configuration
type LoggerConfig struct {
	Level      string // debug, info, warn, error
	OutputPath string // stdout, stderr, or file path
	Format     string // json or console
}

// NewLogger creates a new structured logger
func NewLogger(cfg LoggerConfig) (*zap.Logger, error) {
	// Parse log level
	var level zapcore.Level
	if err := level.UnmarshalText([]byte(cfg.Level)); err != nil {
		level = zapcore.InfoLevel
	}

	// Configure encoder
	var encoderConfig zapcore.EncoderConfig
	if cfg.Format == "json" {
		encoderConfig = zap.NewProductionEncoderConfig()
	} else {
		encoderConfig = zap.NewDevelopmentEncoderConfig()
		encoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	}

	encoderConfig.TimeKey = "timestamp"
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

	// Configure output
	var writeSyncer zapcore.WriteSyncer
	switch cfg.OutputPath {
	case "stdout", "":
		writeSyncer = zapcore.AddSync(os.Stdout)
	case "stderr":
		writeSyncer = zapcore.AddSync(os.Stderr)
	default:
		// Create directory if it doesn't exist
		dir := filepath.Dir(cfg.OutputPath)
		if dir != "." {
			if err := os.MkdirAll(dir, 0755); err != nil {
				return nil, err
			}
		}
		file, err := os.OpenFile(cfg.OutputPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return nil, err
		}
		writeSyncer = zapcore.AddSync(file)
	}

	// Create encoder
	var encoder zapcore.Encoder
	if cfg.Format == "json" {
		encoder = zapcore.NewJSONEncoder(encoderConfig)
	} else {
		encoder = zapcore.NewConsoleEncoder(encoderConfig)
	}

	// Create core
	core := zapcore.NewCore(encoder, writeSyncer, level)

	// Create logger with caller information
	logger := zap.New(core, zap.AddCaller(), zap.AddCallerSkip(0), zap.AddStacktrace(zapcore.ErrorLevel))

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
