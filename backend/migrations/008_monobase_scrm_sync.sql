-- Monobase SCRM data source bridge.

CREATE TABLE IF NOT EXISTS monobase_sync_state (
  resource TEXT PRIMARY KEY,
  last_success_at TIMESTAMPTZ,
  last_started_at TIMESTAMPTZ,
  last_error TEXT NOT NULL DEFAULT '',
  last_total INTEGER NOT NULL DEFAULT 0,
  last_added INTEGER NOT NULL DEFAULT 0,
  last_updated INTEGER NOT NULL DEFAULT 0,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS monobase_scrm_departments (
  scrm_department_id TEXT PRIMARY KEY,
  scrm_account_id TEXT NOT NULL DEFAULT '',
  scrm_department_name TEXT NOT NULL DEFAULT '',
  scrm_department_parent_id TEXT NOT NULL DEFAULT '',
  wecom_department_id TEXT NOT NULL DEFAULT '',
  scrm_department_sort INTEGER NOT NULL DEFAULT 0,
  scrm_synced_at TIMESTAMPTZ,
  scrm_updated_at TIMESTAMPTZ,
  scrm_deleted_at TIMESTAMPTZ,
  raw_payload JSONB NOT NULL DEFAULT '{}'::jsonb,
  imported_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_monobase_scrm_departments_wecom
  ON monobase_scrm_departments (wecom_department_id);
