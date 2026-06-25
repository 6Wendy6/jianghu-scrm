<template>
  <div>
    <PageHeader title="导购工作台" description="导购查看自己的客户、待办、门店活码和聊天侧边栏素材。" />
    <div class="kpi-grid three">
      <KpiCard label="我的客户" :value="customers.length" hint="当前门店授权范围" />
      <KpiCard label="今日新增" value="12" hint="扫码入池" />
      <KpiCard label="今日待办" value="18" hint="SOP 和 AI 建议" />
    </div>
    <DataTable title="我的待办" :columns="columns" :rows="tasks">
      <template #cell-actions><button class="btn green">去跟进</button></template>
    </DataTable>
  </div>
</template>

<script setup>
import PageHeader from '../components/PageHeader.vue'
import KpiCard from '../components/KpiCard.vue'
import DataTable from '../components/DataTable.vue'
defineProps({ guides: { type: Array, required: true }, customers: { type: Array, required: true } })
const columns = [{ key: 'customer', label: '客户' }, { key: 'reason', label: '触发原因' }, { key: 'action', label: '建议动作' }, { key: 'actions', label: '操作' }]
const tasks = [
  { id: 1, customer: '陈女士', reason: '新入池 24 小时内', action: '发送试用反馈话术' },
  { id: 2, customer: '李先生', reason: '未绑定手机号', action: '引导关注服务号并补手机号' },
  { id: 3, customer: '王女士', reason: '成交后 7 天回访', action: '发送售后关怀' }
]
</script>
