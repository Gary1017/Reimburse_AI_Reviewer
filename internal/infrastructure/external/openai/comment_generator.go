package openai

import (
	"context"
	"fmt"
	"strings"

	"github.com/garyjia/ai-reimbursement/internal/application/port"
	openai "github.com/sashabaranov/go-openai"
	"go.uber.org/zap"
)

// CommentGenerator implements port.AICommentGenerator using OpenAI
type CommentGenerator struct {
	client *openai.Client
	model  string
	logger *zap.Logger
}

// NewCommentGenerator creates a new comment generator
func NewCommentGenerator(apiKey, model string, logger *zap.Logger) *CommentGenerator {
	return &CommentGenerator{
		client: openai.NewClient(apiKey),
		model:  model,
		logger: logger,
	}
}

// GenerateComment generates a Chinese-language audit comment
func (g *CommentGenerator) GenerateComment(ctx context.Context, req *port.CommentGenerationRequest) (string, error) {
	if req == nil {
		return "", fmt.Errorf("comment generation request is nil")
	}

	g.logger.Info("Generating comment",
		zap.String("decision", req.Decision),
		zap.Int("violation_count", len(req.Violations)),
		zap.Int("finding_count", len(req.AIFindings)))

	systemPrompt := `你是一个企业报销审核AI助手。你需要根据审核结果生成中文审核意见。

要求：
1. 语言简洁专业
2. 如果是拒绝，列出具体问题
3. 如果是通过但有提醒，说明提醒事项
4. 如果是通过，简要说明审核通过
5. 金额用人民币元显示

参考模板：
- 拒绝: "经AI审核，该报销申请存在以下问题：1. ... 建议修改后重新提交。"
- 通过: "AI审核通过，该报销申请符合公司规定。"
- 通过（有提醒）: "AI审核通过（存在提醒事项）：1. ..."`

	// Build user prompt
	var parts []string
	parts = append(parts, fmt.Sprintf("审核决定: %s", g.translateDecision(req.Decision)))
	parts = append(parts, fmt.Sprintf("报销总金额: %.2f元", float64(req.TotalAmount)/100.0))

	if len(req.Items) > 0 {
		var itemDescs []string
		for _, item := range req.Items {
			itemDescs = append(itemDescs, fmt.Sprintf("- %s: %.2f元 (%s)", item.Description, item.Amount, item.ItemType))
		}
		parts = append(parts, "报销项目:\n"+strings.Join(itemDescs, "\n"))
	}

	if len(req.Violations) > 0 {
		parts = append(parts, "违规问题:")
		for i, v := range req.Violations {
			parts = append(parts, fmt.Sprintf("%d. %s", i+1, v))
		}
	}

	if len(req.AIFindings) > 0 {
		parts = append(parts, "AI审核发现:")
		for i, f := range req.AIFindings {
			parts = append(parts, fmt.Sprintf("%d. %s", i+1, f))
		}
	}

	userPrompt := strings.Join(parts, "\n\n") + "\n\n请根据以上信息生成审核意见。"

	resp, err := g.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model:       g.model,
		Temperature: 0.3,
		MaxTokens:   500,
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleSystem, Content: systemPrompt},
			{Role: openai.ChatMessageRoleUser, Content: userPrompt},
		},
	})

	if err != nil {
		g.logger.Error("OpenAI API call failed, using fallback comment",
			zap.Error(err),
			zap.String("decision", req.Decision))
		return g.fallbackComment(req), nil
	}

	if len(resp.Choices) == 0 {
		g.logger.Error("No response from OpenAI, using fallback comment")
		return g.fallbackComment(req), nil
	}

	comment := strings.TrimSpace(resp.Choices[0].Message.Content)
	g.logger.Info("Comment generated successfully",
		zap.String("decision", req.Decision),
		zap.Int("comment_length", len(comment)))

	return comment, nil
}

// fallbackComment generates a simple fallback comment when API fails
func (g *CommentGenerator) fallbackComment(req *port.CommentGenerationRequest) string {
	switch req.Decision {
	case "REJECT":
		if len(req.Violations) > 0 {
			return "AI审核不通过：" + strings.Join(req.Violations, "；")
		}
		return "AI审核不通过。"
	case "APPROVE_WITH_WARNINGS":
		if len(req.AIFindings) > 0 {
			return "AI审核通过（存在提醒事项）：" + strings.Join(req.AIFindings, "；")
		}
		return "AI审核通过（存在提醒事项）。"
	default:
		return "AI审核通过。"
	}
}

// translateDecision translates decision to Chinese
func (g *CommentGenerator) translateDecision(decision string) string {
	switch decision {
	case "REJECT":
		return "拒绝"
	case "APPROVE":
		return "通过"
	case "APPROVE_WITH_WARNINGS":
		return "通过（有提醒）"
	default:
		return decision
	}
}
