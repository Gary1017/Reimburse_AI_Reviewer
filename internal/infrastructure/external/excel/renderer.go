package excel

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/garyjia/ai-reimbursement/internal/application/port"
	"github.com/garyjia/ai-reimbursement/internal/domain/entity"
	"github.com/xuri/excelize/v2"
	"go.uber.org/zap"
)

// VoucherRenderer implements port.VoucherRenderer using excelize
type VoucherRenderer struct {
	logger *zap.Logger
}

// NewVoucherRenderer creates a new VoucherRenderer
func NewVoucherRenderer(logger *zap.Logger) *VoucherRenderer {
	return &VoucherRenderer{
		logger: logger,
	}
}

// Render fills the voucher template with data and saves to outputPath
func (r *VoucherRenderer) Render(ctx context.Context, data *port.VoucherData, templatePath string, outputPath string) (*port.VoucherRenderResult, error) {
	r.logger.Info("rendering voucher",
		zap.String("template", templatePath),
		zap.String("output", outputPath),
		zap.String("applicant", data.ApplicantName),
		zap.Int("items", len(data.Items)),
	)

	// Open template file
	f, err := excelize.OpenFile(templatePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open template: %w", err)
	}
	defer func() {
		if closeErr := f.Close(); closeErr != nil {
			r.logger.Warn("failed to close excel file", zap.Error(closeErr))
		}
	}()

	// Fill header information
	if err := r.fillHeader(f, data); err != nil {
		return nil, fmt.Errorf("failed to fill header: %w", err)
	}

	// Fill items (max 8 items, rows 6-13)
	if err := r.fillItems(f, data); err != nil {
		return nil, fmt.Errorf("failed to fill items: %w", err)
	}

	// Save to output path (not Save - template must remain unchanged)
	if err := f.SaveAs(outputPath); err != nil {
		return nil, fmt.Errorf("failed to save voucher: %w", err)
	}

	r.logger.Info("voucher rendered successfully", zap.String("output", outputPath))

	return &port.VoucherRenderResult{
		FilePath: outputPath,
		FileName: filepath.Base(outputPath),
	}, nil
}

// fillHeader fills the header cells (row 4)
func (r *VoucherRenderer) fillHeader(f *excelize.File, data *port.VoucherData) error {
	sheet := "Sheet1"

	// B4: Applicant name
	if err := f.SetCellValue(sheet, "B4", data.ApplicantName); err != nil {
		return fmt.Errorf("failed to set applicant name: %w", err)
	}

	// E4: Department
	if err := f.SetCellValue(sheet, "E4", data.Department); err != nil {
		return fmt.Errorf("failed to set department: %w", err)
	}

	// H4: Approval ID
	if err := f.SetCellValue(sheet, "H4", data.ApprovalID); err != nil {
		return fmt.Errorf("failed to set approval ID: %w", err)
	}

	return nil
}

// fillItems fills the item rows (rows 6-13, max 8 items)
func (r *VoucherRenderer) fillItems(f *excelize.File, data *port.VoucherData) error {
	sheet := "Sheet1"
	maxItems := 8
	startRow := 6

	// Build attachment count map (item_id -> count of INVOICE attachments)
	invoiceCountByItem := r.buildInvoiceCountMap(data.Attachments)

	// Fill each item (limited to 8)
	itemCount := len(data.Items)
	if itemCount > maxItems {
		r.logger.Warn("too many items, truncating",
			zap.Int("total", itemCount),
			zap.Int("max", maxItems),
		)
		itemCount = maxItems
	}

	for i := 0; i < itemCount; i++ {
		item := data.Items[i]
		row := startRow + i

		// A{row}: Sequence number (1-based)
		if err := f.SetCellValue(sheet, fmt.Sprintf("A%d", row), i+1); err != nil {
			return fmt.Errorf("failed to set sequence for item %d: %w", i, err)
		}

		// B{row}: 会计科目 (accounting category) - use ItemType
		if err := f.SetCellValue(sheet, fmt.Sprintf("B%d", row), item.ItemType); err != nil {
			return fmt.Errorf("failed to set item type for item %d: %w", i, err)
		}

		// C{row}: 报销事项 (description)
		if err := f.SetCellValue(sheet, fmt.Sprintf("C%d", row), item.Description); err != nil {
			return fmt.Errorf("failed to set description for item %d: %w", i, err)
		}

		// D{row}: 报销类型 (item type) - also use ItemType (same as B)
		if err := f.SetCellValue(sheet, fmt.Sprintf("D%d", row), item.ItemType); err != nil {
			return fmt.Errorf("failed to set item type copy for item %d: %w", i, err)
		}

		// E{row}: 凭证张数 (count of INVOICE attachments)
		invoiceCount := invoiceCountByItem[item.ID]
		if err := f.SetCellValue(sheet, fmt.Sprintf("E%d", row), invoiceCount); err != nil {
			return fmt.Errorf("failed to set invoice count for item %d: %w", i, err)
		}

		// F{row}: 报销金额 (amount in yuan)
		amountYuan := float64(item.AmountCents) / 100.0
		if err := f.SetCellValue(sheet, fmt.Sprintf("F%d", row), amountYuan); err != nil {
			return fmt.Errorf("failed to set amount for item %d: %w", i, err)
		}

		// H{row}: 备注 (remarks) - use BusinessPurpose if available
		remark := item.BusinessPurpose
		if remark == "" {
			remark = item.Vendor
		}
		if err := f.SetCellValue(sheet, fmt.Sprintf("H%d", row), remark); err != nil {
			return fmt.Errorf("failed to set remark for item %d: %w", i, err)
		}
	}

	r.logger.Info("filled items",
		zap.Int("count", itemCount),
		zap.Int("max", maxItems),
	)

	return nil
}

// buildInvoiceCountMap counts INVOICE attachments per item
func (r *VoucherRenderer) buildInvoiceCountMap(attachments []*entity.Attachment) map[int64]int {
	countMap := make(map[int64]int)

	for _, att := range attachments {
		if att.FileType == entity.FileTypeInvoice {
			countMap[att.ItemID]++
		}
	}

	return countMap
}
