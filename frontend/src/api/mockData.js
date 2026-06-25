export const mockData = {
  summary: {
    todayPool: 128,
    attributionRate: '96.8%',
    pending: 162,
    touchRate: '82%',
    trend: [36, 42, 48, 38, 54, 66, 58],
    suggestions: ['南山店物料码入池环比 -20%', '高意向客户 48 人 24 小时未跟进', '天河代理店关注优先完成率更高']
  },
  stores: [
    { id: 's1', name: '深圳南山万象天地店', internalCode: 'BU-KST-HN-001', externalCode: 'POS-3201', brandRegion: '蔻斯汀 / 华南', type: '直营', guideCount: 8, poolCount: 326 },
    { id: 's2', name: '广州天河代理店', internalCode: 'BU-BA-HN-014', externalCode: 'POS-5108', brandRegion: '品牌 A / 华南', type: '代理', guideCount: 5, poolCount: 188 },
    { id: 's3', name: '杭州湖滨银泰店', internalCode: 'BU-BB-HD-006', externalCode: 'POS-7702', brandRegion: '品牌 B / 华东', type: '直营', guideCount: 6, poolCount: 241 }
  ],
  guides: [
    { id: 'g1', name: '江诗颖', code: '南山店-江诗颖', count: '388 / 12', status: '在职', handling: '正常服务' },
    { id: 'g2', name: '王敏', code: '南山店-王敏', count: '205 / 7', status: '在职', handling: '正常服务' },
    { id: 'g3', name: '林浩', code: '南山店-林浩', count: '96 / 0', status: '已调店', handling: '6 人进交接池' }
  ],
  customers: [
    { id: 'c1', name: '陈女士', wecom: '小陈', mobile: '138****8821', owner: '江诗颖', source: '南山店-江诗颖', stage: '新客待转化', tagGroup: '行为标签 / 生命周期标签', tags: ['线下门店', '高意向'], wecomTags: ['企微好友', '南山门店'], lastActive: '今天 15:10', relations: [{ guide: '江诗颖', code: '南山店-江诗颖', store: '蔻斯汀 / 南山店', linkedAt: '2026-06-24', endedAt: '-', main: true }] },
    { id: 'c2', name: '王女士', wecom: 'Wendy', mobile: '136****9120', owner: '王敏', source: '南山店-王敏', stage: '复购培育', tagGroup: '会员标签 / 生命周期标签', tags: ['VIP', '已成交'], wecomTags: ['企微好友', '老客'], lastActive: '昨天 20:10', relations: [{ guide: '王敏', code: '南山店-王敏', store: '蔻斯汀 / 南山店', linkedAt: '2026-06-20', endedAt: '-', main: true }] }
  ],
  groups: [
    { id: 'cg1', name: '南山店会员福利群', owner: '江诗颖', tags: ['门店群', '会员福利'], count: 286, todayJoin: 18, todayQuit: 2, createdAt: '2026-06-09 10:40' },
    { id: 'cg2', name: '618 试用活动群', owner: '张婷', tags: ['活动群', '高意向'], count: 198, todayJoin: 9, todayQuit: 1, createdAt: '2026-06-10 14:22' }
  ],
  touches: [
    { id: 't1', name: '门店默认欢迎语', type: '欢迎语', scope: '全部线下门店', status: '启用' },
    { id: 't2', name: '会员日活动通知', type: '群发任务', scope: '华南直营门店', status: '执行中' }
  ],
  tags: [
    { name: '线下门店', group: '来源渠道', status: '正常', customers: 1284, wecom: '企微-线下' },
    { name: '南山店', group: '来源渠道', status: '正常', customers: 826, wecom: '企微-南山' },
    { name: '高意向', group: '客户状态', status: '正常', customers: 486, wecom: '企微-高意向' },
    { name: '已流失', group: '客户状态', status: '停用', customers: 35, wecom: '企微-流失' }
  ],
  tagGroups: [
    { name: '来源渠道', stores: ['华南直营门店', '深圳南山万象天地店'], tags: ['线下门店', '南山店'] },
    { name: '客户状态', stores: ['全部门店'], tags: ['高意向', '已流失'] }
  ]
}
