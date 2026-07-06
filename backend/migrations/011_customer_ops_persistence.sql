-- Customer operation center persistence baseline.
-- This migration extends the existing customer/tag tables for the customer
-- operation prototype and adds the operation-specific logs/tasks/timeline tables.

ALTER TABLE customers ADD COLUMN IF NOT EXISTS nickname TEXT NOT NULL DEFAULT '';
ALTER TABLE customers ADD COLUMN IF NOT EXISTS avatar TEXT NOT NULL DEFAULT '';
ALTER TABLE customers ADD COLUMN IF NOT EXISTS source_channel TEXT NOT NULL DEFAULT '';
ALTER TABLE customers ADD COLUMN IF NOT EXISTS region_id TEXT NOT NULL DEFAULT '';
ALTER TABLE customers ADD COLUMN IF NOT EXISTS region_name TEXT NOT NULL DEFAULT '';
ALTER TABLE customers ADD COLUMN IF NOT EXISTS store_id TEXT NOT NULL DEFAULT '';
ALTER TABLE customers ADD COLUMN IF NOT EXISTS store_name TEXT NOT NULL DEFAULT '';
ALTER TABLE customers ADD COLUMN IF NOT EXISTS owner_guide_id TEXT NOT NULL DEFAULT '';
ALTER TABLE customers ADD COLUMN IF NOT EXISTS owner_guide_name TEXT NOT NULL DEFAULT '';
ALTER TABLE customers ADD COLUMN IF NOT EXISTS intention_level TEXT NOT NULL DEFAULT '';
ALTER TABLE customers ADD COLUMN IF NOT EXISTS status TEXT NOT NULL DEFAULT '';
ALTER TABLE customers ADD COLUMN IF NOT EXISTS add_wecom_time TIMESTAMPTZ;
ALTER TABLE customers ADD COLUMN IF NOT EXISTS last_follow_up_time TIMESTAMPTZ;
ALTER TABLE customers ADD COLUMN IF NOT EXISTS last_interaction_time TIMESTAMPTZ;
ALTER TABLE customers ADD COLUMN IF NOT EXISTS deal_status TEXT NOT NULL DEFAULT '';
ALTER TABLE customers ADD COLUMN IF NOT EXISTS risk TEXT NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_customers_ops_scope ON customers (region_id, store_id, owner_guide_id, updated_at DESC);
CREATE INDEX IF NOT EXISTS idx_customers_ops_stage ON customers (lifecycle_stage, updated_at DESC);

ALTER TABLE tags ADD COLUMN IF NOT EXISTS category TEXT NOT NULL DEFAULT '';
ALTER TABLE tags ADD COLUMN IF NOT EXISTS color TEXT NOT NULL DEFAULT '';
ALTER TABLE tags ADD COLUMN IF NOT EXISTS source TEXT NOT NULL DEFAULT '';
ALTER TABLE tags ADD COLUMN IF NOT EXISTS is_enabled BOOLEAN NOT NULL DEFAULT true;

ALTER TABLE customer_tags ADD COLUMN IF NOT EXISTS relation_id TEXT;
ALTER TABLE customer_tags ADD COLUMN IF NOT EXISTS operator_id TEXT NOT NULL DEFAULT '';

