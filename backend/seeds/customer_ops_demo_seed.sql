-- Demo seed for the customer operation center.
-- This file is intentionally separate from migrations so production schema
-- changes do not implicitly insert demo customers, tasks, materials, or risks.

INSERT INTO tag_groups (id, name, scope, display_order)
VALUES
  ('tg-ops-source', '运营来源标签', '全部门店', 11),
  ('tg-ops-status', '运营状态标签', '全部门店', 12),
  ('tg-ops-interest', '运营兴趣标签', '全部门店', 13)
ON CONFLICT (id) DO NOTHING;

INSERT INTO tags (id, tag_group_id, name, status, wecom_tag_id, category, color, source, is_enabled)
VALUES
  ('tag_xhs', 'tg-ops-source', '小红书来源', 'active', '', '来源标签', 'blue', 'auto', true),
  ('tag_store', 'tg-ops-source', '门店扫码', 'active', '', '来源标签', 'green', 'auto', true),
  ('tag_douyin', 'tg-ops-source', '抖音来源', 'active', '', '来源标签', 'purple', 'auto', true),
  ('tag_intent', 'tg-ops-status', '高意向', 'active', '', '行为标签', 'orange', 'manual', true),
  ('tag_women', 'tg-ops-interest', '女装意向', 'active', '', '兴趣标签', 'orange', 'manual', true),
  ('tag_kids', 'tg-ops-interest', '童装意向', 'active', '', '兴趣标签', 'teal', 'manual', true),
  ('tag_dormant', 'tg-ops-status', '沉睡客户', 'active', '', '风险标签', 'red', 'auto', true),
  ('tag_deal', 'tg-ops-status', '已成交', 'active', '', '消费标签', 'green', 'auto', true)
ON CONFLICT (tag_group_id, name) DO UPDATE SET
  category = EXCLUDED.category,
  color = EXCLUDED.color,
  source = EXCLUDED.source,
  is_enabled = EXCLUDED.is_enabled,
  updated_at = now();

INSERT INTO customers (
  id, name, wecom_name, nickname, mobile_masked, avatar, source_channel,
  region_id, region_name, store_id, store_name, source_store_id, source_store_name,
  owner_staff_id, owner_staff_name, owner_guide_id, owner_guide_name,
  lifecycle_stage, intention_level, status, add_wecom_time, last_follow_up_time,
  last_interaction_time, sales_stage, deal_status, risk, created_at, updated_at
) VALUES
  ('oc1', '陈女士', '小陈', '小陈', '138****8821', '陈', '小红书',
   'r_south', '华南', 's1', '深圳南山万象天地店', 's1', '深圳南山万象天地店',
   'g1', '江诗颖', 'g1', '江诗颖',
   'high_intent', '高意向', '跟进中', '2026-07-06 14:28+08', '2026-07-06 15:10+08',
   '2026-07-06 15:28+08', '待转化', '未成交', '24小时内需二次跟进', now(), now()),
  ('oc2', '王女士', 'Wendy', 'Wendy', '136****9120', '王', '门店扫码',
   'r_south', '华南', 's2', '广州天河代理店', 's2', '广州天河代理店',
   'g5', '张婷', 'g5', '张婷',
   'purchased', '已成交', '已成交', '2026-07-04 10:20+08', '2026-07-05 20:10+08',
   '2026-07-05 20:30+08', '已成交', '已成交 ¥980', '待售后回访', now(), now()),
  ('oc3', '李先生', 'Lee', 'Lee', '未绑定手机号', '李', '门店物料码',
   'r_south', '华南', 's1', '深圳南山万象天地店', 's1', '深圳南山万象天地店',
   'g2', '王敏', 'g2', '王敏',
   'first_follow_pending', '待判断', '待首次跟进', '2026-07-06 13:02+08', NULL,
   '2026-07-06 13:02+08', '待识别', '未成交', '超过30分钟未首次跟进', now(), now()),
  ('oc4', '赵女士', '赵赵', '赵赵', '', '赵', '抖音',
   'r_east', '华东', 's3', '杭州湖滨银泰店', 's3', '杭州湖滨银泰店',
   'g6', '刘雨', 'g6', '刘雨',
   'dormant', '中意向', '沉睡', '2026-05-20 11:00+08', '2026-05-28 11:00+08',
   '2026-06-01 09:10+08', '待转化', '未成交', '30天未互动', now(), now())
