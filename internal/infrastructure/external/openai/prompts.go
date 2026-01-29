package openai

import (
	"bytes"
	"fmt"
	"os"
	"text/template"

	"gopkg.in/yaml.v3"
)

// PromptConfig holds all AI prompts and model parameters used by the OpenAI auditor
type PromptConfig struct {
	PolicyAudit struct {
		Temperature  float32 `yaml:"temperature"`
		MaxTokens    int     `yaml:"max_tokens"`
		System       string  `yaml:"system"`
		UserTemplate string  `yaml:"user_template"`
	} `yaml:"policy_audit"`

	PriceAudit struct {
		Temperature  float32 `yaml:"temperature"`
		MaxTokens    int     `yaml:"max_tokens"`
		System       string  `yaml:"system"`
		UserTemplate string  `yaml:"user_template"`
	} `yaml:"price_audit"`

	InvoiceExtraction struct {
		Temperature  float32 `yaml:"temperature"`
		MaxTokens    int     `yaml:"max_tokens"`
		System       string  `yaml:"system"`
		UserTemplate string  `yaml:"user_template"`
	} `yaml:"invoice_extraction"`
}

// LoadPrompts loads prompt configuration from YAML file
func LoadPrompts(promptsPath string) (*PromptConfig, error) {
	data, err := os.ReadFile(promptsPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read prompts file: %w", err)
	}

	var prompts PromptConfig
	if err := yaml.Unmarshal(data, &prompts); err != nil {
		return nil, fmt.Errorf("failed to unmarshal prompts: %w", err)
	}

	return &prompts, nil
}

// renderTemplate renders a template with provided data
func renderTemplate(templateStr string, data interface{}) (string, error) {
	tmpl, err := template.New("prompt").Parse(templateStr)
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	return buf.String(), nil
}
