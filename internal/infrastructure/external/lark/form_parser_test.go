package lark

import (
	"encoding/json"
	"os"
	"testing"

	"go.uber.org/zap"
)

// fixtureDir points to the captured Lark API fixtures.
const fixtureDir = "../../../../testdata/lark_fixtures"

// loadInstanceDetailForm reads the captured instance_detail fixture and
// returns the "form" field exactly as the Lark SDK delivers it (a JSON string
// containing a widgets array).
func loadInstanceDetailForm(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(fixtureDir + "/20260217_172016_01_instance_detail.json")
	if err != nil {
		t.Fatalf("Failed to read instance detail fixture: %v", err)
	}

	var detail map[string]interface{}
	if err := json.Unmarshal(data, &detail); err != nil {
		t.Fatalf("Failed to parse instance detail fixture: %v", err)
	}

	formStr, ok := detail["form"].(string)
	if !ok {
		t.Fatal("Expected 'form' field to be a string in instance detail fixture")
	}
	return formStr
}

// simulateDBFormData mimics what serializeFormData does: json.Marshal on the
// form string, producing a double-encoded JSON string like the DB stores.
func simulateDBFormData(t *testing.T, formStr string) string {
	t.Helper()
	encoded, err := json.Marshal(formStr)
	if err != nil {
		t.Fatalf("Failed to marshal form string: %v", err)
	}
	return string(encoded)
}

func TestFormParser_Parse_RawFormString(t *testing.T) {
	// This is the form value as returned by the Lark API: a plain JSON array string.
	formStr := loadInstanceDetailForm(t)

	logger, _ := zap.NewDevelopment()
	parser := NewFormParser(logger)

	items, err := parser.Parse(formStr)
	if err != nil {
		t.Fatalf("Parse failed on raw form string: %v", err)
	}

	if len(items) == 0 {
		t.Fatal("Expected at least one reimbursement item, got 0")
	}

	item := items[0]
	t.Logf("Parsed item: type=%s, description=%s, amount=%.2f", item.ItemType, item.Description, item.Amount)

	if item.Amount != 900 {
		t.Errorf("Expected amount 900, got %.2f", item.Amount)
	}
	if item.Description != "酒店" {
		t.Errorf("Expected description '酒店', got %q", item.Description)
	}
	if item.ItemType != "ACCOMMODATION" {
		t.Errorf("Expected item type 'ACCOMMODATION', got %q", item.ItemType)
	}
}

func TestFormParser_Parse_DoubleEncodedDBFormat(t *testing.T) {
	// This is the form_data as stored in the DB after serializeFormData:
	// json.Marshal(formString) produces a double-encoded JSON string.
	formStr := loadInstanceDetailForm(t)
	dbFormData := simulateDBFormData(t, formStr)

	// Verify it's actually double-encoded (starts with a quote)
	if dbFormData[0] != '"' {
		t.Fatalf("Expected double-encoded string to start with '\"', got %q", dbFormData[:1])
	}

	logger, _ := zap.NewDevelopment()
	parser := NewFormParser(logger)

	items, err := parser.Parse(dbFormData)
	if err != nil {
		t.Fatalf("Parse failed on double-encoded DB form data: %v", err)
	}

	if len(items) == 0 {
		t.Fatal("Expected at least one reimbursement item, got 0")
	}

	item := items[0]
	if item.Amount != 900 {
		t.Errorf("Expected amount 900, got %.2f", item.Amount)
	}
	if item.Description != "酒店" {
		t.Errorf("Expected description '酒店', got %q", item.Description)
	}
}

func TestAttachmentHandler_ExtractAttachmentURLs_RawFormString(t *testing.T) {
	formStr := loadInstanceDetailForm(t)

	logger, _ := zap.NewDevelopment()
	handler := NewAttachmentHandler(logger, "/tmp/test-attachments")

	refs, err := handler.ExtractAttachmentURLs(formStr)
	if err != nil {
		t.Fatalf("ExtractAttachmentURLs failed on raw form string: %v", err)
	}

	if len(refs) == 0 {
		t.Fatal("Expected at least one attachment reference, got 0")
	}

	ref := refs[0]
	t.Logf("Attachment: name=%s, url=%s", ref.OriginalName, ref.URL)

	if ref.OriginalName != "浙江通行费电子发票1.pdf" {
		t.Errorf("Expected filename '浙江通行费电子发票1.pdf', got %q", ref.OriginalName)
	}
	if ref.URL == "" {
		t.Error("Expected non-empty attachment URL")
	}
}

func TestAttachmentHandler_ExtractAttachmentURLs_DoubleEncodedDBFormat(t *testing.T) {
	formStr := loadInstanceDetailForm(t)
	dbFormData := simulateDBFormData(t, formStr)

	logger, _ := zap.NewDevelopment()
	handler := NewAttachmentHandler(logger, "/tmp/test-attachments")

	refs, err := handler.ExtractAttachmentURLs(dbFormData)
	if err != nil {
		t.Fatalf("ExtractAttachmentURLs failed on double-encoded DB form data: %v", err)
	}

	if len(refs) == 0 {
		t.Fatal("Expected at least one attachment reference, got 0")
	}

	if refs[0].URL == "" {
		t.Error("Expected non-empty attachment URL")
	}
}

func TestFormParser_ParseWithAttachments_DoubleEncodedDBFormat(t *testing.T) {
	formStr := loadInstanceDetailForm(t)
	dbFormData := simulateDBFormData(t, formStr)

	logger, _ := zap.NewDevelopment()
	parser := NewFormParser(logger)

	items, attachments, err := parser.ParseWithAttachments(dbFormData)
	if err != nil {
		t.Fatalf("ParseWithAttachments failed: %v", err)
	}

	if len(items) == 0 {
		t.Error("Expected at least one reimbursement item")
	}
	if len(attachments) == 0 {
		t.Error("Expected at least one attachment reference")
	}

	if len(items) > 0 {
		t.Logf("Item: type=%s desc=%s amount=%.2f", items[0].ItemType, items[0].Description, items[0].Amount)
	}
	if len(attachments) > 0 {
		t.Logf("Attachment: name=%s", attachments[0].OriginalName)
	}
}
