-- Persistent store handover workflow for guide leave/transfer handling.

CREATE TABLE IF NOT EXISTS store_handover_syncs (
  store_id TEXT PRIMARY KEY REFERENCES stores(id) ON DELETE CASCADE,
  synced BOOLEAN NOT NULL DEFAULT false,
  synced_at TIMESTAMPTZ,
  source TEXT NOT NULL DEFAULT 'manual',
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS store_handover_items (
  id TEXT PRIMARY KEY,
  store_id TEXT NOT NULL REFERENCES stores(id) ON DELETE CASCADE,
  customer_id TEXT NOT NULL DEFAULT '',
  customer_name TEXT NOT NULL DEFAULT '',
  reason TEXT NOT NULL DEFAULT '',
  current_owner TEXT NOT NULL DEFAULT '',
  required_action TEXT NOT NULL DEFAULT '',
  receiver TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL DEFAULT '待同步',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  submitted_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_store_handover_items_store_status ON store_handover_items (store_id, status, updated_at DESC);
CREATE INDEX IF NOT EXISTS idx_store_handover_items_customer ON store_handover_items (customer_id);

INSERT INTO store_handover_syncs (store_id, synced, source)
VALUES ('s1', false, 'seed')
ON CONFLICT (store_id) DO NOTHING;

INSERT INTO store_handover_items (
  id,
  store_id,
  customer_id,
  customer_name,
  reason,
  current_owner,
  required_action,
  receiver,
  status
) VALUES
  ('h1', 's1', 'c3', '李先生', '赵敏离职', '赵敏', '强制指派', '', '待同步'),
  ('h2', 's1', 'c4', '周女士', '林浩调店', '林浩', '手动判断', '', '待同步')
ON CONFLICT (id) DO NOTHING;