ON CONFLICT (id) DO UPDATE SET
  name = EXCLUDED.name,
  wecom_name = EXCLUDED.wecom_name,
  nickname = EXCLUDED.nickname,
  mobile_masked = EXCLUDED.mobile_masked,
  avatar = EXCLUDED.avatar,
  source_channel = EXCLUDED.source_channel,
  region_id = EXCLUDED.region_id,
  region_name = EXCLUDED.region_name,
  store_id = EXCLUDED.store_id,
  store_name = EXCLUDED.store_name,
  source_store_id = EXCLUDED.source_store_id,
  source_store_name = EXCLUDED.source_store_name,
  owner_staff_id = EXCLUDED.owner_staff_id,
  owner_staff_name = EXCLUDED.owner_staff_name,
  owner_guide_id = EXCLUDED.owner_guide_id,
  owner_guide_name = EXCLUDED.owner_guide_name,
  intention_level = EXCLUDED.intention_level,
  status = EXCLUDED.status,
  updated_at = now();

INSERT INTO customer_tags (customer_id, tag_id, source, operator_id, relation_id)
VALUES
  ('oc1', 'tag_xhs', 'auto', 'system', 'link1'),
  ('oc1', 'tag_intent', 'manual', 'g1', 'link2'),
  ('oc1', 'tag_women', 'manual', 'g1', 'link3'),
  ('oc2', 'tag_store', 'auto', 'system', 'link4'),
  ('oc2', 'tag_deal', 'auto', 'system', 'link5'),
  ('oc3', 'tag_store', 'auto', 'system', 'link6'),
  ('oc4', 'tag_douyin', 'auto', 'system', 'link7'),
  ('oc4', 'tag_kids', 'manual', 'g6', 'link8'),
  ('oc4', 'tag_dormant', 'auto', 'system', 'link9')
ON CONFLICT (customer_id, tag_id) DO NOTHING;

INSERT INTO customer_operation_timeline (id, customer_id, event_type, title, content, related_id, operator_id, operator_name, created_at)
VALUES
  ('tl1', 'oc1', 'source', '扫码进入', '小红书活动页扫码进入私域', '', 'system', '系统', '2026-07-06 14:28+08'),
  ('tl2', 'oc1', 'assignment', '客户分配', '分配给导购江诗颖', '', 'system', '系统', '2026-07-06 14:31+08'),
  ('tl3', 'oc1', 'followup', '首次跟进', '客户关注夏季连衣裙', 'fu1', 'g1', '江诗颖', '2026-07-06 15:10+08'),
  ('tl4', 'oc2', 'deal', '完成成交', '成交金额 ¥980', '', 'g5', '张婷', '2026-07-05 20:10+08'),
  ('tl5', 'oc3', 'task', '首次跟进任务逾期', '客户已分配但超过30分钟未首次跟进', 'ot1', 'system', '系统', '2026-07-06 13:35+08'),
  ('tl6', 'oc4', 'risk', '沉睡客户识别', '30天未互动，生成沉睡唤醒任务', 'ot4', 'system', '系统', '2026-07-06 09:00+08')
ON CONFLICT (id) DO NOTHING;

INSERT INTO follow_up_records (id, customer_id, store_id, guide_id, follow_up_type, content, result, next_follow_up_time, stage_before, stage_after, created_by, created_at)
VALUES
  ('fu1', 'oc1', 's1', 'g1', '首次沟通', '客户关注夏季连衣裙，已发送试穿邀请。', '需继续跟进', '2026-07-06 18:00+08', 'following', 'high_intent', '江诗颖', '2026-07-06 15:10+08')
ON CONFLICT (id) DO NOTHING;

