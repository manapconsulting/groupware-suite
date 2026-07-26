-- Migration 001: per-mailbox quota + unique email addresses
--
-- Apply with:
--   mysql mailserver < deployment/database/migrations/001_user_quota_and_unique_email.sql
--
-- The unique index fails if duplicate addresses already exist. Check first:
--   SELECT email, COUNT(*) c FROM virtual_users GROUP BY email HAVING c > 1;

-- Mailbox size limit in megabytes; 0 means unlimited.
ALTER TABLE `virtual_users`
  ADD COLUMN `quota` int(11) NOT NULL DEFAULT 0 AFTER `password`;

-- Two rows with the same address made delivery and login ambiguous, and left
-- the API's duplicate check open to a race.
ALTER TABLE `virtual_users`
  ADD UNIQUE KEY `unique_email` (`email`);
