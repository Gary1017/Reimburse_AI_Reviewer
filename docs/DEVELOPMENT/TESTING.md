# Testing Guide - AI Reimbursement System

**Document Purpose**: This document provides comprehensive testing guidelines, verification procedures, and test results for the AI Reimbursement Workflow System.

**Last Updated**: January 16, 2026

## Overview

This document covers all aspects of testing the AI Reimbursement System, including unit tests, integration tests, manual testing procedures, and verification checklists.

## Test Structure

### Unit Tests

Located in `internal/*/` directories with `*_test.go` files.

#### Form Parser Tests
**File**: `internal/lark/form_parser_test.go`

**Coverage**:
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

**Run**: `go test ./internal/lark -v -run TestFormParser`

#### Workflow Tests
**File**: `internal/workflow/engine_test.go`

**Coverage**:
- Status transitions
- Status mapping from Lark
- Invalid transition handling

**Run**: `go test ./internal/workflow -v`

#### AI Auditor Tests
**File**: `internal/ai/auditor_test.go`

**Coverage**:
- Decision determination logic
- Confidence scoring
- Policy validation integration

**Run**: `go test ./internal/ai -v`

#### Excel Filler Tests
**File**: `internal/voucher/excel_filler_test.go`

**Coverage**:
- Chinese number capitalization
- Template filling
- Amount calculations

**Run**: `go test ./internal/voucher -v`

### Integration Tests

#### Form Parser Integration
**Test Endpoint**: `POST /api/v1/test/parse-form`

**Test Cases**:

1. **Simple Nested Form**
```bash
curl -X POST http://localhost:8080/api/v1/test/parse-form \
  -H "Content-Type: application/json" \
  -d '{"form":{"amount":150.50,"description":"Conference ticket","expense_date":"2024-01-15","vendor":"TechConf Inc.","business_purpose":"Attend conference"}}'
```

2. **Multiple Items Array**
```bash
curl -X POST http://localhost:8080/api/v1/test/parse-form \
  -H "Content-Type: application/json" \
  -d '{"reimbursement_items":[{"amount":1200,"description":"Flight ticket","expense_date":"2024-01-10","vendor":"China Eastern","business_purpose":"Business trip"},{"amount":450,"description":"Hotel","expense_date":"2024-01-10","vendor":"Marriott","business_purpose":"Accommodation"}]}'
```

3. **Indexed Fields**
```bash
curl -X POST http://localhost:8080/api/v1/test/parse-form \
  -H "Content-Type: application/json" \
  -d '{"description_1":"Office supplies","amount_1":120,"vendor_1":"Office Depot","business_purpose_1":"Monthly supplies","expense_date_1":"2024-01-12","description_2":"Software license","amount_2":2000,"vendor_2":"Microsoft","business_purpose_2":"Annual license","expense_date_2":"2024-01-13"}'
```

4. **Chinese Field Names**
```bash
curl -X POST http://localhost:8080/api/v1/test/parse-form \
  -H "Content-Type: application/json" \
  -d '{"form":{"金额":300,"摘要":"出差交通费","商户":"滴滴出行","用途":"出差去客户现场","日期":"2024-01-15"}}'
```

## Manual Testing Procedures

### Test 1: Approval Creation Flow

1. **Create Approval in Lark**
   - Submit a reimbursement approval with expense items
   - Include attachment if testing Phase 3 features

2. **Verify Server Logs**
   ```bash
   tail -f /tmp/server.log | grep -E "(approval|instance|parse|item)"
   ```
   Expected logs:
   - "Created approval instance"
   - "Successfully parsed reimbursement items"
   - "Saved reimbursement items"

3. **Check Database**
   ```bash
   sqlite3 data/reimbursement.db "SELECT * FROM approval_instances ORDER BY id DESC LIMIT 1;"
   sqlite3 data/reimbursement.db "SELECT * FROM reimbursement_items WHERE instance_id = (SELECT id FROM approval_instances ORDER BY id DESC LIMIT 1);"
   ```

4. **Verify Extracted Fields**
   - Expense date populated
   - Business purpose populated
   - Item type correctly inferred
   - Amount extracted correctly

### Test 2: Status Transition Flow

