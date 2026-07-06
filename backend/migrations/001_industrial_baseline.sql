-- AI SCRM industrial baseline schema.
-- Target database: PostgreSQL 14+.
-- This migration captures the core high-concurrency data model before the
-- current in-memory demo API is moved onto persistent storage.

CREATE TABLE IF NOT EXISTS customers (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  wecom_name TEXT NOT NULL DEFAULT '',
  mobile_masked TEXT NOT NULL DEFAULT '',
  lifecycle_stage TEXT NOT NULL DEFAULT 'new',
  sales_stage TEXT NOT NULL DEFAULT '',
  owner_staff_id TEXT NOT NULL DEFAULT '',
  owner_staff_name TEXT NOT NULL DEFAULT '',
  source_store_id TEXT NOT NULL DEFAULT '',
  source_store_name TEXT NOT NULL DEFAULT '',
  intent_score INTEGER NOT NULL DEFAULT 0,
  deal_amount_cents BIGINT NOT NULL DEFAULT 0,
  next_follow_up_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_customers_owner_stage ON customers (owner_staff_id, lifecycle_stage);
CREATE INDEX IF NOT EXISTS idx_customers_source_store_created ON customers (source_store_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_customers_updated ON customers (updated_at DESC);

CREATE TABLE IF NOT EXISTS customer_identities (
  id TEXT PRIMARY KEY,
  customer_id TEXT NOT NULL REFERENCES customers(id) ON DELETE CASCADE,
  identity_type TEXT NOT NULL,
  identity_value TEXT NOT NULL,
  is_primary BOOLEAN NOT NULL DEFAULT false,
  merged_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (identity_type, identity_value)
);

CREATE INDEX IF NOT EXISTS idx_customer_identities_customer ON customer_identities (customer_id);

CREATE TABLE IF NOT EXISTS customer_attributions (
  customer_id TEXT PRIMARY KEY REFERENCES customers(id) ON DELETE CASCADE,
  source_store_id TEXT NOT NULL,
  source_brand_id TEXT NOT NULL DEFAULT '',
  first_staff_id TEXT NOT NULL DEFAULT '',
  source_code_id TEXT NOT NULL DEFAULT '',
  entry_mode TEXT NOT NULL DEFAULT '',
  entry_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  current_service_store_id TEXT NOT NULL DEFAULT '',
  main_follow_staff_id TEXT NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS idx_customer_attributions_source_store ON customer_attributions (source_store_id, entry_at DESC);
CREATE INDEX IF NOT EXISTS idx_customer_attributions_first_staff ON customer_attributions (first_staff_id, entry_at DESC);
CREATE INDEX IF NOT EXISTS idx_customer_attributions_source_code ON customer_attributions (source_code_id, entry_at DESC);

CREATE TABLE IF NOT EXISTS customer_staff_relations (
  id TEXT PRIMARY KEY,
  customer_id TEXT NOT NULL REFERENCES customers(id) ON DELETE CASCADE,
  staff_id TEXT NOT NULL,
  staff_name TEXT NOT NULL DEFAULT '',
  via_code_id TEXT NOT NULL DEFAULT '',
  store_id TEXT NOT NULL DEFAULT '',
  linked_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  ended_at TIMESTAMPTZ,
  is_main_follow BOOLEAN NOT NULL DEFAULT false,
  status TEXT NOT NULL DEFAULT 'active'
);

CREATE INDEX IF NOT EXISTS idx_customer_staff_relations_customer ON customer_staff_relations (customer_id, status);
CREATE INDEX IF NOT EXISTS idx_customer_staff_relations_staff ON customer_staff_relations (staff_id, linked_at DESC);

CREATE TABLE IF NOT EXISTS store_codes (
  id TEXT PRIMARY KEY,
  code_type TEXT NOT NULL,
  store_id TEXT NOT NULL,
  staff_id TEXT NOT NULL DEFAULT '',
  entry_mode TEXT NOT NULL DEFAULT '',
  welcome_rule_id TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL DEFAULT 'active',
  qr_payload TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  disabled_at TIMESTAMPTZ,
  destroyed_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_store_codes_store_status ON store_codes (store_id, status);
CREATE INDEX IF NOT EXISTS idx_store_codes_staff_status ON store_codes (staff_id, status);

CREATE TABLE IF NOT EXISTS tag_groups (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL UNIQUE,
  scope TEXT NOT NULL DEFAULT '',
  display_order INTEGER NOT NULL DEFAULT 0,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS tags (
  id TEXT PRIMARY KEY,
  tag_group_id TEXT NOT NULL REFERENCES tag_groups(id),
  name TEXT NOT NULL,
  status TEXT NOT NULL DEFAULT 'active',
  wecom_tag_id TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (tag_group_id, name)
);

CREATE INDEX IF NOT EXISTS idx_tags_group_status ON tags (tag_group_id, status);

CREATE TABLE IF NOT EXISTS customer_tags (
  customer_id TEXT NOT NULL REFERENCES customers(id) ON DELETE CASCADE,
  tag_id TEXT NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
  source TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (customer_id, tag_id)
);

CREATE INDEX IF NOT EXISTS idx_customer_tags_tag_customer ON customer_tags (tag_id, customer_id);

CREATE TABLE IF NOT EXISTS customer_events (
  id TEXT PRIMARY KEY,
  customer_id TEXT NOT NULL DEFAULT '',
  event_type TEXT NOT NULL,
  source TEXT NOT NULL DEFAULT '',
  object_id TEXT NOT NULL DEFAULT '',
  payload JSONB NOT NULL DEFAULT '{}'::jsonb,
  occurred_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_customer_events_customer_time ON customer_events (customer_id, occurred_at DESC);
CREATE INDEX IF NOT EXISTS idx_customer_events_type_time ON customer_events (event_type, occurred_at DESC);

CREATE TABLE IF NOT EXISTS event_inbox (
  id TEXT PRIMARY KEY,
  source TEXT NOT NULL,
  event_type TEXT NOT NULL,
  object_id TEXT NOT NULL DEFAULT '',
  idempotency_key TEXT NOT NULL UNIQUE,
  status TEXT NOT NULL DEFAULT 'queued',
  attempts INTEGER NOT NULL DEFAULT 0,
  payload JSONB NOT NULL DEFAULT '{}'::jsonb,
  received_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  processed_at TIMESTAMPTZ,
  last_error TEXT NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS idx_event_inbox_status_received ON event_inbox (status, received_at);
CREATE INDEX IF NOT EXISTS idx_event_inbox_source_type ON event_inbox (source, event_type, received_at DESC);

CREATE TABLE IF NOT EXISTS tasks (
  id TEXT PRIMARY KEY,
  task_type TEXT NOT NULL,
  status TEXT NOT NULL DEFAULT 'queued',
  total_count INTEGER NOT NULL DEFAULT 0,
  success_count INTEGER NOT NULL DEFAULT 0,
  failed_count INTEGER NOT NULL DEFAULT 0,
  payload JSONB NOT NULL DEFAULT '{}'::jsonb,
  created_by TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  started_at TIMESTAMPTZ,
  finished_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_tasks_status_created ON tasks (status, created_at DESC);

CREATE TABLE IF NOT EXISTS task_items (
  id TEXT PRIMARY KEY,
  task_id TEXT NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
  target_id TEXT NOT NULL,
  status TEXT NOT NULL DEFAULT 'queued',
  attempts INTEGER NOT NULL DEFAULT 0,
  result JSONB NOT NULL DEFAULT '{}'::jsonb,
  last_error TEXT NOT NULL DEFAULT '',
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_task_items_task_status ON task_items (task_id, status);

CREATE TABLE IF NOT EXISTS metric_snapshots (
  id TEXT PRIMARY KEY,
  metric_date DATE NOT NULL,
  scope_type TEXT NOT NULL,
  scope_id TEXT NOT NULL,
  metric_key TEXT NOT NULL,
  metric_value NUMERIC NOT NULL DEFAULT 0,
  extra JSONB NOT NULL DEFAULT '{}'::jsonb,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (metric_date, scope_type, scope_id, metric_key)
);

CREATE INDEX IF NOT EXISTS idx_metric_snapshots_scope_date ON metric_snapshots (scope_type, scope_id, metric_date DESC);
