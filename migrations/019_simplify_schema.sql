-- Migration: simplify_schema
-- Description: Remove redundant invoice_lists and review_notifications tables
--
-- Rationale:
-- 1. invoice_lists: Aggregations can be computed from invoices_v2 directly
--    since invoices_v2 already has instance_id for efficient queries
-- 2. review_notifications: Notifications are runtime-generated, not safety-critical
--    Delivery tracking moved to approval_tasks.notification_sent_at

-- ============================================================================
-- Add notification_sent_at to approval_tasks for delivery tracking
-- ============================================================================

ALTER TABLE approval_tasks ADD COLUMN notification_sent_at DATETIME;

-- ============================================================================
-- Deprecate invoice_lists table (rename to preserve data)
-- ============================================================================

ALTER TABLE invoice_lists RENAME TO invoice_lists_deprecated;

-- ============================================================================
-- Deprecate review_notifications table (rename to preserve data)
-- ============================================================================

ALTER TABLE review_notifications RENAME TO review_notifications_deprecated_v2;

-- ============================================================================
-- Note: invoices_v2.invoice_list_id column remains but is now unused
-- SQLite doesn't support DROP COLUMN in older versions
-- New code should use instance_id directly instead
-- ============================================================================