1. **Monitor Status Changes**
   ```bash
   sqlite3 data/reimbursement.db "SELECT * FROM approval_history WHERE instance_id = <instance_id> ORDER BY timestamp;"
   ```

2. **Verify Status Progression**
   - CREATED → PENDING → AI_AUDITING → APPROVED → COMPLETED

### Test 3: Form Parser with Real Lark Data

1. **Get Form Data from Database**
   ```bash
   sqlite3 data/reimbursement.db "SELECT form_data FROM approval_instances WHERE id = <instance_id>;" > /tmp/form_data.json
   ```

2. **Test Parsing**
   ```bash
   curl -X POST http://localhost:8080/api/v1/test/parse-form \
     -H "Content-Type: application/json" \
     -d @/tmp/form_data.json
   ```

3. **Verify Results**
   - Check item count matches form
   - Verify all fields extracted correctly
   - Confirm item types are correct

## Verification Results

### Demo Approval #1 (Instance ID: 5)
- **Status**: APPROVED
- **Items Extracted**: 1
- **Fields Verified**:
  - ✅ Item Type: ACCOMMODATION
  - ✅ Description: "yuyu"
  - ✅ Amount: 236.05 CNY
  - ✅ Expense Date: 2026-01-13
  - ✅ Business Purpose: "yyy02"
  - ⚠️ Vendor: Not in form (optional)

### Demo Approval #2 (Instance ID: 6)
- **Status**: APPROVED
- **Items Extracted**: 1
- **Fields Verified**:
  - ✅ Item Type: ACCOMMODATION
  - ✅ Description: "yuyu"
  - ✅ Amount: 236.05 CNY
  - ✅ Expense Date: 2026-01-13
  - ✅ Business Purpose: "yyy03"
  - ⚠️ Vendor: Not in form (optional)

## Test Utilities

### Form Parser Test Script
**File**: `test_form_parsing.sh`

Runs multiple test cases against the form parser endpoint.

**Usage**:
```bash
./test_form_parsing.sh
```

### Reprocess Instance Utility
**File**: `reprocess_instance.go`

Reprocesses an existing approval instance to extract items.

**Usage**:
```bash
go run reprocess_instance.go <instance_id>
```

## Running All Tests

```bash
# Run all unit tests
go test ./... -v

# Run specific package tests
go test ./internal/lark -v
go test ./internal/workflow -v
go test ./internal/ai -v
go test ./internal/voucher -v

# Run with coverage
go test ./... -cover
```

## Test Coverage

Current test coverage:
- Form Parser: 10/10 tests passing
- Workflow Engine: 4/4 tests passing
- AI Auditor: 4/4 tests passing
- Excel Filler: 4/4 tests passing
- Event Processor: 4/4 tests passing

**Total**: 26+ tests, all passing ✅

## Continuous Testing

### Pre-Commit Checks
```bash
# Run tests before committing
go test ./... -v

# Check code formatting
go fmt ./...

# Run linter
golangci-lint run
```

### CI/CD Testing
Tests run automatically on:
- Pull request creation
- Push to main branch
- Manual workflow trigger

## Troubleshooting

### Tests Failing
1. Check database state: `sqlite3 data/reimbursement.db ".tables"`
2. Verify migrations: Check `migrations/` directory
3. Check logs: `tail -f /tmp/server.log`
4. Verify configuration: `cat configs/config.yaml`

### Integration Test Failures
1. Ensure server is running: `curl http://localhost:8080/health`
2. Check database connection
3. Verify Lark credentials are set
4. Check network connectivity

## Best Practices

1. **Write Tests First**: Use TDD approach for new features
2. **Test Edge Cases**: Include boundary conditions
3. **Mock External Dependencies**: Use interfaces for testability
4. **Keep Tests Fast**: Unit tests should run quickly
5. **Document Test Cases**: Explain what each test verifies
6. **Maintain Test Coverage**: Aim for >80% coverage

## Future Test Improvements

- [ ] Add end-to-end tests
- [ ] Implement test fixtures for consistent data
- [ ] Add performance benchmarks
- [ ] Create test data generators
- [ ] Add contract testing for Lark API
- [ ] Implement chaos engineering tests

---

**Status**: ✅ Comprehensive Testing Framework in Place