CREATE TABLE IF NOT EXISTS customer_lifecycle_logs (
  id TEXT PRIMARY KEY,
  customer_id TEXT NOT NULL REFERENCES customers(id) ON DELETE CASCADE,
  stage_before TEXT NOT NULL DEFAULT '',
  stage_after TEXT NOT NULL DEFAULT '',
  reason TEXT NOT NULL DEFAULT '',
  operator_id TEXT NOT NULL DEFAULT '',
  operator_name TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_customer_lifecycle_logs_customer_time ON customer_lifecycle_logs (customer_id, created_at DESC);

CREATE TABLE IF NOT EXISTS follow_up_records (
  id TEXT PRIMARY KEY,
  customer_id TEXT NOT NULL REFERENCES customers(id) ON DELETE CASCADE,
  store_id TEXT NOT NULL DEFAULT '',
  guide_id TEXT NOT NULL DEFAULT '',
  follow_up_type TEXT NOT NULL DEFAULT '',
  content TEXT NOT NULL DEFAULT '',
  result TEXT NOT NULL DEFAULT '',
  next_follow_up_time TIMESTAMPTZ,
  stage_before TEXT NOT NULL DEFAULT '',
  stage_after TEXT NOT NULL DEFAULT '',
  created_by TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_follow_up_records_customer_time ON follow_up_records (customer_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_follow_up_records_store_time ON follow_up_records (store_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_follow_up_records_guide_time ON follow_up_records (guide_id, created_at DESC);

CREATE TABLE IF NOT EXISTS sop_tasks (
  id TEXT PRIMARY KEY,
  task_type TEXT NOT NULL DEFAULT '',
  title TEXT NOT NULL DEFAULT '',
  description TEXT NOT NULL DEFAULT '',
  customer_id TEXT NOT NULL DEFAULT '',
  customer_name TEXT NOT NULL DEFAULT '',
  region_id TEXT NOT NULL DEFAULT '',
  region_name TEXT NOT NULL DEFAULT '',
  store_id TEXT NOT NULL DEFAULT '',
  store_name TEXT NOT NULL DEFAULT '',
  assigned_to_user_id TEXT NOT NULL DEFAULT '',
  assigned_to_name TEXT NOT NULL DEFAULT '',
  assigned_to_role TEXT NOT NULL DEFAULT '',
  priority TEXT NOT NULL DEFAULT 'P1',
  status TEXT NOT NULL DEFAULT 'pending',
  due_time TIMESTAMPTZ,
  completed_at TIMESTAMPTZ,
  source TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_sop_tasks_scope_status ON sop_tasks (region_id, store_id, assigned_to_user_id, status, due_time);
CREATE INDEX IF NOT EXISTS idx_sop_tasks_customer ON sop_tasks (customer_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_sop_tasks_status_due ON sop_tasks (status, due_time);

CREATE TABLE IF NOT EXISTS sop_task_logs (
  id TEXT PRIMARY KEY,
  task_id TEXT NOT NULL REFERENCES sop_tasks(id) ON DELETE CASCADE,
  action TEXT NOT NULL DEFAULT '',
  old_status TEXT NOT NULL DEFAULT '',
  new_status TEXT NOT NULL DEFAULT '',
  remark TEXT NOT NULL DEFAULT '',
  operator_id TEXT NOT NULL DEFAULT '',
  operator_name TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_sop_task_logs_task_time ON sop_task_logs (task_id, created_at DESC);

CREATE TABLE IF NOT EXISTS customer_operation_timeline (
  id TEXT PRIMARY KEY,
  customer_id TEXT NOT NULL REFERENCES customers(id) ON DELETE CASCADE,
  event_type TEXT NOT NULL DEFAULT '',
  title TEXT NOT NULL DEFAULT '',
  content TEXT NOT NULL DEFAULT '',
  related_id TEXT NOT NULL DEFAULT '',
  operator_id TEXT NOT NULL DEFAULT '',
  operator_name TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_customer_operation_timeline_customer_time ON customer_operation_timeline (customer_id, created_at DESC);

CREATE TABLE IF NOT EXISTS materials (
  id TEXT PRIMARY KEY,
  title TEXT NOT NULL DEFAULT '',
  content TEXT NOT NULL DEFAULT '',
  material_type TEXT NOT NULL DEFAULT '',
  applicable_stage TEXT NOT NULL DEFAULT '',
  applicable_tags TEXT[] NOT NULL DEFAULT ARRAY[]::TEXT[],
  status TEXT NOT NULL DEFAULT 'enabled',
  created_by TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_materials_stage_status ON materials (applicable_stage, status, updated_at DESC);

CREATE TABLE IF NOT EXISTS operation_exceptions (
  id TEXT PRIMARY KEY,
  exception_type TEXT NOT NULL DEFAULT '',
  title TEXT NOT NULL DEFAULT '',
  description TEXT NOT NULL DEFAULT '',
  severity TEXT NOT NULL DEFAULT 'P1',
  region_id TEXT NOT NULL DEFAULT '',
  region_name TEXT NOT NULL DEFAULT '',
  store_id TEXT NOT NULL DEFAULT '',
  store_name TEXT NOT NULL DEFAULT '',
  customer_id TEXT NOT NULL DEFAULT '',
  customer_name TEXT NOT NULL DEFAULT '',
  task_id TEXT NOT NULL DEFAULT '',
  assigned_to TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL DEFAULT 'pending',
  suggestion TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  resolved_at TIMESTAMPTZ
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_operation_exceptions_open_unique
ON operation_exceptions (exception_type, customer_id, task_id)
WHERE status NOT IN ('resolved', 'ignored');

CREATE INDEX IF NOT EXISTS idx_operation_exceptions_scope_status ON operation_exceptions (region_id, store_id, status, created_at DESC);
