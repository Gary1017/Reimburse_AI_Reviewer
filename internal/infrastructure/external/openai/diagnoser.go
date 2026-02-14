package openai

import (
	"context"
	"fmt"

	openai "github.com/sashabaranov/go-openai"
	"go.uber.org/zap"
)

// ErrorDiagnoser implements port.AIErrorDiagnoser using OpenAI
type ErrorDiagnoser struct {
	client *openai.Client
	model  string
	logger *zap.Logger
}

// NewErrorDiagnoser creates a new OpenAI error diagnoser
func NewErrorDiagnoser(apiKey, model string, logger *zap.Logger) *ErrorDiagnoser {
	return &ErrorDiagnoser{
		client: openai.NewClient(apiKey),
		model:  model,
		logger: logger,
	}
}

// DiagnoseError diagnoses an error and suggests possible causes and solutions
func (d *ErrorDiagnoser) DiagnoseError(ctx context.Context, errorDescription string) (string, error) {
	d.logger.Debug("Diagnosing error with GPT",
		zap.String("error_description", errorDescription))

	resp, err := d.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model:       d.model,
		Temperature: 0.3,
		MaxTokens:   500,
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleSystem,
				Content: "You are an IT support assistant. Diagnose the following error and suggest possible causes and solutions. Respond in Chinese.",
			},
			{
				Role:    openai.ChatMessageRoleUser,
				Content: errorDescription,
			},
		},
	})

	if err != nil {
		d.logger.Error("OpenAI API call failed", zap.Error(err))
		return "", fmt.Errorf("GPT diagnosis: %w", err)
	}

	if len(resp.Choices) == 0 {
		d.logger.Error("No response from OpenAI")
		return "", fmt.Errorf("no response from GPT")
	}

	diagnosis := resp.Choices[0].Message.Content

	d.logger.Info("Error diagnosis completed",
		zap.Int("diagnosis_length", len(diagnosis)))

	return diagnosis, nil
}
