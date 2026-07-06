-- Store manager code wrapping plus guide round-robin assignment.
-- This module binds existing Enterprise WeChat contact-way config_id values.
-- It must never create a new Enterprise WeChat QR code.

CREATE TABLE IF NOT EXISTS scrm_stores (
  id TEXT PRIMARY KEY,
  store_code TEXT NOT NULL UNIQUE,
  store_name TEXT NOT NULL,
  manager_userid TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL DEFAULT 'active',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_scrm_stores_status
  ON scrm_stores (status, updated_at DESC);

CREATE TABLE IF NOT EXISTS scrm_store_guides (
  id TEXT PRIMARY KEY,
  store_id TEXT NOT NULL REFERENCES scrm_stores(id) ON DELETE CASCADE,
  guide_userid TEXT NOT NULL,
  guide_name TEXT NOT NULL DEFAULT '',
  sort_order INTEGER NOT NULL DEFAULT 0,
  status TEXT NOT NULL DEFAULT 'active',
  daily_limit INTEGER NOT NULL DEFAULT 0,
  assigned_today INTEGER NOT NULL DEFAULT 0,
  last_assigned_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (store_id, guide_userid)
);

CREATE INDEX IF NOT EXISTS idx_scrm_store_guides_pool
  ON scrm_store_guides (store_id, status, sort_order, id);

CREATE TABLE IF NOT EXISTS scrm_wecom_contact_way_bindings (
  id TEXT PRIMARY KEY,
  store_id TEXT NOT NULL REFERENCES scrm_stores(id) ON DELETE CASCADE,
  manager_userid TEXT NOT NULL DEFAULT '',
  config_id TEXT NOT NULL UNIQUE,
  qr_code_url TEXT NOT NULL DEFAULT '',
  current_guide_userid TEXT NOT NULL DEFAULT '',
  next_index INTEGER NOT NULL DEFAULT 0,
  state_prefix TEXT NOT NULL,
  remark TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL DEFAULT 'active',
  last_switched_at TIMESTAMPTZ,
  last_error TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_scrm_contact_way_bindings_store
  ON scrm_wecom_contact_way_bindings (store_id, status, updated_at DESC);

CREATE INDEX IF NOT EXISTS idx_scrm_contact_way_bindings_state_prefix
  ON scrm_wecom_contact_way_bindings (state_prefix);

CREATE TABLE IF NOT EXISTS scrm_customer_assignments (
  id TEXT PRIMARY KEY,
  store_id TEXT NOT NULL REFERENCES scrm_stores(id),
  binding_id TEXT NOT NULL REFERENCES scrm_wecom_contact_way_bindings(id),
  config_id TEXT NOT NULL,
  manager_userid TEXT NOT NULL DEFAULT '',
  guide_userid TEXT NOT NULL,
  external_userid TEXT NOT NULL DEFAULT '',
  state TEXT NOT NULL DEFAULT '',
  event_type TEXT NOT NULL DEFAULT '',
  add_time TIMESTAMPTZ,
  status TEXT NOT NULL DEFAULT 'added',
  raw_event JSONB NOT NULL DEFAULT '{}'::jsonb,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_scrm_customer_assignments_binding
  ON scrm_customer_assignments (binding_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_scrm_customer_assignments_external
  ON scrm_customer_assignments (external_userid, created_at DESC);

CREATE TABLE IF NOT EXISTS scrm_wecom_callback_events (
  id TEXT PRIMARY KEY,
  event_key TEXT NOT NULL UNIQUE,
  event_type TEXT NOT NULL,
  external_userid TEXT NOT NULL DEFAULT '',
  user_id TEXT NOT NULL DEFAULT '',
  state TEXT NOT NULL DEFAULT '',
  processed BOOLEAN NOT NULL DEFAULT false,
  raw_event JSONB NOT NULL DEFAULT '{}'::jsonb,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_scrm_wecom_callback_events_created
  ON scrm_wecom_callback_events (created_at DESC);
