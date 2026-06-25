<template>
  <div>
    <PageHeader title="触达运营" description="配置欢迎语、群发任务、SOP 和话术素材，实际发送仍走企微确认。">
      <template #actions><button class="btn primary">新建欢迎语</button><button class="btn">新建 SOP</button></template>
    </PageHeader>
    <DataTable title="触达任务" :columns="columns" :rows="touches">
      <template #cell-status="{ row }"><StatusTag :text="row.status" :tone="row.status === '完成' ? 'green' : 'orange'" /></template>
    </DataTable>
  </div>
</template>

<script setup>
import PageHeader from '../components/PageHeader.vue'
import DataTable from '../components/DataTable.vue'
import StatusTag from '../components/StatusTag.vue'
defineProps({ touches: { type: Array, required: true } })
const columns = [{ key: 'name', label: '任务' }, { key: 'type', label: '类型' }, { key: 'scope', label: '范围' }, { key: 'status', label: '状态' }]
</script>
