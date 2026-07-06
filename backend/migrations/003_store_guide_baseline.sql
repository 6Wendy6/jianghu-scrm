-- Persistent store and guide baseline for PostgreSQL mode.
-- This keeps store configuration, guide lifecycle state, and guide audit events
-- out of process memory so the API can scale horizontally.

CREATE TABLE IF NOT EXISTS stores (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  internal_code TEXT NOT NULL DEFAULT '',
  external_code TEXT NOT NULL DEFAULT '',
  brand TEXT NOT NULL DEFAULT '',
  region TEXT NOT NULL DEFAULT '',
  store_type TEXT NOT NULL DEFAULT '',
  entry_mode TEXT NOT NULL DEFAULT '',
  service_guide TEXT NOT NULL DEFAULT '',
  group_name TEXT NOT NULL DEFAULT '',
  welcome_rule TEXT NOT NULL DEFAULT '',
  polling_rule TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL DEFAULT 'active',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_stores_brand_region ON stores (brand, region);
CREATE INDEX IF NOT EXISTS idx_stores_updated ON stores (updated_at DESC);

CREATE TABLE IF NOT EXISTS guides (
  id TEXT PRIMARY KEY,
  store_id TEXT NOT NULL REFERENCES stores(id),
  name TEXT NOT NULL,
  code TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL DEFAULT 'active',
  employment_status TEXT NOT NULL DEFAULT 'active',
  paused BOOLEAN NOT NULL DEFAULT false,
  handling TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (store_id, name)
);

CREATE INDEX IF NOT EXISTS idx_guides_store_status ON guides (store_id, status, employment_status);
CREATE INDEX IF NOT EXISTS idx_guides_updated ON guides (updated_at DESC);

CREATE TABLE IF NOT EXISTS guide_events (
  id TEXT PRIMARY KEY,
  guide_id TEXT NOT NULL REFERENCES guides(id) ON DELETE CASCADE,
  action TEXT NOT NULL,
  result TEXT NOT NULL DEFAULT '',
  operator TEXT NOT NULL DEFAULT '',
  people TEXT NOT NULL DEFAULT '',
  detail TEXT NOT NULL DEFAULT '',
  occurred_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_guide_events_guide_time ON guide_events (guide_id, occurred_at DESC);

INSERT INTO stores (
  id,
  name,
  internal_code,
  external_code,
  brand,
  region,
  store_type,
  entry_mode,
  service_guide,
  group_name,
  welcome_rule,
  polling_rule
) VALUES
  ('s1', '深圳南山万象天地店', 'BU-KST-HN-001', 'POS-3201', '蔻斯汀', '华南', '直营', '跟随全局 · 企微优先', '开启', '南山店会员福利群', '门店默认欢迎语', '按顺序轮巡'),
  ('s2', '广州天河代理店', 'BU-BA-HN-014', 'POS-5108', '品牌 A', '华南', '代理', '关注优先', '开启', '618 试用活动群', '天河关注优先欢迎语', '按新导购优先（入职2个月内保护期）'),
  ('s3', '杭州湖滨银泰店', 'BU-BB-HD-006', 'POS-7702', '品牌 B', '华东', '直营', '跟随全局 · 企微优先', '开启', '杭州会员福利群', '门店默认欢迎语', '按顺序轮巡')
ON CONFLICT (id) DO NOTHING;

INSERT INTO guides (
  id,
  store_id,
  name,
  code,
  status,
  employment_status,
  paused,
  handling
) VALUES
  ('g1', 's1', '江诗颖', '南山店-江诗颖', 'active', 'active', false, '正常服务'),
  ('g2', 's1', '王敏', '南山店-王敏', 'active', 'active', false, '正常服务'),
  ('g4', 's1', '赵敏', '南山店-赵敏', 'removed', 'removed', true, '客户关系保留，后续不再分配新客')
ON CONFLICT (id) DO NOTHING;

INSERT INTO guide_events (id, guide_id, action, result, operator, people, detail, occurred_at)
VALUES
  ('gev-g1-seed', 'g1', '导购入职', '已生成导购活码', '系统', '导购：江诗颖', '南山店-江诗颖 已进入物料码轮巡池。', now()),
  ('gev-g2-seed', 'g2', '导购入职', '已生成导购活码', '系统', '导购：王敏', '南山店-王敏 已进入物料码轮巡池。', now()),
  ('gev-g4-seed', 'g4', '移除成员', '停用可恢复', '系统', '导购：赵敏', '客户关系保留，活码退出物料码轮巡。', now())
ON CONFLICT (id) DO NOTHING;
