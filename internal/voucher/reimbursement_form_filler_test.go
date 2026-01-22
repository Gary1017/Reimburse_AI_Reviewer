package voucher

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xuri/excelize/v2"
	"go.uber.org/zap"
)

// ARCH-013-A: ReimbursementFormFiller tests
// Tests for Excel template filling with form data

func TestReimbursementFormFiller_FillTemplate(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	ctx := context.Background()

	// Use the actual template from the templates directory
	templatePath := filepath.Join("..", "..", "templates", "报销单模板.xlsx")

	t.Run("fills template with complete form data", func(t *testing.T) {
		// Skip if template doesn't exist
		if _, err := os.Stat(templatePath); os.IsNotExist(err) {
			t.Skip("Template file not found, skipping test")
		}

		tempDir := t.TempDir()
		outputPath := filepath.Join(tempDir, "output.xlsx")

		filler, err := NewReimbursementFormFiller(templatePath, logger)
		require.NoError(t, err)

		formData := &FormData{
			ApplicantName:  "张三",
			Department:     "技术部",
			LarkInstanceID: "ABC123-XYZ",
			SubmissionDate: time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
			TotalAmount:    650.50,
			TotalReceipts:  3,
			Items: []FormItemData{
				{
					SequenceNumber:    1,
					AccountingSubject: "差旅费",
					Description:       "北京出差",
					ExpenseType:       "差旅费",
					ReceiptCount:      2,
					Amount:            500.00,
					Remarks:           "客户会议",
				},
				{
					SequenceNumber:    2,
					AccountingSubject: "餐费",
					Description:       "工作餐",
					ExpenseType:       "餐饮费",
					ReceiptCount:      1,
					Amount:            150.50,
					Remarks:           "加班",
				},
			},
		}

		resultPath, err := filler.FillTemplate(ctx, formData, outputPath)

		require.NoError(t, err)
		assert.Equal(t, outputPath, resultPath)
		assert.FileExists(t, outputPath)

		// Verify the output file can be opened and contains expected data
		f, err := excelize.OpenFile(outputPath)
		require.NoError(t, err)
		defer f.Close()

		// Verify header section
		applicant, _ := f.GetCellValue("Sheet1", "B4")
		assert.Equal(t, "张三", applicant)

		department, _ := f.GetCellValue("Sheet1", "E4")
		assert.Equal(t, "技术部", department)

		instanceID, _ := f.GetCellValue("Sheet1", "H4")
		assert.Equal(t, "ABC123-XYZ", instanceID)

		// Verify first item row (row 6)
		seq1, _ := f.GetCellValue("Sheet1", "A6")
		assert.Equal(t, "1", seq1)

		subject1, _ := f.GetCellValue("Sheet1", "B6")
		assert.Equal(t, "差旅费", subject1)

		desc1, _ := f.GetCellValue("Sheet1", "C6")
		assert.Equal(t, "北京出差", desc1)

		// Verify second item row (row 7)
		seq2, _ := f.GetCellValue("Sheet1", "A7")
		assert.Equal(t, "2", seq2)
	})

	t.Run("handles empty items list", func(t *testing.T) {
		if _, err := os.Stat(templatePath); os.IsNotExist(err) {
			t.Skip("Template file not found, skipping test")
		}

		tempDir := t.TempDir()
		outputPath := filepath.Join(tempDir, "empty_items.xlsx")

		filler, err := NewReimbursementFormFiller(templatePath, logger)
		require.NoError(t, err)

		formData := &FormData{
			ApplicantName:  "李四",
			Department:     "财务部",
			LarkInstanceID: "EMPTY-ITEMS",
			SubmissionDate: time.Now(),
			Items:          []FormItemData{},
			TotalAmount:    0,
			TotalReceipts:  0,
		}

		resultPath, err := filler.FillTemplate(ctx, formData, outputPath)

		require.NoError(t, err)
		assert.FileExists(t, resultPath)
	})

	t.Run("handles maximum 8 items", func(t *testing.T) {
		if _, err := os.Stat(templatePath); os.IsNotExist(err) {
			t.Skip("Template file not found, skipping test")
		}

		tempDir := t.TempDir()
		outputPath := filepath.Join(tempDir, "max_items.xlsx")

		filler, err := NewReimbursementFormFiller(templatePath, logger)
		require.NoError(t, err)

		// Create 8 items (maximum for template)
		items := make([]FormItemData, 8)
		totalAmount := 0.0
		for i := 0; i < 8; i++ {
			items[i] = FormItemData{
				SequenceNumber:    i + 1,
				AccountingSubject: "差旅费",
				Description:       "出差",
				ExpenseType:       "差旅费",
				ReceiptCount:      1,
				Amount:            100.00,
			}
			totalAmount += 100.00
		}

		formData := &FormData{
			ApplicantName:  "王五",
			Department:     "销售部",
			LarkInstanceID: "MAX-ITEMS",
			SubmissionDate: time.Now(),
			Items:          items,
			TotalAmount:    totalAmount,
			TotalReceipts:  8,
		}

		resultPath, err := filler.FillTemplate(ctx, formData, outputPath)

		require.NoError(t, err)
		assert.FileExists(t, resultPath)

		// Verify all 8 rows are filled
		f, err := excelize.OpenFile(outputPath)
		require.NoError(t, err)
		defer f.Close()

		// Check last item row (row 13)
		seq8, _ := f.GetCellValue("Sheet1", "A13")
		assert.Equal(t, "8", seq8)
	})

	t.Run("returns error for non-existent template", func(t *testing.T) {
		_, err := NewReimbursementFormFiller("/nonexistent/template.xlsx", logger)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "template")
	})

	t.Run("returns error for invalid output path", func(t *testing.T) {
		if _, err := os.Stat(templatePath); os.IsNotExist(err) {
			t.Skip("Template file not found, skipping test")
		}

		filler, err := NewReimbursementFormFiller(templatePath, logger)
		require.NoError(t, err)

		formData := &FormData{
			ApplicantName:  "测试",
			LarkInstanceID: "TEST",
			SubmissionDate: time.Now(),
		}

		// Try to write to an invalid path
		_, err = filler.FillTemplate(ctx, formData, "/nonexistent/dir/output.xlsx")

		assert.Error(t, err)
	})
}

func TestReimbursementFormFiller_ValidateTemplate(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	templatePath := filepath.Join("..", "..", "templates", "报销单模板.xlsx")

	t.Run("validates correct template", func(t *testing.T) {
		if _, err := os.Stat(templatePath); os.IsNotExist(err) {
			t.Skip("Template file not found, skipping test")
		}

		filler, err := NewReimbursementFormFiller(templatePath, logger)
		require.NoError(t, err)

		err = filler.ValidateTemplate()
		assert.NoError(t, err)
	})
}

func TestNewReimbursementFormFiller(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	templatePath := filepath.Join("..", "..", "templates", "报销单模板.xlsx")

	t.Run("creates filler with valid template", func(t *testing.T) {
		if _, err := os.Stat(templatePath); os.IsNotExist(err) {
			t.Skip("Template file not found, skipping test")
		}

		filler, err := NewReimbursementFormFiller(templatePath, logger)

		assert.NoError(t, err)
		assert.NotNil(t, filler)
	})
}
