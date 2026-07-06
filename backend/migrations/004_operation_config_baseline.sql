-- Persistent operation configuration for customer groups, touch rules, and tag rules.

CREATE TABLE IF NOT EXISTS touch_rules (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  rule_type TEXT NOT NULL DEFAULT '',
  scope TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL DEFAULT 'enabled',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_touch_rules_type_status ON touch_rules (rule_type, status, updated_at DESC);

CREATE TABLE IF NOT EXISTS customer_groups (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  store_name TEXT NOT NULL DEFAULT '',
  owner_name TEXT NOT NULL DEFAULT '',
  tags JSONB NOT NULL DEFAULT '[]'::jsonb,
  member_count INTEGER NOT NULL DEFAULT 0,
  today_join INTEGER NOT NULL DEFAULT 0,
  today_quit INTEGER NOT NULL DEFAULT 0,
  status TEXT NOT NULL DEFAULT 'active',
  today_event TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_customer_groups_store_status ON customer_groups (store_name, status, updated_at DESC);

CREATE TABLE IF NOT EXISTS group_mass_tasks (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  groups JSONB NOT NULL DEFAULT '[]'::jsonb,
  content TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL DEFAULT 'pending',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_group_mass_tasks_status ON group_mass_tasks (status, updated_at DESC);

CREATE TABLE IF NOT EXISTS group_welcomes (
  id TEXT PRIMARY KEY,
  group_name TEXT NOT NULL DEFAULT '',
  content TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL DEFAULT 'enabled',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS group_sops (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  groups JSONB NOT NULL DEFAULT '[]'::jsonb,
  stage TEXT NOT NULL DEFAULT '',
  content TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL DEFAULT 'enabled',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS group_calendar_events (
  id TEXT PRIMARY KEY,
  group_name TEXT NOT NULL DEFAULT '',
  title TEXT NOT NULL,
  event_date TEXT NOT NULL DEFAULT '',
  owner_name TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL DEFAULT 'pending',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_group_calendar_events_date ON group_calendar_events (event_date, status);

CREATE TABLE IF NOT EXISTS group_reminders (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  trigger_text TEXT NOT NULL DEFAULT '',
  owner_name TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL DEFAULT 'enabled',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS group_tag_groups (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  tags JSONB NOT NULL DEFAULT '[]'::jsonb,
  scope TEXT NOT NULL DEFAULT '',
  owner_name TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL DEFAULT 'active',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS auto_tag_rules (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  trigger_text TEXT NOT NULL DEFAULT '',
  actions JSONB NOT NULL DEFAULT '[]'::jsonb,
  scope TEXT NOT NULL DEFAULT '',
  impact INTEGER NOT NULL DEFAULT 0,
  status TEXT NOT NULL DEFAULT 'enabled',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_auto_tag_rules_status ON auto_tag_rules (status, updated_at DESC);

CREATE TABLE IF NOT EXISTS pre_tag_rules (
  id TEXT PRIMARY KEY,
  entry TEXT NOT NULL,
  tags JSONB NOT NULL DEFAULT '[]'::jsonb,
  trigger_text TEXT NOT NULL DEFAULT '',
  period TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL DEFAULT 'enabled',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_pre_tag_rules_status ON pre_tag_rules (status, updated_at DESC);

INSERT INTO touch_rules (id, name, rule_type, scope, status) VALUES
  ('t1', '门店默认欢迎语', '欢迎语', '全部线下门店', '启用'),
  ('t2', '会员日活动通知', '群发任务', '华南直营门店', '执行中'),
  ('t3', '新客 1/3/7 天培育', 'SOP', '新入池客户', '启用')
ON CONFLICT (id) DO NOTHING;

INSERT INTO customer_groups (id, name, store_name, owner_name, tags, member_count, today_join, today_quit, status, today_event, created_at) VALUES
  ('cg1', '南山店会员福利群', '蔻斯汀 / 南山店', '江诗颖', '["门店群","会员福利"]'::jsonb, 286, 18, 2, '运营中', '入群 18 / 退群 2', '2026-06-09 10:40:00+08'),
  ('cg2', '618 试用活动群', '华南门店', '张婷', '["活动群","高意向"]'::jsonb, 198, 9, 1, '运营中', '关键词提醒 3', '2026-06-10 14:22:00+08')
ON CONFLICT (id) DO NOTHING;

INSERT INTO group_welcomes (id, group_name, content, status) VALUES
  ('gw1', '南山店会员福利群', '欢迎加入南山店会员福利群。', '启用')
ON CONFLICT (id) DO NOTHING;

INSERT INTO group_sops (id, name, groups, stage, content, status) VALUES
  ('gs1', '新入群 3 天转化', '["南山店会员福利群"]'::jsonb, '新入群', '群主第 1/3 天提醒新品试用权益。', '启用')
ON CONFLICT (id) DO NOTHING;

INSERT INTO group_calendar_events (id, group_name, title, event_date, owner_name, status) VALUES
  ('gc1', '南山店会员福利群', '会员日福利提醒', '2026-06-28', '江诗颖', '待发送')
ON CONFLICT (id) DO NOTHING;

INSERT INTO group_reminders (id, name, trigger_text, owner_name, status) VALUES
  ('gr1', '关键词提醒', '退货/投诉/价格', '店长', '启用')
ON CONFLICT (id) DO NOTHING;

INSERT INTO group_tag_groups (id, name, tags, scope, owner_name, status) VALUES
  ('gt1', '群类型', '["门店群","活动群"]'::jsonb, '全部门店', '运营', '正常')
ON CONFLICT (id) DO NOTHING;

INSERT INTO auto_tag_rules (id, name, trigger_text, actions, scope, impact, status) VALUES
  ('ar1', '门店码入池打来源', '扫码来源=门店活码', '["打线下门店","打门店标签"]'::jsonb, '全部门店', 3426, '启用')
ON CONFLICT (id) DO NOTHING;

INSERT INTO pre_tag_rules (id, entry, tags, trigger_text, period, status) VALUES
  ('pr1', '南山店试用活动', '["南山店","高意向"]'::jsonb, '扫码/提交表单/进群', '2026-06-24 至 2026-07-24', '启用')
ON CONFLICT (id) DO NOTHING;
