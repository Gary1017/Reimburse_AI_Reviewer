package lark

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/garyjia/ai-reimbursement/internal/models"
	"go.uber.org/zap"
)

// FormParser parses Lark approval form data into reimbursement items
type FormParser struct {
	logger            *zap.Logger
	attachmentHandler *AttachmentHandler
}

// NewFormParser creates a new form parser
func NewFormParser(logger *zap.Logger) *FormParser {
	return &FormParser{
		logger:            logger,
		attachmentHandler: NewAttachmentHandler(logger, "/tmp/attachments"), // Default attachment dir
	}
}

// NewFormParserWithAttachmentHandler creates form parser with custom attachment handler
func NewFormParserWithAttachmentHandler(logger *zap.Logger, handler *AttachmentHandler) *FormParser {
	return &FormParser{
		logger:            logger,
		attachmentHandler: handler,
	}
}

// NewFormParserWithAttachmentSupport creates form parser with attachment support
// Used in tests
func NewFormParserWithAttachmentSupport(logger *zap.Logger) *FormParser {
	return &FormParser{
		logger:            logger,
		attachmentHandler: NewAttachmentHandler(logger, "/tmp/attachments"),
	}
}

// Parse transforms raw Lark form JSON into reimbursement items
func (fp *FormParser) Parse(jsonData string) ([]*models.ReimbursementItem, error) {
	if jsonData == "" {
		return nil, fmt.Errorf("empty form data")
	}

	var rawData map[string]interface{}
	if err := json.Unmarshal([]byte(jsonData), &rawData); err != nil {
		fp.logger.Error("Failed to unmarshal form data", zap.Error(err))
		return nil, fmt.Errorf("failed to parse form JSON: %w", err)
	}

	var items []*models.ReimbursementItem

	// Try to extract items from common Lark form structures
	// Lark forms typically have a "form" or "widgets" field containing form fields
	formFields := fp.extractFormFields(rawData)

	if len(formFields) == 0 {
		fp.logger.Warn("No form fields found in approval data")
		return nil, fmt.Errorf("no form fields extracted from data")
	}

	// Try to identify reimbursement items structure
	// This could be in a table-like structure or repeated fields
	itemsData := fp.extractItemsData(rawData, formFields)

	if len(itemsData) == 0 {
		fp.logger.Warn("No reimbursement items found in approval data")
		return nil, fmt.Errorf("no reimbursement items found")
	}

	for idx, itemData := range itemsData {
		item, err := fp.parseItem(itemData)
		if err != nil {
			fp.logger.Warn("Failed to parse item",
				zap.Int("index", idx),
				zap.Error(err))
			continue
		}
		if item != nil {
			items = append(items, item)
		}
	}

	if len(items) == 0 {
		return nil, fmt.Errorf("no valid reimbursement items parsed")
	}

	fp.logger.Info("Successfully parsed reimbursement items",
		zap.Int("count", len(items)))

	return items, nil
}

// ParseWithAttachments parses form data and also extracts attachments
// Implements ARCH-001: Parse items and attachments without blocking each other
func (fp *FormParser) ParseWithAttachments(jsonData string) ([]*models.ReimbursementItem, []*models.AttachmentReference, error) {
	// Parse items normally (this should not fail due to attachment issues)
	items, err := fp.Parse(jsonData)
	if err != nil {
		fp.logger.Warn("Failed to parse items, continuing with attachment extraction",
			zap.Error(err))
		items = nil
	}

	// Extract attachments independently (ARCH-001: extraction doesn't block)
	attachments, attachErr := fp.attachmentHandler.ExtractAttachmentURLs(jsonData)
	if attachErr != nil {
		fp.logger.Warn("Failed to extract attachments",
			zap.Error(attachErr))
		// Don't fail the whole parse, attachments are optional
	}

	// If attachments exist, link them to items
	if len(attachments) > 0 && len(items) > 0 {
		// Link first attachment to first item by default
		// In real scenario, would need better mapping from widget to item
		attachments[0].ItemID = items[0].ID
	}

	return items, attachments, err
}

