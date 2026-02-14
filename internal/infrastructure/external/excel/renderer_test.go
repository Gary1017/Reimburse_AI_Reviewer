package excel

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/garyjia/ai-reimbursement/internal/application/port"
	"github.com/garyjia/ai-reimbursement/internal/domain/entity"
	"github.com/xuri/excelize/v2"
	"go.uber.org/zap"
)

func TestVoucherRenderer_Render(t *testing.T) {
	renderer := NewVoucherRenderer(zap.NewNop())

	data := &port.VoucherData{
		CompanyName:   "测试公司",
		ApplicantName: "张三",
		Department:    "技术部",
		ApprovalID:    "TEST-001",
		Items: []*entity.ReimbursementItem{
			{ID: 1, AmountCents: 30000, Description: "出差餐饮", ItemType: "餐饮费", BusinessPurpose: "客户拜访"},
			{ID: 2, AmountCents: 50000, Description: "火车票", ItemType: "交通费", BusinessPurpose: "出差北京"},
		},
		Attachments: []*entity.Attachment{
			{ID: 10, ItemID: 1, FileType: entity.FileTypeInvoice},
			{ID: 11, ItemID: 2, FileType: entity.FileTypeInvoice},
		},
		TotalAmountCents: 80000,
	}

	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "test_voucher.xlsx")

	// Template path relative from test file to project root
	// The test is in internal/infrastructure/external/excel/ so need to go up 4 dirs
	templatePath := filepath.Join("..", "..", "..", "..", "templates", "Riembursement_Form_Template.xlsx")

	result, err := renderer.Render(context.Background(), data, templatePath, outputPath)
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	if result.FilePath != outputPath {
		t.Errorf("expected FilePath %s, got %s", outputPath, result.FilePath)
	}

	if result.FileName != "test_voucher.xlsx" {
		t.Errorf("expected FileName test_voucher.xlsx, got %s", result.FileName)
	}

	// Verify output file exists
	if _, err := os.Stat(outputPath); os.IsNotExist(err) {
		t.Fatalf("output file does not exist: %s", outputPath)
	}

	// Verify some data was written correctly
	f, err := excelize.OpenFile(outputPath)
	if err != nil {
		t.Fatalf("failed to open output file: %v", err)
	}
	defer f.Close()

	// Check header data
	applicantName, err := f.GetCellValue("Sheet1", "B4")
	if err != nil {
		t.Fatalf("failed to read applicant name: %v", err)
	}
	if applicantName != "张三" {
		t.Errorf("expected applicant name '张三', got '%s'", applicantName)
	}

	department, err := f.GetCellValue("Sheet1", "E4")
	if err != nil {
		t.Fatalf("failed to read department: %v", err)
	}
	if department != "技术部" {
		t.Errorf("expected department '技术部', got '%s'", department)
	}

	approvalID, err := f.GetCellValue("Sheet1", "H4")
	if err != nil {
		t.Fatalf("failed to read approval ID: %v", err)
	}
	if approvalID != "TEST-001" {
		t.Errorf("expected approval ID 'TEST-001', got '%s'", approvalID)
	}

	// Check first item data (row 6)
	itemType, err := f.GetCellValue("Sheet1", "B6")
	if err != nil {
		t.Fatalf("failed to read item type: %v", err)
	}
	if itemType != "餐饮费" {
		t.Errorf("expected item type '餐饮费', got '%s'", itemType)
	}

	description, err := f.GetCellValue("Sheet1", "C6")
	if err != nil {
		t.Fatalf("failed to read description: %v", err)
	}
	if description != "出差餐饮" {
		t.Errorf("expected description '出差餐饮', got '%s'", description)
	}

	// Check invoice count (should be 1)
	invoiceCount, err := f.GetCellValue("Sheet1", "E6")
	if err != nil {
		t.Fatalf("failed to read invoice count: %v", err)
	}
	if invoiceCount != "1" {
		t.Errorf("expected invoice count '1', got '%s'", invoiceCount)
	}

	// Check amount (should be 300.00 yuan)
	amount, err := f.GetCellValue("Sheet1", "F6")
	if err != nil {
		t.Fatalf("failed to read amount: %v", err)
	}
	// Excel formats numbers with decimals and may include trailing spaces
	expectedAmounts := []string{"300", "300.00", "300.00 "}
	found := false
	for _, expected := range expectedAmounts {
		if amount == expected {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected amount to be one of %v, got '%s'", expectedAmounts, amount)
	}
}

func TestVoucherRenderer_RenderMaxItems(t *testing.T) {
	renderer := NewVoucherRenderer(zap.NewNop())

	// Create 10 items (more than max 8)
	items := make([]*entity.ReimbursementItem, 10)
	for i := 0; i < 10; i++ {
		items[i] = &entity.ReimbursementItem{
			ID:              int64(i + 1),
			AmountCents:     10000,
			Description:     "测试项目",
			ItemType:        "测试类型",
			BusinessPurpose: "测试用途",
		}
	}

	data := &port.VoucherData{
		CompanyName:      "测试公司",
		ApplicantName:    "测试人",
		Department:       "测试部",
		ApprovalID:       "TEST-002",
		Items:            items,
		Attachments:      []*entity.Attachment{},
		TotalAmountCents: 100000,
	}

	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "test_voucher_max.xlsx")
	templatePath := filepath.Join("..", "..", "..", "..", "templates", "Riembursement_Form_Template.xlsx")

	result, err := renderer.Render(context.Background(), data, templatePath, outputPath)
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	// Verify file was created
	if result.FilePath != outputPath {
		t.Errorf("expected FilePath %s, got %s", outputPath, result.FilePath)
	}

	// Open and verify only 8 items were written
	f, err := excelize.OpenFile(outputPath)
	if err != nil {
		t.Fatalf("failed to open output file: %v", err)
	}
	defer f.Close()

	// Row 13 is the 8th item (startRow=6, so row 13 = 6+7)
	row13Value, err := f.GetCellValue("Sheet1", "A13")
	if err != nil {
		t.Fatalf("failed to read row 13: %v", err)
	}
	if row13Value != "8" {
		t.Errorf("expected row 13 to have sequence '8', got '%s'", row13Value)
	}

	// Row 14 should not have a sequence number (9th item should not be written)
	// Note: row 14 might contain template content like "合计", so we just check A14 for sequence
	row14Value, err := f.GetCellValue("Sheet1", "A14")
	if err != nil {
		t.Fatalf("failed to read row 14: %v", err)
	}
	// A14 should not be "9" (the 9th item sequence)
	if row14Value == "9" {
		t.Errorf("expected row 14 not to have sequence '9' (9th item should not be written), got '%s'", row14Value)
	}
}

func TestVoucherRenderer_RenderInvalidTemplate(t *testing.T) {
	renderer := NewVoucherRenderer(zap.NewNop())

	data := &port.VoucherData{
		CompanyName:      "测试公司",
		ApplicantName:    "测试人",
		Department:       "测试部",
		ApprovalID:       "TEST-003",
		Items:            []*entity.ReimbursementItem{},
		Attachments:      []*entity.Attachment{},
		TotalAmountCents: 0,
	}

	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "test_voucher_invalid.xlsx")
	templatePath := "/nonexistent/path/template.xlsx"

	_, err := renderer.Render(context.Background(), data, templatePath, outputPath)
	if err == nil {
		t.Fatal("expected error for invalid template path, got nil")
	}
}
