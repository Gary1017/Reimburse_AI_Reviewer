package voucher

import (
	"context"
	"fmt"
	"os"

	"github.com/xuri/excelize/v2"
	"go.uber.org/zap"
)

// ReimbursementFormFiller fills Excel templates with form data
// ARCH-013-A: Excel template filling implementation
type ReimbursementFormFiller struct {
	templatePath string
	logger       *zap.Logger
}

// Template cell positions (based on 报销单模板.xlsx structure)
const (
	sheetName = "Sheet1"

	// Header section (row 4)
	cellApplicant  = "B4"
	cellDepartment = "E4"
	cellInstanceID = "H4"

	// Data rows start at row 6 and end at row 13 (8 items max)
	dataRowStart = 6
	dataRowEnd   = 13
	maxItems     = 8

	// Column letters for data rows
	colSequence    = "A"
	colSubject     = "B"
	colDescription = "C"
	colExpenseType = "D"
	colReceiptCnt  = "E"
	colAmount      = "F"
	colRemarks     = "H"
)

// NewReimbursementFormFiller creates a new ReimbursementFormFiller
func NewReimbursementFormFiller(templatePath string, logger *zap.Logger) (*ReimbursementFormFiller, error) {
	// Verify template exists
	if _, err := os.Stat(templatePath); os.IsNotExist(err) {
		return nil, fmt.Errorf("template file not found: %s", templatePath)
	}

	return &ReimbursementFormFiller{
		templatePath: templatePath,
		logger:       logger,
	}, nil
}

// FillTemplate opens the template, fills cells, and saves to outputPath
func (f *ReimbursementFormFiller) FillTemplate(ctx context.Context, data *FormData, outputPath string) (string, error) {
	f.logger.Debug("Filling reimbursement form template",
		zap.String("lark_instance_id", data.LarkInstanceID),
		zap.String("output_path", outputPath))

	// Open template
	file, err := excelize.OpenFile(f.templatePath)
	if err != nil {
		return "", fmt.Errorf("failed to open template: %w", err)
	}
	defer file.Close()

	// Fill header section
	if err := f.fillHeaderSection(file, data); err != nil {
		return "", fmt.Errorf("failed to fill header: %w", err)
	}

	// Fill item rows
	if err := f.fillItemRows(file, data.Items); err != nil {
		return "", fmt.Errorf("failed to fill items: %w", err)
	}

	// Save to output path
	// Use SaveAs with Options to prevent CJK font errors
	opts := excelize.Options{
		// Disable password protection
		Password: "",
	}
	if err := file.SaveAs(outputPath, opts); err != nil {
		return "", fmt.Errorf("failed to save file: %w", err)
	}

	f.logger.Info("Reimbursement form filled successfully",
		zap.String("lark_instance_id", data.LarkInstanceID),
		zap.String("output_path", outputPath),
		zap.Int("item_count", len(data.Items)))

	return outputPath, nil
}

// fillHeaderSection fills rows 1-4 with instance metadata
func (f *ReimbursementFormFiller) fillHeaderSection(file *excelize.File, data *FormData) error {
	// Applicant name (B4)
	if err := file.SetCellValue(sheetName, cellApplicant, data.ApplicantName); err != nil {
		return fmt.Errorf("failed to set applicant: %w", err)
	}

	// Department (E4)
	if err := file.SetCellValue(sheetName, cellDepartment, data.Department); err != nil {
		return fmt.Errorf("failed to set department: %w", err)
	}

	// Lark Instance ID (H4)
	if err := file.SetCellValue(sheetName, cellInstanceID, data.LarkInstanceID); err != nil {
		return fmt.Errorf("failed to set instance ID: %w", err)
	}

	return nil
}

// fillItemRows fills rows 6-13 with expense items
func (f *ReimbursementFormFiller) fillItemRows(file *excelize.File, items []FormItemData) error {
	for i, item := range items {
		if i >= maxItems {
			f.logger.Warn("Item count exceeds template capacity, truncating",
				zap.Int("total_items", len(items)),
				zap.Int("max_items", maxItems))
			break
		}

		row := dataRowStart + i

		// Sequence number (A column)
		cell := fmt.Sprintf("%s%d", colSequence, row)
		if err := file.SetCellValue(sheetName, cell, item.SequenceNumber); err != nil {
			return fmt.Errorf("failed to set sequence at row %d: %w", row, err)
		}

		// Accounting subject (B column)
		cell = fmt.Sprintf("%s%d", colSubject, row)
		if err := file.SetCellValue(sheetName, cell, item.AccountingSubject); err != nil {
			return fmt.Errorf("failed to set subject at row %d: %w", row, err)
		}

		// Description (C column)
		cell = fmt.Sprintf("%s%d", colDescription, row)
		if err := file.SetCellValue(sheetName, cell, item.Description); err != nil {
			return fmt.Errorf("failed to set description at row %d: %w", row, err)
		}

		// Expense type (D column)
		cell = fmt.Sprintf("%s%d", colExpenseType, row)
		if err := file.SetCellValue(sheetName, cell, item.ExpenseType); err != nil {
			return fmt.Errorf("failed to set expense type at row %d: %w", row, err)
		}

		// Receipt count (E column)
		cell = fmt.Sprintf("%s%d", colReceiptCnt, row)
		if err := file.SetCellValue(sheetName, cell, item.ReceiptCount); err != nil {
			return fmt.Errorf("failed to set receipt count at row %d: %w", row, err)
		}

		// Amount (F column)
		cell = fmt.Sprintf("%s%d", colAmount, row)
		if err := file.SetCellValue(sheetName, cell, item.Amount); err != nil {
			return fmt.Errorf("failed to set amount at row %d: %w", row, err)
		}

		// Remarks (H column)
		cell = fmt.Sprintf("%s%d", colRemarks, row)
		if err := file.SetCellValue(sheetName, cell, item.Remarks); err != nil {
			return fmt.Errorf("failed to set remarks at row %d: %w", row, err)
		}
	}

	return nil
}

// ValidateTemplate checks if the template has the expected structure
func (f *ReimbursementFormFiller) ValidateTemplate() error {
	file, err := excelize.OpenFile(f.templatePath)
	if err != nil {
		return fmt.Errorf("failed to open template: %w", err)
	}
	defer file.Close()

	// Check if the expected sheet exists
	sheets := file.GetSheetList()
	found := false
	for _, s := range sheets {
		if s == sheetName {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("template missing expected sheet: %s", sheetName)
	}

	// Validate expected header positions exist
	titleCell, err := file.GetCellValue(sheetName, "A1")
	if err != nil {
		return fmt.Errorf("failed to read title cell: %w", err)
	}
	if titleCell == "" {
		f.logger.Warn("Template title cell A1 is empty")
	}

	return nil
}
