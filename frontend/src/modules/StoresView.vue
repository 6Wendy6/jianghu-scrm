<template>
  <div>
    <PageHeader title="门店" description="门店来自底座业务单元，SCRM 只叠加私域配置、接待导购和活码生命周期。">
      <template #actions><button class="btn primary" @click="openConfig(stores[0])">查看私域配置</button></template>
    </PageHeader>
    <FilterBar @search="applyFilter" @reset="resetFilter">
      <select v-model="filters.brand" class="input"><option>全部品牌</option><option>蔻斯汀</option><option>品牌 A</option></select>
      <select v-model="filters.region" class="input"><option>全部区域</option><option>华南</option><option>华东</option></select>
      <select v-model="filters.type" class="input"><option>全部类型</option><option>直营</option><option>代理</option></select>
      <input v-model="filters.keyword" class="input" placeholder="搜索门店名称/编码" />
    </FilterBar>
    <div class="kpi-grid three">
      <KpiCard label="门店数" :value="filteredStores.length" hint="按当前筛选结果统计" />
      <KpiCard label="导购码" :value="guides.length" hint="添加导购后自动生成" />
      <KpiCard label="门店物料码" :value="stores.length" hint="收银台/海报长期投放" />
    </div>
    <DataTable title="门店列表" :columns="columns" :rows="filteredStores">
      <template #cell-type="{ row }"><StatusTag :text="row.type" :tone="row.type === '直营' ? 'green' : 'orange'" /></template>
      <template #cell-actions="{ row }"><button class="btn primary" @click="openConfig(row)">私域配置</button></template>
    </DataTable>

    <DrawerPanel :open="!!selectedStore" title="门店私域配置" @close="selectedStore = null">
      <div v-if="selectedStore">
        <h3>{{ selectedStore.name }}</h3>
        <div class="grid two">
          <section class="panel"><h3>私域承接策略</h3><p>默认入池模式：企微优先</p><p>活码轮询：按顺序轮巡</p><p>客户群：南山店会员福利群</p></section>
          <section class="panel"><h3>二维码入口</h3><div class="qr">QR</div><button class="btn" @click="downloadQr">下载二维码</button></section>
        </div>
        <DataTable title="门店活码生命周期" :columns="guideColumns" :rows="guides">
          <template #actions><button class="btn primary">添加接待导购</button><button class="btn green">处理导购交接</button></template>
          <template #cell-status="{ row }"><StatusTag :text="row.status" :tone="row.status === '在职' ? 'green' : 'orange'" /></template>
          <template #cell-actions="{ row }"><button class="btn">查看</button><button class="btn">查看客户</button><button class="btn">下载活码</button></template>
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
import KpiCard from '../components/KpiCard.vue'
import StatusTag from '../components/StatusTag.vue'

const props = defineProps({ stores: { type: Array, required: true }, guides: { type: Array, required: true } })
const filters = reactive({ brand: '全部品牌', region: '全部区域', type: '全部类型', keyword: '' })
const applied = ref({ ...filters })
const selectedStore = ref(null)
const columns = [
  { key: 'name', label: '门店名称' },
  { key: 'internalCode', label: '内部编码' },
  { key: 'externalCode', label: '外部编码' },
  { key: 'brandRegion', label: '品牌/区域' },
  { key: 'type', label: '类型' },
  { key: 'guideCount', label: '接待导购' },
  { key: 'poolCount', label: '本月入池' },
  { key: 'actions', label: '操作' }
]
const guideColumns = [
  { key: 'name', label: '导购' },
  { key: 'code', label: '活码名称' },
  { key: 'count', label: '累计/今日' },
  { key: 'status', label: '状态' },
  { key: 'handling', label: '客户处理' },
  { key: 'actions', label: '操作' }
]
const filteredStores = computed(() => props.stores.filter(store => {
  const text = `${store.name}${store.internalCode}${store.externalCode}${store.brandRegion}${store.type}`
  return (applied.value.brand === '全部品牌' || store.brandRegion.includes(applied.value.brand))
    && (applied.value.region === '全部区域' || store.brandRegion.includes(applied.value.region))
    && (applied.value.type === '全部类型' || store.type.includes(applied.value.type))
    && (!applied.value.keyword || text.includes(applied.value.keyword))
}))
function applyFilter() { applied.value = { ...filters } }
function resetFilter() { Object.assign(filters, { brand: '全部品牌', region: '全部区域', type: '全部类型', keyword: '' }); applyFilter() }
function openConfig(store) { selectedStore.value = store }
function downloadQr() { alert('二维码已生成下载任务') }
</script>