// extractFormFields extracts all form fields from the raw data
func (fp *FormParser) extractFormFields(rawData map[string]interface{}) map[string]interface{} {
	fields := make(map[string]interface{})

	// Lark API returns form data in different possible structures
	// Try common field names
	if form, ok := rawData["form"].(map[string]interface{}); ok {
		for k, v := range form {
			fields[k] = v
		}
		// If form object exists, return early to avoid mixing
		return fields
	}

	// Handle Lark form structure where "form" is a JSON string containing widgets array
	if formStr, ok := rawData["form"].(string); ok {
		var widgets []interface{}
		if err := json.Unmarshal([]byte(formStr), &widgets); err == nil {
			// Store widgets array for later processing
			fields["_lark_widgets"] = widgets
			return fields
		}
	}

	if widgets, ok := rawData["widgets"].([]interface{}); ok {
		for _, w := range widgets {
			if widget, ok := w.(map[string]interface{}); ok {
				// Widget might have id, name, value
				if name, ok := widget["name"].(string); ok {
					fields[name] = widget
				}
				if id, ok := widget["id"].(string); ok {
					fields[id] = widget
				}
			}
		}
	}

	// Also copy top-level fields that might be form data (but skip meta fields)
	for k, v := range rawData {
		if !strings.Contains(strings.ToLower(k), "id") && k != "form" && k != "widgets" && k != "header" && k != "event" {
			fields[k] = v
		}
	}

	return fields
}

// extractItemsData extracts item-level data from the form
func (fp *FormParser) extractItemsData(rawData map[string]interface{}, formFields map[string]interface{}) []map[string]interface{} {
	var items []map[string]interface{}

	// Handle Lark widget-based form structure (from formFields)
	if widgets, ok := formFields["_lark_widgets"].([]interface{}); ok {
		items = fp.extractItemsFromLarkWidgets(widgets)
		if len(items) > 0 {
			return items
		}
	}

	// Also check rawData directly for Lark form string
	if formStr, ok := rawData["form"].(string); ok {
		var widgets []interface{}
		if err := json.Unmarshal([]byte(formStr), &widgets); err == nil {
			items = fp.extractItemsFromLarkWidgets(widgets)
			if len(items) > 0 {
				return items
			}
		}
	}

	// Look for common item container structures
	if reimbursementItems, ok := rawData["reimbursement_items"].([]interface{}); ok {
		for _, item := range reimbursementItems {
			if itemMap, ok := item.(map[string]interface{}); ok {
				items = append(items, itemMap)
			}
		}
		if len(items) > 0 {
			return items
		}
	}

	if expenseItems, ok := rawData["expense_items"].([]interface{}); ok {
		for _, item := range expenseItems {
			if itemMap, ok := item.(map[string]interface{}); ok {
				items = append(items, itemMap)
			}
		}
		if len(items) > 0 {
			return items
		}
	}

	if tableData, ok := rawData["table_data"].([]interface{}); ok {
		for _, row := range tableData {
			if rowMap, ok := row.(map[string]interface{}); ok {
				items = append(items, rowMap)
			}
		}
		if len(items) > 0 {
			return items
		}
	}

	// If formFields have indexed patterns, try to infer items from them
	items = fp.inferItemsFromFields(formFields)
	if len(items) > 0 {
		return items
	}

	// If all else fails, treat formFields itself as a single item (for simple forms)
	if len(formFields) > 0 && formFields["_lark_widgets"] == nil {
		items = append(items, formFields)
	}

	return items
}

