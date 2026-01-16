# Phase 3: Attachment Handling Implementation Plan

**Document Purpose**: This document outlines the implementation plan for attachment extraction and handling logic for Lark approval instances.

**Status**: 📋 Planned  
**Priority**: High

## Objective

Implement attachment extraction and handling logic to download, store, and link receipt attachments from Lark approval instances to reimbursement items.

## Current State

- ✅ Form parser extracts reimbursement items successfully
- ✅ Items saved to database with required fields
- ⚠️ Attachment URLs present in form data but not extracted
- ⚠️ Attachments not downloaded or stored

## Requirements

### 1. Attachment Extraction
- Extract attachment URLs from Lark form data
- Identify attachment widget (`attachmentV2` type)
- Extract file tokens/URLs from widget value array
- Link attachments to reimbursement items

### 2. Attachment Download
- Download files from Lark Drive API
- Handle different file types (PDF, images, etc.)
- Store files in configured attachment directory
- Generate unique file names

### 3. Database Integration
- Store attachment metadata in database
- Link attachments to reimbursement items
- Track download status
- Store file paths

### 4. Error Handling
- Handle download failures gracefully
- Retry logic for transient failures
- Log attachment processing status

## Current Attachment Data Structure

From form data analysis:
```json
{
  "id": "widget16510510447300001",
  "name": "附件",
  "type": "attachmentV2",
  "ext": "发票海油.pdf",
  "value": [
    "https://internal-api-drive-stream.feishu.cn/space/api/box/stream/download/authcode/?code=..."
  ]
}
```

## Implementation Plan

### Phase 3.1: Extract Attachment URLs

**Files to Modify**:
- `internal/lark/form_parser.go` - Extract attachment URLs from widgets
- `internal/models/instance.go` - Add attachment fields if needed

**Tasks**:
1. Identify `attachmentV2` widget in form data
2. Extract file URLs/tokens from widget value
3. Store attachment references in reimbursement items

### Phase 3.2: Download Attachments

**Files to Create/Modify**:
- `internal/lark/attachment_handler.go` - Download logic
- `internal/voucher/attachment_handler.go` - May need updates

**Tasks**:
1. Implement Lark Drive API client for file downloads
2. Download files using authentication tokens
3. Save files to `attachments/` directory
4. Generate unique file names (e.g., `{instance_id}_{item_id}_{filename}`)

### Phase 3.3: Database Schema

**Files to Create**:
- `migrations/004_add_attachments.sql` - If needed

**Considerations**:
- May need `attachments` table or add fields to `reimbursement_items`
- Store: file_path, file_name, file_size, download_status, downloaded_at

### Phase 3.4: Workflow Integration

**Files to Modify**:
- `internal/workflow/engine.go` - Trigger attachment download
- `internal/voucher/generator.go` - Include attachments in voucher generation

**Tasks**:
1. Download attachments after items are parsed
2. Update item records with attachment file paths
3. Include attachments when generating vouchers

## Key Considerations

1. **Authentication**: Use Lark client credentials for API access
2. **File Storage**: Store in `configs/config.yaml` → `voucher.attachment_dir`
3. **File Naming**: Ensure unique names to avoid conflicts
4. **Error Handling**: Don't fail approval processing if download fails
5. **Performance**: Consider async download for large files
6. **Security**: Validate file types, scan for malware (future)

## Testing Strategy

1. Test with approval containing PDF attachment
2. Verify file download and storage
3. Verify file linking to reimbursement items
4. Test error handling (invalid URL, network failure)
5. Test with multiple attachments

## Success Criteria

- ✅ Attachment URLs extracted from form data
- ✅ Files downloaded successfully from Lark
- ✅ Files stored in correct directory
- ✅ Database records updated with file paths
- ✅ Attachments linked to reimbursement items
- ✅ Error handling works correctly
- ✅ No impact on approval processing if download fails

## Dependencies

- Lark Drive API access
- File system write permissions
- Sufficient disk space for attachments
- Network connectivity to Lark servers

## Estimated Effort

- Phase 3.1: 2-3 hours
- Phase 3.2: 4-6 hours
- Phase 3.3: 1-2 hours
- Phase 3.4: 2-3 hours
- Testing: 2-3 hours

**Total**: ~12-17 hours

---

**Status**: 📋 Ready to Start Implementation