INSERT INTO sop_tasks (
  id, task_type, title, description, customer_id, customer_name, region_id, region_name, store_id, store_name,
  assigned_to_user_id, assigned_to_name, assigned_to_role, priority, status, due_time, source, created_at, updated_at
) VALUES
  ('ot1', '新客户首次跟进', '首次跟进李先生', '客户已分配但超过30分钟未首次跟进。', 'oc3', '李先生', 'r_south', '华南', 's1', '深圳南山万象天地店', 'g2', '王敏', 'guide', 'P0', 'overdue', '2026-07-06 13:35+08', '客户分配', now(), now()),
  ('ot2', '高意向客户回访', '回访陈女士连衣裙需求', '客户打开活动链接并被标记为高意向，需要二次触达。', 'oc1', '陈女士', 'r_south', '华南', 's1', '深圳南山万象天地店', 'g1', '江诗颖', 'guide', 'P1', 'pending', '2026-07-06 18:00+08', 'SOP', now(), now()),
  ('ot3', '成交客户售后回访', '王女士成交后售后回访', '成交后第3天确认体验并推荐搭配。', 'oc2', '王女士', 'r_south', '华南', 's2', '广州天河代理店', 'g5', '张婷', 'guide', 'P2', 'pending', '2026-07-08 11:00+08', '成交回写', now(), now()),
  ('ot4', '沉睡客户唤醒', '赵女士30天未互动唤醒', '客户超过30天未互动，建议使用童装上新话术。', 'oc4', '赵女士', 'r_east', '华东', 's3', '杭州湖滨银泰店', 'g6', '刘雨', 'guide', 'P1', 'pending', '2026-07-06 20:00+08', '自动规则', now(), now())
ON CONFLICT (id) DO NOTHING;

INSERT INTO materials (id, title, content, material_type, applicable_stage, applicable_tags, status, created_by, created_at, updated_at)
VALUES
  ('om1', '新客欢迎 + 需求确认', '您好，我是门店顾问。看到您刚添加企业微信，我可以先了解一下您最近想看的品类和预算。', '首次跟进话术', 'first_follow_pending', ARRAY['首次跟进'], 'enabled', '总部运营', now(), now()),
  ('om2', '高意向连衣裙逼单', '这款连衣裙今天门店还有试穿名额，老客价可以保留到今晚。我帮您约一个到店时间？', '高意向跟进话术', 'high_intent', ARRAY['高意向','女装意向'], 'enabled', '总部运营', now(), now()),
  ('om3', '成交后售后回访', '您上次购买的商品体验怎么样？如果尺码或搭配有问题，我可以帮您一起调整。', '售后回访话术', 'purchased', ARRAY['已成交'], 'enabled', '总部运营', now(), now()),
  ('om4', '沉睡客户唤醒', '我们最近上新了几款适合您之前关注风格的款式，老客户有专属试穿福利。', '沉睡唤醒话术', 'dormant', ARRAY['沉睡客户'], 'enabled', '总部运营', now(), now())
ON CONFLICT (id) DO NOTHING;

INSERT INTO operation_exceptions (
  id, exception_type, title, description, severity, region_id, region_name, store_id, store_name,
  customer_id, customer_name, task_id, assigned_to, status, suggestion, created_at
) VALUES
  ('oe_seed_ot1', 'task_overdue', '跟进任务逾期', '任务已超过截止时间：2026-07-06 13:35', 'P0', 'r_south', '华南', 's1', '深圳南山万象天地店', 'oc3', '李先生', 'ot1', '王敏', 'pending', '立即分派导购处理，并使用对应阶段话术。', now()),
  ('oe_seed_oc3', 'first_follow_timeout', '首次跟进超时', '超过30分钟未首次跟进', 'P0', 'r_south', '华南', 's1', '深圳南山万象天地店', 'oc3', '李先生', '', '王敏', 'pending', '请店长立即督促导购首次跟进，或转派给可接待导购。', now()),
  ('oe_seed_oc4', 'customer_dormant', '沉睡客户', '30天未互动', 'P1', 'r_east', '华东', 's3', '杭州湖滨银泰店', 'oc4', '赵女士', '', '刘雨', 'pending', '生成沉睡唤醒任务，并使用沉睡唤醒话术。', now())
ON CONFLICT DO NOTHING;
