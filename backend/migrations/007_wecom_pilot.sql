-- Enterprise WeChat department-level pilot tables.

CREATE TABLE IF NOT EXISTS wecom_corp_config (
  id TEXT PRIMARY KEY,
  corp_id TEXT NOT NULL UNIQUE,
  agent_id TEXT NOT NULL DEFAULT '',
  secret_encrypted TEXT NOT NULL DEFAULT '',
  token TEXT NOT NULL DEFAULT '',
  encoding_aes_key TEXT NOT NULL DEFAULT '',
  callback_url TEXT NOT NULL DEFAULT '',
  test_department_id TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL DEFAULT 'configured',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS wecom_access_tokens (
  corp_id TEXT PRIMARY KEY,
  access_token TEXT NOT NULL,
  expires_at TIMESTAMPTZ NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS wecom_users (
  id TEXT PRIMARY KEY,
  corp_id TEXT NOT NULL,
  userid TEXT NOT NULL,
  name TEXT NOT NULL DEFAULT '',
  department_id TEXT NOT NULL DEFAULT '',
  department_name TEXT NOT NULL DEFAULT '',
  mobile TEXT NOT NULL DEFAULT '',
  email TEXT NOT NULL DEFAULT '',
  avatar TEXT NOT NULL DEFAULT '',
  status INTEGER NOT NULL DEFAULT 0,
  role_type TEXT NOT NULL DEFAULT 'unknown',
  synced_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT wecom_users_role_type_check CHECK (role_type IN ('sales', 'guide', 'admin', 'unknown')),
  CONSTRAINT wecom_users_corp_user_unique UNIQUE (corp_id, userid)
);

CREATE INDEX IF NOT EXISTS idx_wecom_users_department
  ON wecom_users (corp_id, department_id, role_type);

CREATE TABLE IF NOT EXISTS wecom_contact_ways (
  id TEXT PRIMARY KEY,
  corp_id TEXT NOT NULL,
  config_id TEXT NOT NULL UNIQUE,
  name TEXT NOT NULL,
  qr_code_url TEXT NOT NULL DEFAULT '',
  scene TEXT NOT NULL DEFAULT 'dept_test',
  state TEXT NOT NULL DEFAULT '',
  bound_userids JSONB NOT NULL DEFAULT '[]'::jsonb,
  department_id TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL DEFAULT 'active',
  created_by TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_wecom_contact_ways_state
  ON wecom_contact_ways (corp_id, state);

CREATE TABLE IF NOT EXISTS wecom_customer_events (
  id TEXT PRIMARY KEY,
  corp_id TEXT NOT NULL,
  external_userid TEXT NOT NULL DEFAULT '',
  follow_userid TEXT NOT NULL DEFAULT '',
  state TEXT NOT NULL DEFAULT '',
  contact_way_id TEXT NOT NULL DEFAULT '',
  event_type TEXT NOT NULL DEFAULT '',
  add_time TIMESTAMPTZ,
  raw_payload JSONB NOT NULL DEFAULT '{}'::jsonb,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_wecom_customer_events_contact_way
  ON wecom_customer_events (corp_id, contact_way_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_wecom_customer_events_follow_user
  ON wecom_customer_events (corp_id, follow_userid, created_at DESC);