// extractItemsFromLarkWidgets extracts items from Lark widget structure
func (fp *FormParser) extractItemsFromLarkWidgets(widgets []interface{}) []map[string]interface{} {
	var items []map[string]interface{}
	var defaultItemType string
	var defaultBusinessPurpose string

	// First pass: Extract form-level fields (reimbursement type, reason, etc.)
	for _, w := range widgets {
		widget, ok := w.(map[string]interface{})
		if !ok {
			continue
		}

		widgetType, _ := widget["type"].(string)
		name, _ := widget["name"].(string)
		value := widget["value"]

		// Extract reimbursement type (报销类型) from radio widget
		if widgetType == "radioV2" && (name == "报销类型" || name == "reimbursement_type") {
			if valStr, ok := value.(string); ok {
				defaultItemType = fp.mapLarkReimbursementType(valStr)
			}
		}

		// Extract reimbursement reason (报销事由) from textarea
		if widgetType == "textarea" && (name == "报销事由" || name == "reimbursement_reason") {
			if valStr, ok := value.(string); ok {
				defaultBusinessPurpose = valStr
			}
		}
	}

	// Second pass: Extract items from fieldList widget (费用明细)
	for _, w := range widgets {
		widget, ok := w.(map[string]interface{})
		if !ok {
			continue
		}

		widgetType, _ := widget["type"].(string)
		if widgetType != "fieldList" {
			continue
		}

		// Extract value array from fieldList widget
		value, ok := widget["value"].([]interface{})
		if !ok {
			continue
		}

		// Each element in value array is a row (expense item)
		for _, row := range value {
			rowArray, ok := row.([]interface{})
			if !ok {
				continue
			}

			// Convert row widgets to a map
			itemData := make(map[string]interface{})
			for _, cell := range rowArray {
				cellWidget, ok := cell.(map[string]interface{})
				if !ok {
					continue
				}

				// Extract widget name and value
				name, _ := cellWidget["name"].(string)
				value := cellWidget["value"]

				// Map Lark widget names to our field names
				if name == "内容" || name == "内容描述" || name == "description" {
					itemData["description"] = value
				} else if name == "日期（年-月-日）" || name == "日期" || name == "date" || name == "expense_date" {
					itemData["expense_date"] = value
				} else if name == "金额" || name == "amount" {
					itemData["amount"] = value
				} else if name == "商户" || name == "vendor" || name == "merchant" {
					itemData["vendor"] = value
				} else if name == "用途" || name == "business_purpose" || name == "purpose" {
					itemData["business_purpose"] = value
				} else if name == "报销类型" || name == "item_type" || name == "category" {
					itemData["item_type"] = value
				} else {
					// Store other fields as-is
					itemData[name] = value
				}
			}

			// Apply defaults if not set in item
			if defaultItemType != "" && itemData["item_type"] == nil {
				itemData["item_type"] = defaultItemType
			}
			if defaultBusinessPurpose != "" && itemData["business_purpose"] == nil {
				itemData["business_purpose"] = defaultBusinessPurpose
			}

			if len(itemData) > 0 {
				items = append(items, itemData)
			}
		}
	}

	return items
}

// mapLarkReimbursementType maps Lark reimbursement type names to our item types
func (fp *FormParser) mapLarkReimbursementType(larkType string) string {
	larkTypeLower := strings.ToLower(larkType)

	// Map common Lark reimbursement types
	if strings.Contains(larkTypeLower, "住宿") || strings.Contains(larkTypeLower, "accommodation") {
		return models.ItemTypeAccommodation
	}
	if strings.Contains(larkTypeLower, "交通") || strings.Contains(larkTypeLower, "travel") || strings.Contains(larkTypeLower, "差旅") {
		return models.ItemTypeTravel
	}
	if strings.Contains(larkTypeLower, "餐饮") || strings.Contains(larkTypeLower, "meal") || strings.Contains(larkTypeLower, "餐费") {
		return models.ItemTypeMeal
	}
	if strings.Contains(larkTypeLower, "设备") || strings.Contains(larkTypeLower, "equipment") || strings.Contains(larkTypeLower, "办公") {
		return models.ItemTypeEquipment
	}

	return models.ItemTypeOther
}

