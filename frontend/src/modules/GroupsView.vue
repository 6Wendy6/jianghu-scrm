<template>
  <div>
    <PageHeader title="客户群运营" description="客户群是门店复购资产，承接群档案、群发、入群欢迎语、群 SOP 和群提醒。">
      <template #actions><button class="btn primary">新建客户群</button><button class="btn">批量打群标签</button></template>
    </PageHeader>
    <Tabs v-model="tab" :tabs="tabs" />
    <DataTable v-if="tab === 'manage'" title="客户群管理" :columns="columns" :rows="groups">
      <template #cell-tags="{ row }"><StatusTag v-for="tag in row.tags" :key="tag" :text="tag" /></template>
      <template #cell-actions><button class="btn">详情</button><button class="btn">设置群SOP</button></template>
    </DataTable>
    <section v-else class="panel"><h3>{{ tabs.find(item => item.key === tab).label }}</h3><p>该子页面已预留独立模块，后续接入对应 API 即可复用列表、筛选和抽屉组件。</p></section>
  </div>
</template>

<script setup>
import { ref } from 'vue'
import PageHeader from '../components/PageHeader.vue'
import DataTable from '../components/DataTable.vue'
import StatusTag from '../components/StatusTag.vue'
import Tabs from '../components/Tabs.vue'
defineProps({ groups: { type: Array, required: true } })
const tab = ref('manage')
const tabs = [
  { key: 'manage', label: '客户群管理' },
  { key: 'welcome', label: '入群欢迎语' },
  { key: 'mass', label: '客户群群发' },
  { key: 'sop', label: '群SOP' },
  { key: 'calendar', label: '群日历' },
  { key: 'reminder', label: '客户群提醒' },
  { key: 'tag', label: '客户群标签' }
]
const columns = [
  { key: 'name', label: '群名称' },
  { key: 'owner', label: '群主' },
  { key: 'tags', label: '群标签' },
  { key: 'count', label: '群人数' },
  { key: 'todayJoin', label: '当日入群' },
  { key: 'todayQuit', label: '当日退群' },
  { key: 'createdAt', label: '创建时间' },
  { key: 'actions', label: '操作' }
]
</script>
