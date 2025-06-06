-- Add unique constraint to social_pages table to support ON CONFLICT clause
-- This ensures we can properly handle duplicate page_id + platform combinations

-- Add unique constraint if it doesn't already exist
ALTER TABLE social_pages 
DROP CONSTRAINT IF EXISTS social_pages_platform_page_id_key;

ALTER TABLE social_pages 
ADD CONSTRAINT social_pages_platform_page_id_key 
UNIQUE (platform, page_id);

-- Log completion
DO $$ 
BEGIN 
    RAISE NOTICE 'Added unique constraint to social_pages table successfully';
END $$; 