// inferItemsFromFields tries to infer item data from individual form fields
// This handles cases where items are stored as separate fields rather than arrays
func (fp *FormParser) inferItemsFromFields(formFields map[string]interface{}) []map[string]interface{} {
	var items []map[string]interface{}

	// Group fields by index (e.g., amount_1, amount_2, vendor_1, vendor_2)
	indexedFields := make(map[int]map[string]interface{})
	hasIndexedFields := false

	for fieldName, fieldValue := range formFields {
		// Try to extract index from field name (e.g., "description_1" -> index 1)
		idx, baseName := fp.extractFieldIndex(fieldName)
		if idx >= 0 {
			hasIndexedFields = true
			if _, ok := indexedFields[idx]; !ok {
				indexedFields[idx] = make(map[string]interface{})
			}
			indexedFields[idx][baseName] = fieldValue
		}
	}

	// Convert indexed fields to items in proper order
	if hasIndexedFields && len(indexedFields) > 0 {
		// Find max index
		maxIdx := 0
		for idx := range indexedFields {
			if idx > maxIdx {
				maxIdx = idx
			}
		}
		// Add items in order
		for i := 1; i <= maxIdx; i++ {
			if item, ok := indexedFields[i]; ok && len(item) > 0 {
				items = append(items, item)
			}
		}
	}

	return items
}

// extractFieldIndex extracts the index from field names like "description_1" -> (1, "description")
func (fp *FormParser) extractFieldIndex(fieldName string) (int, string) {
	// Look for patterns like field_1, field_2, etc.
	parts := strings.Split(fieldName, "_")
	if len(parts) < 2 {
		return -1, fieldName
	}

	lastPart := parts[len(parts)-1]
	if idx, err := strconv.Atoi(lastPart); err == nil {
		baseName := strings.Join(parts[:len(parts)-1], "_")
		return idx, baseName
	}

	return -1, fieldName
}

// parseItem parses a single item from item data
func (fp *FormParser) parseItem(itemData map[string]interface{}) (*models.ReimbursementItem, error) {
	item := &models.ReimbursementItem{
		Currency: "CNY", // Default currency
	}

	// Extract amount (required)
	amountValue := fp.extractField(itemData, []string{"amount", "钱数", "金额", "expense_amount"})
	if amountValue == nil {
		return nil, fmt.Errorf("amount field not found")
	}

	amount, err := fp.parseAmount(amountValue)
	if err != nil {
		return nil, fmt.Errorf("failed to parse amount: %w", err)
	}
	item.Amount = amount

	// Extract description
	description := fp.extractFieldAsString(itemData, []string{"description", "摘要", "说明", "expense_description", "expense_notes"})
	if description == "" {
		return nil, fmt.Errorf("description field not found")
	}
	item.Description = description

	// Extract item type
	itemType := fp.extractFieldAsString(itemData, []string{"item_type", "category", "expense_category", "type", "类型"})
	if itemType == "" {
		// Try to infer from description
		itemType = fp.inferItemType(description)
	}
	item.ItemType = itemType

	// Extract optional fields
	// Expense date
	expenseDateStr := fp.extractFieldAsString(itemData, []string{"expense_date", "date", "日期", "transaction_date"})
	if expenseDateStr != "" {
		if expenseDate, err := fp.parseDate(expenseDateStr); err == nil {
			item.ExpenseDate = &expenseDate
		}
	}

	// Vendor/merchant
	vendor := fp.extractFieldAsString(itemData, []string{"vendor", "merchant", "supplier", "merchant_name", "vendor_name", "商户", "供应商"})
	item.Vendor = vendor

	// Business purpose
	purpose := fp.extractFieldAsString(itemData, []string{"business_purpose", "purpose", "business_reason", "business_justification", "用途", "事由"})
	item.BusinessPurpose = purpose

	// Receipt attachment
	receipt := fp.extractFieldAsString(itemData, []string{"receipt", "receipt_attachment", "receipt_file", "invoice", "file", "attachment"})
	item.ReceiptAttachment = receipt

	fp.logger.Debug("Parsed reimbursement item",
		zap.String("description", item.Description),
		zap.Float64("amount", item.Amount),
		zap.String("type", item.ItemType),
		zap.String("vendor", item.Vendor))

	return item, nil
}

// extractField extracts a field value by trying multiple possible field names
func (fp *FormParser) extractField(data map[string]interface{}, possibleNames []string) interface{} {
	for _, name := range possibleNames {
		if val, ok := data[name]; ok && val != nil {
			return val
		}
		// Try case-insensitive match
		for k, v := range data {
			if strings.EqualFold(k, name) && v != nil {
				return v
			}
		}
	}
	return nil
}

