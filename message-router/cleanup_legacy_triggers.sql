-- Cleanup Legacy Bot Reactivation System
-- This migration removes the database triggers and functions that were used
-- for the 6-hour bot reactivation system, now replaced by Facebook Handover Protocol

-- PART 1: REMOVE TRIGGERS
-- ========================

-- Remove the trigger that was automatically checking for bot reactivation on message insert
DROP TRIGGER IF EXISTS trigger_check_bot_reactivation ON messages;

-- PART 2: REMOVE FUNCTIONS
-- =========================

-- Remove the function that was called by the trigger
DROP FUNCTION IF EXISTS check_bot_reactivation_on_message();

-- Remove the main bot reactivation check function (called by background worker)
DROP FUNCTION IF EXISTS run_bot_reactivation_check();

-- Remove the core reactivation function
DROP FUNCTION IF EXISTS reactivate_idle_bots();

-- PART 3: VERIFICATION
-- ====================

-- Verify that all functions have been removed
SELECT 
    routine_name, 
    routine_type
FROM information_schema.routines 
WHERE routine_schema = 'public' 
    AND routine_name LIKE '%bot%reactivation%'
    OR routine_name IN ('reactivate_idle_bots', 'run_bot_reactivation_check', 'check_bot_reactivation_on_message');

-- If the above query returns no rows, cleanup was successful

-- PART 4: COMMENTS ON PRESERVED COLUMNS
-- =====================================

-- Add comments to legacy columns indicating they're deprecated but preserved for rollback
COMMENT ON COLUMN conversations.bot_enabled IS 'DEPRECATED: Used only for fallback. Thread control now managed by Facebook Handover Protocol via thread_control_status column.';
COMMENT ON COLUMN conversations.last_human_message_at IS 'DEPRECATED: Kept for analytics. Thread control transitions tracked via handover_timestamp column.';
COMMENT ON COLUMN conversations.bot_disabled_at IS 'DEPRECATED: Kept for analytics. Thread control transitions tracked via handover_timestamp column.';

-- PART 5: SUMMARY
-- ===============

-- This migration removes:
-- 1. Automatic trigger-based bot reactivation system
-- 2. Background worker database functions  
-- 3. 6-hour timer logic implemented in database
--
-- Thread control is now managed by:
-- 1. Facebook Handover Protocol API calls
-- 2. thread_control_status column
-- 3. handover_timestamp and handover_reason columns
--
-- Benefits:
-- - Eliminates race conditions in bot state management
-- - Removes complex database trigger logic
-- - Reduces database load (no more periodic reactivation checks)
-- - Native Facebook integration is more reliable
-- - Simplified codebase with fewer moving parts

-- Migration completed successfully
SELECT 'Legacy bot reactivation system cleanup completed' AS status;