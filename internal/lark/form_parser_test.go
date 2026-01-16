package lark

import (
	"testing"
	"time"

	"github.com/garyjia/ai-reimbursement/internal/models"
	"go.uber.org/zap"
)

func TestFormParserBasicExtraction(t *testing.T) {
	parser := NewFormParser(zap.NewNop())

	// Test basic form structure with required fields
	formJSON := `{
		"form": {
			"amount": 150.50,
			"description": "Conference ticket",
			"expense_date": "2024-01-15",
			"vendor": "TechConf Inc.",
			"business_purpose": "Annual tech conference attendance"
		}
	}`

	items, err := parser.Parse(formJSON)
	if err != nil {
		t.Fatalf("Failed to parse form: %v", err)
	}

	if len(items) != 1 {
		t.Fatalf("Expected 1 item, got %d", len(items))
	}

	item := items[0]
	if item.Amount != 150.50 {
		t.Errorf("Expected amount 150.50, got %f", item.Amount)
	}
	if item.Description != "Conference ticket" {
		t.Errorf("Expected description 'Conference ticket', got '%s'", item.Description)
	}
	if item.Vendor != "TechConf Inc." {
		t.Errorf("Expected vendor 'TechConf Inc.', got '%s'", item.Vendor)
	}
	if item.BusinessPurpose != "Annual tech conference attendance" {
		t.Errorf("Expected purpose, got '%s'", item.BusinessPurpose)
	}
	if item.ExpenseDate == nil {
		t.Error("Expected expense date to be set")
	}
}

func TestFormParserMultipleItems(t *testing.T) {
	parser := NewFormParser(zap.NewNop())

	// Test form with multiple items (array structure)
	formJSON := `{
		"reimbursement_items": [
			{
				"amount": 100.0,
				"description": "Flight ticket",
				"item_type": "TRAVEL",
				"expense_date": "2024-01-10",
				"vendor": "China Eastern Airlines"
			},
			{
				"amount": 50.0,
				"description": "Hotel accommodation",
				"item_type": "ACCOMMODATION",
				"expense_date": "2024-01-10",
				"vendor": "Marriott"
			}
		]
	}`

	items, err := parser.Parse(formJSON)
	if err != nil {
		t.Fatalf("Failed to parse form: %v", err)
	}

	if len(items) != 2 {
		t.Fatalf("Expected 2 items, got %d", len(items))
	}

	if items[0].Amount != 100.0 {
		t.Errorf("Expected first item amount 100.0, got %f", items[0].Amount)
	}
	if items[1].Amount != 50.0 {
		t.Errorf("Expected second item amount 50.0, got %f", items[1].Amount)
	}
}

func TestFormParserIndexedFields(t *testing.T) {
	parser := NewFormParser(zap.NewNop())

	// Test form with indexed fields (description_1, amount_1, etc.)
	formJSON := `{
		"description_1": "Meal - Team lunch",
		"amount_1": 80.0,
		"vendor_1": "Restaurant A",
		"business_purpose_1": "Team building",
		"description_2": "Office supplies",
		"amount_2": 120.0,
		"vendor_2": "Office Depot",
		"business_purpose_2": "Monthly supplies"
	}`

	items, err := parser.Parse(formJSON)
	if err != nil {
		t.Fatalf("Failed to parse form: %v", err)
	}

	if len(items) != 2 {
		t.Fatalf("Expected 2 items, got %d", len(items))
	}

	// Check first item
	if items[0].Amount != 80.0 {
		t.Errorf("Expected first item amount 80.0, got %f", items[0].Amount)
	}
	if items[0].Description != "Meal - Team lunch" {
		t.Errorf("Expected description 'Meal - Team lunch', got '%s'", items[0].Description)
	}

	// Check second item
	if items[1].Amount != 120.0 {
		t.Errorf("Expected second item amount 120.0, got %f", items[1].Amount)
	}
}

func TestFormParserItemTypeInference(t *testing.T) {
	parser := NewFormParser(zap.NewNop())

	tests := []struct {
		description  string
		expectedType string
	}{
		{"Flight ticket to Beijing", models.ItemTypeTravel},
		{"Hotel accommodation 3 nights", models.ItemTypeAccommodation},
		{"Lunch with client", models.ItemTypeMeal},
		{"Laptop software license", models.ItemTypeEquipment},
	}

	for _, tt := range tests {
		formJSON := `{
			"form": {
				"amount": 100.0,
				"description": "` + tt.description + `"
			}
		}`

		items, err := parser.Parse(formJSON)
		if err != nil {
			t.Fatalf("Failed to parse form for %s: %v", tt.description, err)
		}

		if items[0].ItemType != tt.expectedType {
			t.Errorf("For description '%s', expected type %s, got %s",
				tt.description, tt.expectedType, items[0].ItemType)
		}
	}
}

