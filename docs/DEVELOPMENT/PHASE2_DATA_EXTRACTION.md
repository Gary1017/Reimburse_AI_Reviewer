# Phase 2: Reimbursement Data Extraction Implementation

**Document Purpose**: This document details the implementation of structured data extraction from Lark approval instances, including form parsing, database schema changes, and workflow integration.

**Implementation Date**: January 16, 2026  
**Status**: ✅ Complete

## Overview

Phase 2 successfully implemented a structured data extraction layer that parses Lark approval instance JSON responses into strongly-typed reimbursement items with required fields for policy validation and AI auditing.

## Problem Statement

The system was storing complete Lark approval responses as JSON blobs without extracting critical fields required by `configs/policies.json`:
- **Expense Date** - Required for time-based policy rules
- **Vendor/Merchant Name** - Optional, not required in accounting system
- **Business Purpose** - Required for business justification policies

This prevented effective AI-driven policy validation and forced manual review of all reimbursements.

## Solution Architecture

### Data Flow
```
Lark API Response (JSON)
    ↓
FormParser (new extraction layer)
    ├── Extract form fields (nested or indexed)
    ├── Parse item data
    ├── Type inference
    └── Populate required fields
    ↓
ReimbursementItem records (database)
    ↓
Used by Policy Validator & Price Benchmarker
```

## Implementation Details

### 1. Database Schema Enhancement

**File**: `migrations/003_add_expense_details.sql`

- Added `expense_date DATE` column
- Added `vendor TEXT` column (optional)
- Added `business_purpose TEXT` column
- Created indices for query performance

### 2. Form Parser Implementation

**File**: `internal/lark/form_parser.go` (500+ lines)

#### Core Features
- Parses Lark widget-based form structure
- Handles `form` field as JSON string containing widgets array
- Extracts items from `fieldList` widget (费用明细)
- Supports multiple form formats (nested, array-based, indexed fields)
- Supports Chinese field names (金额, 摘要, 商户, 用途, 日期)
- Intelligent item type inference
- Date parsing with timezone support (RFC3339)

#### Lark Widget Structure Support
- Parses widget array from JSON string
- Extracts reimbursement type from `radioV2` widget (报销类型)
- Extracts business purpose from `textarea` widget (报销事由)
- Extracts expense items from `fieldList` widget (费用明细)
- Maps widget fields: 内容→description, 日期→expense_date, 金额→amount

#### Field Mapping
- "内容" / "内容描述" → description
- "日期（年-月-日）" / "日期" → expense_date
- "金额" → amount
- "报销类型" → item_type (mapped to ACCOMMODATION/TRAVEL/MEAL/EQUIPMENT)
- "报销事由" → business_purpose
- "商户" → vendor (optional)

### 3. Repository Layer

**File**: `internal/repository/reimbursement_item_repo.go`

- CRUD operations for reimbursement items
- Transaction support for ACID compliance
- Methods: Create, GetByID, GetByInstanceID, Update, DeleteByInstanceID

### 4. Workflow Integration

**File**: `internal/workflow/engine.go`

- Integrated FormParser into Engine
- Updated `HandleInstanceCreated()` to:
  - Parse form data using FormParser
  - Save items to database within transaction
  - Log parsing results
  - Graceful error handling (continues if parsing fails)

### 5. Model Updates

**File**: `internal/models/instance.go`

Updated `ReimbursementItem` struct:
```go
type ReimbursementItem struct {
    // ... existing fields ...
    ExpenseDate     *time.Time `json:"expense_date,omitempty"`
    Vendor          string     `json:"vendor,omitempty"`
    BusinessPurpose string     `json:"business_purpose,omitempty"`
}
```

## Testing & Verification

### Unit Tests
**File**: `internal/lark/form_parser_test.go`

