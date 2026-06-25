<template>
  <div>
    <PageHeader title="客户管理" description="管理从门店活码进入的客户，来源归因只读，服务归属可交接。" />
    <FilterBar @search="applyFilter" @reset="resetFilter">
      <button class="btn primary">新增客户</button>
      <button class="btn" @click="openBatchTag">批量打标签</button>
      <button class="btn">客户迁移/交接</button>
      <select v-model="filters.guide" class="input"><option>全部导购</option><option>江诗颖</option><option>王敏</option><option>林浩</option></select>
      <select v-model="filters.stage" class="input"><option>全部阶段</option><option>新客待转化</option><option>复购培育</option><option>待交接</option></select>
      <input v-model="filters.keyword" class="input" placeholder="搜索客户姓名/手机号/企微昵称" />
    </FilterBar>
    <DataTable title="客户列表" :columns="columns" :rows="filteredCustomers">
      <template #cell-tags="{ row }"><StatusTag v-for="tag in row.tags" :key="tag" :text="tag" /></template>
      <template #cell-actions="{ row }"><button class="btn" @click="selectedCustomer = row">客户详情</button><button class="btn green">触达客户</button></template>
    </DataTable>
    <DrawerPanel :open="!!selectedCustomer" title="客户详情" @close="selectedCustomer = null">
      <div v-if="selectedCustomer">
        <h3>{{ selectedCustomer.name }}</h3>
        <p>来源归属：{{ selectedCustomer.source }}</p>
        <p>服务归属：主跟进导购 {{ selectedCustomer.owner }}</p>
        <h3>关联导购信息</h3>
        <DataTable :columns="relationColumns" :rows="selectedCustomer.relations || []" row-key="guide">
          <template #cell-main="{ row }"><StatusTag :text="row.main ? '是' : '否'" :tone="row.main ? 'green' : 'blue'" /></template>
        </DataTable>
      </div>
    </DrawerPanel>
  </div>
</template>

<script setup>
import { computed, reactive, ref } from 'vue'
import PageHeader from '../components/PageHeader.vue'
import FilterBar from '../components/FilterBar.vue'
import DataTable from '../components/DataTable.vue'
import DrawerPanel from '../components/DrawerPanel.vue'
import StatusTag from '../components/StatusTag.vue'

const props = defineProps({ customers: { type: Array, required: true } })
const filters = reactive({ guide: '全部导购', stage: '全部阶段', keyword: '' })
const applied = ref({ ...filters })
const selectedCustomer = ref(null)
const columns = [
  { key: 'name', label: '客户' },
  { key: 'owner', label: '当前主导购' },
  { key: 'source', label: '来源活码' },
  { key: 'tagGroup', label: '系统标签组' },
  { key: 'tags', label: '系统标签' },
  { key: 'wecomTagsText', label: '企微标签' },
  { key: 'lastActive', label: '最近互动' },
  { key: 'actions', label: '操作' }
]
const relationColumns = [
  { key: 'guide', label: '导购' },
  { key: 'code', label: '通过活码' },
  { key: 'store', label: '门店' },
  { key: 'linkedAt', label: '关联日期' },
  { key: 'endedAt', label: '结束时间' },
  { key: 'main', label: '主跟进' }
]
const filteredCustomers = computed(() => props.customers.filter(customer => {
  const text = `${customer.name}${customer.mobile}${customer.wecom}${customer.source}`
  return (applied.value.guide === '全部导购' || customer.owner === applied.value.guide)
    && (applied.value.stage === '全部阶段' || customer.stage === applied.value.stage)
    && (!applied.value.keyword || text.includes(applied.value.keyword))
}).map(customer => ({ ...customer, wecomTagsText: customer.wecomTags?.join('、') || '' })))
function applyFilter() { applied.value = { ...filters } }
function resetFilter() { Object.assign(filters, { guide: '全部导购', stage: '全部阶段', keyword: '' }); applyFilter() }
function openBatchTag() { alert('请先勾选客户，批量标签会写入系统标签字段') }
</script>