func TestFormParserDateParsing(t *testing.T) {
	parser := NewFormParser(zap.NewNop())

	tests := []struct {
		dateStr string
	}{
		{"2024-01-15"},
		{"2024/01/15"},
		{"20240115"},
	}

	for _, tt := range tests {
		formJSON := `{
			"form": {
				"amount": 100.0,
				"description": "Test",
				"expense_date": "` + tt.dateStr + `"
			}
		}`

		items, err := parser.Parse(formJSON)
		if err != nil {
			t.Fatalf("Failed to parse form with date %s: %v", tt.dateStr, err)
		}

		if items[0].ExpenseDate == nil {
			t.Errorf("Expected expense date to be parsed for format %s", tt.dateStr)
		} else if items[0].ExpenseDate.Year() != 2024 || items[0].ExpenseDate.Month() != time.January {
			t.Errorf("Expected date 2024-01-15, got %v", items[0].ExpenseDate)
		}
	}
}

func TestFormParserMissingRequiredFields(t *testing.T) {
	parser := NewFormParser(zap.NewNop())

	// Missing amount (required)
	formJSON := `{
		"form": {
			"description": "Test item"
		}
	}`

	_, err := parser.Parse(formJSON)
	if err == nil {
		t.Error("Expected error when amount is missing")
	}

	// Missing description (required)
	formJSON = `{
		"form": {
			"amount": 100.0
		}
	}`

	_, err = parser.Parse(formJSON)
	if err == nil {
		t.Error("Expected error when description is missing")
	}
}

func TestFormParserNumericAmountVariations(t *testing.T) {
	parser := NewFormParser(zap.NewNop())

	// Test that numeric amounts are parsed correctly
	formJSON := `{
		"form": {
			"amount": 100.0,
			"description": "Test"
		}
	}`

	items, err := parser.Parse(formJSON)
	if err != nil {
		t.Fatalf("Failed to parse form: %v", err)
	}

	if items[0].Amount != 100.0 {
		t.Errorf("Expected amount %f, got %f", 100.0, items[0].Amount)
	}
}

func TestFormParserEmptyData(t *testing.T) {
	parser := NewFormParser(zap.NewNop())

	_, err := parser.Parse("")
	if err == nil {
		t.Error("Expected error when parsing empty string")
	}

	_, err = parser.Parse("{}")
	if err == nil {
		t.Error("Expected error when parsing empty JSON object")
	}
}

func TestFormParserCurrencyDefault(t *testing.T) {
	parser := NewFormParser(zap.NewNop())

	formJSON := `{
		"form": {
			"amount": 100.0,
			"description": "Test"
		}
	}`

	items, err := parser.Parse(formJSON)
	if err != nil {
		t.Fatalf("Failed to parse form: %v", err)
	}

	if items[0].Currency != "CNY" {
		t.Errorf("Expected currency CNY, got %s", items[0].Currency)
	}
}

func TestFormParserChineseFieldNames(t *testing.T) {
	parser := NewFormParser(zap.NewNop())

	// Test form with Chinese field names
	formJSON := `{
		"form": {
			"金额": 150.0,
			"摘要": "出差交通费",
			"商户": "滴滴出行",
			"用途": "出差去客户现场",
			"日期": "2024-01-15"
		}
	}`

	items, err := parser.Parse(formJSON)
	if err != nil {
		t.Fatalf("Failed to parse form with Chinese names: %v", err)
	}

	if len(items) != 1 {
		t.Fatalf("Expected 1 item, got %d", len(items))
	}

	if items[0].Amount != 150.0 {
		t.Errorf("Expected amount 150.0, got %f", items[0].Amount)
	}
	if items[0].Description != "出差交通费" {
		t.Errorf("Expected description '出差交通费', got '%s'", items[0].Description)
	}
	if items[0].Vendor != "滴滴出行" {
		t.Errorf("Expected vendor '滴滴出行', got '%s'", items[0].Vendor)
	}
}