10 comprehensive test cases covering:
- ✅ Basic extraction from nested form
- ✅ Multiple items array parsing
- ✅ Indexed field inference
- ✅ Item type inference (Travel, Accommodation, Meal, Equipment)
- ✅ Date parsing in multiple formats
- ✅ Missing required fields validation
- ✅ Numeric amount parsing
- ✅ Empty data handling
- ✅ Currency default (CNY)
- ✅ Chinese field name support

**All tests pass** (10/10 ✅)

### Integration Testing

**Demo Approval #1 (Instance ID: 5)**
- Status: APPROVED
- Items Extracted: 1
- Fields Verified:
  - ✅ Item Type: ACCOMMODATION (from "住宿费")
  - ✅ Description: "yuyu"
  - ✅ Amount: 236.05 CNY
  - ✅ Expense Date: 2026-01-13
  - ✅ Business Purpose: "yyy02"
  - ⚠️ Vendor: Not in form (optional field)

**Demo Approval #2 (Instance ID: 6)**
- Status: APPROVED
- Items Extracted: 1
- Fields Verified:
  - ✅ Item Type: ACCOMMODATION (from "住宿费")
  - ✅ Description: "yuyu"
  - ✅ Amount: 236.05 CNY
  - ✅ Expense Date: 2026-01-13
  - ✅ Business Purpose: "yyy03"
  - ⚠️ Vendor: Not in form (optional field)

## Files Created/Modified

### Created
- `migrations/003_add_expense_details.sql`
- `internal/lark/form_parser.go`
- `internal/lark/form_parser_test.go`
- `internal/repository/reimbursement_item_repo.go`
- `test_form_parsing.sh` (test utility)
- `reprocess_instance.go` (utility script)

### Modified
- `internal/models/instance.go`
- `internal/workflow/engine.go`
- `cmd/server/main.go`

## Key Achievements

1. **Lark Widget Structure Support**: Successfully parsed Lark's widget-based form structure
2. **Automatic Processing**: System now automatically extracts and saves items when approvals are created
3. **Required Fields**: All required fields (date, purpose) are extracted correctly
4. **Type Mapping**: Chinese reimbursement types correctly mapped to internal types
5. **Production Ready**: Code tested, verified, and ready for production use

## Graceful Degradation

If form parsing fails:
1. Instance is still created (core flow continues)
2. Warning is logged with failure details
3. Reimbursement items table remains empty
4. System can still process (manual review can still extract items)
5. No data loss

## Performance Considerations

- Parser runs synchronously during webhook processing (typical form: <10ms)
- Indexed field detection optimized with early returns
- Database indices on `expense_date` and `vendor` for quick queries

## Migration Path

When deploying:
1. Deploy code changes (backward compatible)
2. Run migration `003_add_expense_details.sql`
3. FormParser will start extracting data on next incoming approval
4. Existing approvals in DB unaffected (columns are nullable)

## Verification Checklist

- [x] Database migration created
- [x] Model updated with new fields
- [x] FormParser implemented with multiple format support
- [x] ReimbursementItemRepository created
- [x] WorkflowEngine integration complete
- [x] Main app initialization updated
- [x] Comprehensive unit tests written
- [x] All tests pass (10/10)
- [x] Code compiles without errors
- [x] No breaking changes to existing code
- [x] Graceful error handling implemented
- [x] Documentation complete

## Summary

**Phase 2 is complete.** The system now successfully:
1. ✅ Extracts structured data from Lark approval JSON
2. ✅ Populates required fields (Date, Purpose)
3. ✅ Stores items in reimbursement_items table
4. ✅ Makes data available for policy validation
5. ✅ Handles multiple form structures gracefully
6. ✅ Supports both English and Chinese field names
7. ✅ Maintains ACID compliance
8. ✅ Includes comprehensive test coverage

**Next Steps**: Phase 3 - Attachment Handling (see [PHASE3_ATTACHMENTS.md](PHASE3_ATTACHMENTS.md))

---

**Status**: ✅ Complete - Ready for Phase 3
