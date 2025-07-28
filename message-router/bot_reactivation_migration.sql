-- Bot Reactivation Migration (Trigger-Based Approach)
-- This migration creates a PostgreSQL function that automatically reactivates bots
-- after 6 hours of no human agent activity using database triggers

-- PART 1: CORE FUNCTIONS (Required)
-- =====================================

-- Function that reactivates bots that should be enabled
CREATE OR REPLACE FUNCTION reactivate_idle_bots()
RETURNS TABLE(thread_id text, reactivated_count integer) AS $$
DECLARE
    reactivated_count integer := 0;
    conversation_record RECORD;
BEGIN
    -- Find conversations where bot should be reactivated
    -- Added extra safety checks to prevent premature reactivation
    FOR conversation_record IN
        SELECT c.thread_id, c.page_id, c.platform, sp.page_id as meta_page_id, sp.client_id
        FROM conversations c
        JOIN social_pages sp ON sp.id = c.page_id
        WHERE c.bot_enabled = false 
          AND c.last_human_message_at IS NOT NULL
          AND c.last_human_message_at < NOW() - INTERVAL '6 hours'
          -- Additional safety: ensure the conversation was updated more than 5 minutes ago
          -- This prevents reactivation if there's recent activity
          AND c.updated_at < NOW() - INTERVAL '5 minutes'
    LOOP
        -- Reactivate the bot
        UPDATE conversations 
        SET bot_enabled = true,
            updated_at = NOW()
        WHERE conversations.thread_id = conversation_record.thread_id;
        
        -- Log the reactivation with a system message
        IF conversation_record.client_id IS NOT NULL THEN
            INSERT INTO messages (
                id,
                client_id, 
                page_id,
                platform, 
                thread_id,
                content, 
                from_user, 
                source, 
                requires_attention,
                timestamp,
                read
            ) VALUES (
                gen_random_uuid(),
                conversation_record.client_id,
                conversation_record.page_id,
                conversation_record.platform,
                conversation_record.thread_id,
                'Bot automatically reactivated after 6 hours of no human agent activity',
                'system',
                'system',
                false,
                NOW(),
                true
            );
        ELSE
            INSERT INTO messages (
                id,
                page_id,
                platform, 
                thread_id,
                content, 
                from_user, 
                source, 
                requires_attention,
                timestamp,
                read
            ) VALUES (
                gen_random_uuid(),
                conversation_record.page_id,
                conversation_record.platform,
                conversation_record.thread_id,
                'Bot automatically reactivated after 6 hours of no human agent activity',
                'system',
                'system',
                false,
                NOW(),
                true
            );
        END IF;
        
        reactivated_count := reactivated_count + 1;
    END LOOP;
    
    -- Return summary
    RETURN QUERY SELECT 'summary'::text, reactivated_count;
END;
$$ LANGUAGE plpgsql;

-- Wrapper function for the reactivation check
CREATE OR REPLACE FUNCTION run_bot_reactivation_check()
RETURNS void AS $$
DECLARE
    result_record RECORD;
    total_reactivated integer := 0;
BEGIN
    -- Run the reactivation function
    FOR result_record IN SELECT * FROM reactivate_idle_bots() LOOP
        total_reactivated := result_record.reactivated_count;
    END LOOP;
    
    -- Log the result (optional, for monitoring)
    IF total_reactivated > 0 THEN
        RAISE NOTICE 'Bot reactivation check completed: % bots reactivated', total_reactivated;
    END IF;
END;
$$ LANGUAGE plpgsql;

-- PART 2: TRIGGER SETUP (Required for automatic execution)
-- ========================================================

-- Trigger function that runs reactivation check on new messages
-- Modified to prevent immediate reactivation after handoff
CREATE OR REPLACE FUNCTION check_bot_reactivation_on_message()
RETURNS TRIGGER AS $$
DECLARE
    last_check_time timestamp;
    min_check_interval interval := '15 minutes'; -- Minimum time between reactivation checks
BEGIN
    -- Only run reactivation check when a user message is inserted
    -- This ensures we check periodically without needing a cron job
    IF NEW.from_user != 'system' AND NEW.from_user != 'admin' THEN
        -- Check if we've run the reactivation check recently to avoid excessive processing
        SELECT created_at INTO last_check_time 
        FROM messages 
        WHERE content LIKE 'Bot reactivation check:%' 
        AND from_user = 'system'
        ORDER BY created_at DESC 
        LIMIT 1;
        
        -- Only run if it's been more than the minimum interval since last check
        IF last_check_time IS NULL OR last_check_time < NOW() - min_check_interval THEN
            -- Log that we're running a reactivation check
            INSERT INTO messages (
                id, page_id, platform, thread_id, content, from_user, source, 
                requires_attention, timestamp, read
            ) VALUES (
                gen_random_uuid(), NEW.page_id, NEW.platform, 'system', 
                'Bot reactivation check: triggered by user message', 
                'system', 'system', false, NOW(), true
            );
            
            PERFORM run_bot_reactivation_check();
        END IF;
    END IF;
    
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Create the trigger that automatically runs the reactivation check
DROP TRIGGER IF EXISTS trigger_check_bot_reactivation ON messages;
CREATE TRIGGER trigger_check_bot_reactivation
    AFTER INSERT ON messages
    FOR EACH ROW
    EXECUTE FUNCTION check_bot_reactivation_on_message();

-- PART 3: DOCUMENTATION AND TESTING
-- =================================

-- Manual execution for testing (run this to test the function)
-- SELECT run_bot_reactivation_check();

-- Check which conversations are eligible for reactivation:
-- SELECT c.thread_id, c.platform, c.last_human_message_at,
--        NOW() - c.last_human_message_at as time_since_human
-- FROM conversations c
-- WHERE c.bot_enabled = false 
--   AND c.last_human_message_at IS NOT NULL
--   AND c.last_human_message_at < NOW() - INTERVAL '6 hours';

COMMENT ON FUNCTION reactivate_idle_bots() IS 'Reactivates bots that have been idle for more than 6 hours due to human agent activity';
COMMENT ON FUNCTION run_bot_reactivation_check() IS 'Wrapper function to run bot reactivation check and log results';
COMMENT ON FUNCTION check_bot_reactivation_on_message() IS 'Trigger function that runs bot reactivation check when new messages arrive';

-- =====================================
-- SETUP COMPLETE! 
-- =====================================
-- 
-- This trigger-based approach will automatically:
-- 1. Run reactivation checks when users send messages
-- 2. Reactivate bots after 6 hours of human agent silence
-- 3. Log all reactivations as system messages
-- 
-- No pg_cron extension or additional setup required! 