package voucher

import (
	"encoding/json"
	"fmt"

	"github.com/garyjia/ai-reimbursement/internal/models"
	"github.com/xuri/excelize/v2"
	"go.uber.org/zap"
)

// ExcelFiller fills Excel templates with reimbursement data
type ExcelFiller struct {
	templatePath string
	companyName  string
	companyTaxID string
	logger       *zap.Logger
}

// NewExcelFiller creates a new Excel filler
func NewExcelFiller(templatePath, companyName, companyTaxID string, logger *zap.Logger) (*ExcelFiller, error) {
	return &ExcelFiller{
		templatePath: templatePath,
		companyName:  companyName,
		companyTaxID: companyTaxID,
		logger:       logger,
	}, nil
}

// FillTemplate fills the Excel template with instance data
func (ef *ExcelFiller) FillTemplate(instance *models.ApprovalInstance, voucherNumber, outputPath string) error {
	ef.logger.Info("Filling Excel template",
		zap.Int64("instance_id", instance.ID),
		zap.String("voucher_number", voucherNumber))

	// Open template
	f, err := excelize.OpenFile(ef.templatePath)
	if err != nil {
		return fmt.Errorf("failed to open template: %w", err)
	}
	defer f.Close()

	// Get the first sheet (assuming single sheet template)
	sheets := f.GetSheetList()
	if len(sheets) == 0 {
		return fmt.Errorf("template has no sheets")
	}
	sheetName := sheets[0]

	// Parse form data
	var formData map[string]interface{}
	if err := json.Unmarshal([]byte(instance.FormData), &formData); err != nil {
		return fmt.Errorf("failed to parse form data: %w", err)
	}

	// Fill basic information
	ef.setCell(f, sheetName, "B2", ef.companyName)                               // Company name
	ef.setCell(f, sheetName, "B3", ef.companyTaxID)                              // Tax ID
	ef.setCell(f, sheetName, "B4", voucherNumber)                                // Voucher number
	ef.setCell(f, sheetName, "B5", instance.SubmissionTime.Format("2006-01-02")) // Submission date
	ef.setCell(f, sheetName, "B6", instance.ApplicantUserID)                     // Applicant ID
	ef.setCell(f, sheetName, "B7", instance.Department)                          // Department

	// Fill accounting period (会计期间)
	accountingPeriod := instance.SubmissionTime.Format("2006年01月")
	ef.setCell(f, sheetName, "B8", accountingPeriod)

	// TODO: Fill itemized expense table
	// This depends on the actual template structure
	// For now, we'll add a note that items need to be filled

	// Calculate total amount
	totalAmount := ef.calculateTotalAmount(formData)
	ef.setCell(f, sheetName, "D20", fmt.Sprintf("%.2f", totalAmount))

	// Convert amount to Chinese capitalization (大写金额)
	chineseAmount := ef.numberToChinese(totalAmount)
	ef.setCell(f, sheetName, "D21", chineseAmount)

	// Fill approval information if available
	if instance.ApprovalTime != nil {
		ef.setCell(f, sheetName, "B25", instance.ApprovalTime.Format("2006-01-02"))
	}

	// Save to output path
	if err := f.SaveAs(outputPath); err != nil {
		return fmt.Errorf("failed to save Excel file: %w", err)
	}

	ef.logger.Info("Excel template filled successfully",
		zap.String("output_path", outputPath))

	return nil
}

// setCell sets a cell value in the Excel file
func (ef *ExcelFiller) setCell(f *excelize.File, sheet, cell, value string) {
	if err := f.SetCellValue(sheet, cell, value); err != nil {
		ef.logger.Warn("Failed to set cell value",
			zap.String("sheet", sheet),
			zap.String("cell", cell),
			zap.Error(err))
	}
}

// calculateTotalAmount calculates the total reimbursement amount
func (ef *ExcelFiller) calculateTotalAmount(formData map[string]interface{}) float64 {
	// TODO: Parse actual form structure to extract amounts
	// This is a placeholder implementation
	total := 0.0

	if items, ok := formData["items"].([]interface{}); ok {
		for _, item := range items {
			if itemMap, ok := item.(map[string]interface{}); ok {
				if amount, ok := itemMap["amount"].(float64); ok {
					total += amount
				}
			}
		}
	}

	return total
}

// numberToChinese converts a number to Chinese capitalization
func (ef *ExcelFiller) numberToChinese(amount float64) string {
	// Chinese number characters
	digits := []string{"零", "壹", "贰", "叁", "肆", "伍", "陆", "柒", "捌", "玖"}
	units := []string{"", "拾", "佰", "仟"}
	bigUnits := []string{"", "万", "亿"}

	if amount == 0 {
		return "零元整"
	}

	// Split into integer and decimal parts
	yuan := int(amount)
	jiao := int((amount - float64(yuan)) * 10)
	fen := int((amount - float64(yuan) - float64(jiao)/10) * 100)

	result := ""

	// Convert yuan (integer part)
	if yuan == 0 {
		result = "零"
	} else {
		result = ef.convertInteger(yuan, digits, units, bigUnits)
	}

	result += "元"

	// Convert decimal part
	if jiao == 0 && fen == 0 {
		result += "整"
	} else {
		if jiao != 0 {
			result += digits[jiao] + "角"
		}
		if fen != 0 {
			if jiao == 0 {
				result += "零"
			}
			result += digits[fen] + "分"
		}
	}

	return result
}

// convertInteger converts integer part to Chinese
func (ef *ExcelFiller) convertInteger(num int, digits, units, bigUnits []string) string {
	if num == 0 {
		return ""
	}

	result := ""
	unitIndex := 0
	bigUnitIndex := 0
	lastZero := false

	for num > 0 {
		digit := num % 10
		num = num / 10

		if digit == 0 {
			lastZero = true
		} else {
			if lastZero && result != "" {
				result = digits[0] + result
			}
			result = digits[digit] + units[unitIndex%4] + result
			lastZero = false
		}

		unitIndex++
		if unitIndex%4 == 0 {
			if result != "" && result[0:3] != digits[0] {
				result = bigUnits[bigUnitIndex] + result
			}
			bigUnitIndex++
		}
	}

	return result
}
