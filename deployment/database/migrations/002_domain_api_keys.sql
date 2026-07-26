-- Migration 002: per-domain API keys with source (IP/CIDR/FQDN) restriction
--
-- Apply with:
--   mysql mailserver < deployment/database/migrations/002_domain_api_keys.sql
--
-- Each key belongs to exactly one domain and may only manage that domain's
-- mailboxes. `allowed_sources` is a comma-separated allowlist of literal IPs,
-- CIDR blocks, or FQDNs (resolved at request time); a request is accepted only
-- when its client IP matches one entry. Only the SHA-256 hash of the key is
-- stored -- the plaintext is shown once at creation and never persisted.

CREATE TABLE IF NOT EXISTS `domain_api_keys` (
  `id` int(11) NOT NULL AUTO_INCREMENT,
  `domain_id` int(11) NOT NULL,
  `name` varchar(100) NOT NULL,
  `key_prefix` varchar(16) NOT NULL,
  `key_hash` char(64) NOT NULL,
  `allowed_sources` text NOT NULL,
  `enabled` tinyint(1) NOT NULL DEFAULT 1,
  `last_used_at` datetime DEFAULT NULL,
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `unique_key_hash` (`key_hash`),
  KEY `domain_id` (`domain_id`),
  CONSTRAINT `domain_api_keys_ibfk_1` FOREIGN KEY (`domain_id`) REFERENCES `virtual_domains` (`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci;