// extractFieldAsString extracts a field as a string
func (fp *FormParser) extractFieldAsString(data map[string]interface{}, possibleNames []string) string {
	val := fp.extractField(data, possibleNames)
	if val == nil {
		return ""
	}

	switch v := val.(type) {
	case string:
		return strings.TrimSpace(v)
	case float64:
		return fmt.Sprintf("%.0f", v)
	case int:
		return fmt.Sprintf("%d", v)
	case map[string]interface{}:
		// Might be a nested object, try to extract text
		if text, ok := v["text"].(string); ok {
			return strings.TrimSpace(text)
		}
		if text, ok := v["value"].(string); ok {
			return strings.TrimSpace(text)
		}
		return ""
	default:
		return fmt.Sprintf("%v", v)
	}
}

// parseAmount parses a numeric amount
func (fp *FormParser) parseAmount(val interface{}) (float64, error) {
	switch v := val.(type) {
	case float64:
		return v, nil
	case int:
		return float64(v), nil
	case string:
		// Try to parse as float
		var f float64
		_, err := fmt.Sscanf(strings.TrimSpace(v), "%f", &f)
		if err != nil {
			return 0, err
		}
		return f, nil
	case map[string]interface{}:
		// Might be a nested object with a value field
		if num, ok := v["value"].(float64); ok {
			return num, nil
		}
		if num, ok := v["amount"].(float64); ok {
			return num, nil
		}
		return 0, fmt.Errorf("cannot extract amount from object")
	default:
		return 0, fmt.Errorf("unsupported amount type: %T", val)
	}
}

// parseDate parses a date string in various formats
func (fp *FormParser) parseDate(dateStr string) (time.Time, error) {
	dateStr = strings.TrimSpace(dateStr)

	// Handle Lark date format with timezone: "2026-01-13T00:00:00+08:00"
	// Try RFC3339 first (handles timezone)
	if t, err := time.Parse(time.RFC3339, dateStr); err == nil {
		return t, nil
	}

	// Try RFC3339Nano
	if t, err := time.Parse(time.RFC3339Nano, dateStr); err == nil {
		return t, nil
	}

	// Try common date formats
	formats := []string{
		"2006-01-02",
		"2006/01/02",
		"20060102",
		"01-02-2006",
		"01/02/2006",
	}

	for _, format := range formats {
		if t, err := time.Parse(format, dateStr); err == nil {
			return t, nil
		}
	}

	return time.Time{}, fmt.Errorf("failed to parse date: %s", dateStr)
}

// inferItemType infers the item type from the description
func (fp *FormParser) inferItemType(description string) string {
	desc := strings.ToLower(description)

	if strings.Contains(desc, "flight") || strings.Contains(desc, "airline") || strings.Contains(desc, "机票") ||
		strings.Contains(desc, "train") || strings.Contains(desc, "taxi") || strings.Contains(desc, "car") ||
		strings.Contains(desc, "travel") || strings.Contains(desc, "trip") {
		return models.ItemTypeTravel
	}
	if strings.Contains(desc, "hotel") || strings.Contains(desc, "accommodation") || strings.Contains(desc, "住宿") ||
		strings.Contains(desc, "lodge") || strings.Contains(desc, "stay") {
		return models.ItemTypeAccommodation
	}
	if strings.Contains(desc, "meal") || strings.Contains(desc, "lunch") || strings.Contains(desc, "dinner") ||
		strings.Contains(desc, "breakfast") || strings.Contains(desc, "restaurant") || strings.Contains(desc, "食") ||
		strings.Contains(desc, "food") || strings.Contains(desc, "coffee") {
		return models.ItemTypeMeal
	}
	if strings.Contains(desc, "equipment") || strings.Contains(desc, "device") || strings.Contains(desc, "软件") ||
		strings.Contains(desc, "设备") || strings.Contains(desc, "laptop") || strings.Contains(desc, "computer") ||
		strings.Contains(desc, "software") || strings.Contains(desc, "license") || strings.Contains(desc, "office") ||
		strings.Contains(desc, "supplies") {
		return models.ItemTypeEquipment
	}

	return models.ItemTypeOther
}
