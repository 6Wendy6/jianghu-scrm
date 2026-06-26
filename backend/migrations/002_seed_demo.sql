-- Demo seed data for local PostgreSQL runs.
-- This file is idempotent and can be applied repeatedly during development.

INSERT INTO customers (
  id,
  name,
  wecom_name,
  mobile_masked,
  lifecycle_stage,
  sales_stage,
  owner_staff_id,
  owner_staff_name,
  source_store_id,
  source_store_name,
  intent_score,
  created_at,
  updated_at
) VALUES
  ('c1', '陈女士', '小陈', '138****8821', '新客待转化', '待转化', 'g1', '江诗颖', 's1', '蔻斯汀 / 南山店', 86, now(), now()),
  ('c2', '王女士', 'Wendy', '136****9120', '复购培育', '已成交', 'g2', '王敏', 's1', '蔻斯汀 / 南山店', 70, now(), now()),
  ('c3', '李先生', 'Lee', '未绑定手机号', '待交接', '待识别', 'g4', '赵敏', 's1', '蔻斯汀 / 南山店', 42, now(), now())
ON CONFLICT (id) DO NOTHING;

INSERT INTO customer_identities (id, customer_id, identity_type, identity_value, is_primary)
VALUES
  ('ci1', 'c1', 'external_userid', 'external-c1', true),
  ('ci2', 'c1', 'mobile', '138****8821', false),
  ('ci3', 'c2', 'external_userid', 'external-c2', true),
  ('ci4', 'c3', 'external_userid', 'external-c3', true)
ON CONFLICT (identity_type, identity_value) DO NOTHING;

INSERT INTO customer_attributions (
  customer_id,
  source_store_id,
  source_brand_id,
  first_staff_id,
  source_code_id,
  entry_mode,
  entry_at,
  current_service_store_id,
  main_follow_staff_id
) VALUES
  ('c1', 's1', 'brand-kst', 'g1', 'code-g1', 'wecom_first', now(), 's1', 'g1'),
  ('c2', 's1', 'brand-kst', 'g2', 'code-g2', 'wecom_first', now(), 's1', 'g2'),
  ('c3', 's1', 'brand-kst', 'g4', 'code-g4', 'wecom_first', now(), 's1', 'g4')
ON CONFLICT (customer_id) DO NOTHING;

INSERT INTO customer_staff_relations (
  id,
  customer_id,
  staff_id,
  staff_name,
  via_code_id,
  store_id,
  is_main_follow,
  status
) VALUES
  ('r1', 'c1', 'g1', '江诗颖', 'code-g1', 's1', true, 'active'),
  ('r2', 'c2', 'g2', '王敏', 'code-g2', 's1', true, 'active'),
  ('r3', 'c3', 'g4', '赵敏', 'code-g4', 's1', true, 'active')
ON CONFLICT (id) DO NOTHING;

INSERT INTO store_codes (
  id,
  code_type,
  store_id,
  staff_id,
  entry_mode,
  welcome_rule_id,
  status,
  qr_payload
) VALUES
  ('code-g1', 'guide_code', 's1', 'g1', 'wecom_first', 'welcome-default', 'active', '南山店-江诗颖'),
  ('code-g2', 'guide_code', 's1', 'g2', 'wecom_first', 'welcome-default', 'active', '南山店-王敏'),
  ('code-g4', 'guide_code', 's1', 'g4', 'wecom_first', 'welcome-default', 'destroyed', '南山店-赵敏'),
  ('code-material-s1', 'material_code', 's1', '', 'wecom_first', 'welcome-default', 'active', '南山店-门店物料码')
ON CONFLICT (id) DO NOTHING;

INSERT INTO tag_groups (id, name, scope, display_order)
VALUES
  ('tg-source', '来源渠道', '全部门店', 1),
  ('tg-status', '客户状态', '全部门店', 2)
ON CONFLICT (id) DO NOTHING;

INSERT INTO tags (id, tag_group_id, name, status, wecom_tag_id)
VALUES
  ('tag-offline', 'tg-source', '线下门店', 'active', 'wecom-offline'),
  ('tag-nanshan', 'tg-source', '南山店', 'active', 'wecom-nanshan'),
  ('tag-intent', 'tg-status', '高意向', 'active', 'wecom-intent'),
  ('tag-deal', 'tg-status', '已成交', 'active', 'wecom-deal')
ON CONFLICT (tag_group_id, name) DO NOTHING;

INSERT INTO customer_tags (customer_id, tag_id, source)
VALUES
  ('c1', 'tag-offline', 'seed'),
  ('c1', 'tag-intent', 'seed'),
  ('c2', 'tag-deal', 'seed'),
  ('c3', 'tag-offline', 'seed')
ON CONFLICT (customer_id, tag_id) DO NOTHING;
