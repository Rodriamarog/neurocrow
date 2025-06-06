-- Migration to add Dify conversation tracking
-- Adds dify_conversation_id column to conversations table

ALTER TABLE conversations 
ADD COLUMN dify_conversation_id TEXT;

-- Add index for faster lookups
CREATE INDEX idx_conversations_dify_conversation_id 
ON conversations(dify_conversation_id);

-- Add comment explaining the column
COMMENT ON COLUMN conversations.dify_conversation_id 
IS 'Stores the conversation_id returned by Dify API to maintain conversation context'; 