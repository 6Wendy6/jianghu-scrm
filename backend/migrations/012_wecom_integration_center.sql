-- Enterprise WeChat integration center phase 2B.
-- This keeps the existing wecom_corp_config/wecom_access_tokens tables intact
-- and adds a scoped integration-center model for config, token cache, and
-- permission-check history.

CREATE TABLE IF NOT EXISTS wecom_configs (
  id TEXT PRIMARY KEY,
  corp_id TEXT NOT NULL UNIQUE,
  contact_secret_encrypted TEXT NOT NULL DEFAULT '',
  customer_secret_encrypted TEXT NOT NULL DEFAULT '',
  agent_id TEXT NOT NULL DEFAULT '',
  agent_secret_encrypted TEXT NOT NULL DEFAULT '',
  callback_token TEXT NOT NULL DEFAULT '',
  encoding_aes_key TEXT NOT NULL DEFAULT '',
  enabled BOOLEAN NOT NULL DEFAULT true,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS wecom_tokens (
  id TEXT PRIMARY KEY,
  corp_id TEXT NOT NULL,
  token_type TEXT NOT NULL,
  access_token TEXT NOT NULL DEFAULT '',
  expires_at TIMESTAMPTZ,
  last_error_code TEXT NOT NULL DEFAULT '',
  last_error_message TEXT NOT NULL DEFAULT '',
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT wecom_tokens_type_check CHECK (token_type IN ('contact_token', 'customer_token', 'app_token')),
  CONSTRAINT wecom_tokens_corp_type_unique UNIQUE (corp_id, token_type)
);

CREATE INDEX IF NOT EXISTS idx_wecom_tokens_type_expires
  ON wecom_tokens (token_type, expires_at);

CREATE TABLE IF NOT EXISTS wecom_permission_checks (
  id TEXT PRIMARY KEY,
  check_type TEXT NOT NULL,
  status TEXT NOT NULL DEFAULT 'not_checked',
  errcode INTEGER NOT NULL DEFAULT 0,
  errmsg TEXT NOT NULL DEFAULT '',
  local_code TEXT NOT NULL DEFAULT '',
  suggestion TEXT NOT NULL DEFAULT '',
  checked_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT wecom_permission_checks_status_check CHECK (status IN ('passed', 'failed', 'warning', 'not_checked', 'unsupported', 'planned'))
);

CREATE INDEX IF NOT EXISTS idx_wecom_permission_checks_type_time
  ON wecom_permission_checks (check_type, checked_at DESC);

CREATE INDEX IF NOT EXISTS idx_wecom_permission_checks_status_time
  ON wecom_permission_checks (status, checked_at DESC);
