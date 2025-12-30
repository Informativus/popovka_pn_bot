-- Add subscription_url column to subscriptions table
ALTER TABLE subscriptions ADD COLUMN subscription_url VARCHAR(512);

-- Update existing records (optional - if you have existing subscriptions)
-- You may need to manually update these or they will get the URL on next payment
UPDATE subscriptions SET subscription_url = '' WHERE subscription_url IS NULL;
