-- SCRM-MVP-02 Enterprise WeChat real-chain integration support.
-- Records update_contact_way failures for headquarters anomaly monitoring and manual retry.

CREATE TABLE IF NOT EXISTS scrm_wecom_contact_way_retry_tasks (
  id TEXT PRIMARY KEY,
  binding_id TEXT NOT NULL REFERENCES scrm_wecom_contact_way_bindings(id) ON DELETE CASCADE,
  config_id TEXT NOT NULL,
  target_guide_userid TEXT NOT NULL DEFAULT '',
  target_state TEXT NOT NULL DEFAULT '',
  failed_reason TEXT NOT NULL DEFAULT '',
  retry_count INTEGER NOT NULL DEFAULT 0,
  status TEXT NOT NULL DEFAULT 'pending',
  next_retry_at TIMESTAMPTZ,
  last_attempt_at TIMESTAMPTZ,
  resolved_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_scrm_wecom_retry_status
  ON scrm_wecom_contact_way_retry_tasks (status, next_retry_at, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_scrm_wecom_retry_binding
  ON scrm_wecom_contact_way_retry_tasks (binding_id, created_at DESC);
