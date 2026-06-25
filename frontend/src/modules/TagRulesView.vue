<template>
  <div>
    <PageHeader title="标签与规则" description="标签管理管单个标签生命周期；标签组管理只组织在用标签和适用门店。">
      <template #actions><button class="btn primary" @click="openCreateTag">＋ 添加</button></template>
    </PageHeader>
    <Tabs v-model="tab" :tabs="tabs" />
    <DataTable v-if="tab === 'tags'" title="标签管理" :columns="tagColumns" :rows="tags">
      <template #cell-status="{ row }"><StatusTag :text="row.status" :tone="row.status === '正常' ? 'green' : 'red'" /></template>
      <template #cell-actions="{ row }"><button class="btn" @click="openRename(row)">修改名称</button><button class="btn" @click="toggleTag(row)">{{ row.status === '正常' ? '停用' : '启用' }}</button><button class="btn" @click="viewCustomers(row)">查看客户</button></template>
    </DataTable>
    <DataTable v-else-if="tab === 'groups'" title="标签组管理" :columns="groupColumns" :rows="tagGroups">
      <template #cell-tags="{ row }"><StatusTag v-for="tag in row.tags" :key="tag" :text="tag" /></template>
      <template #cell-stores="{ row }">{{ row.stores.join('、') }}</template>
      <template #cell-actions="{ row }"><button class="btn" @click="editGroup(row)">修改</button></template>
    </DataTable>
    <section v-else class="panel"><h3>{{ tabs.find(item => item.key === tab).label }}</h3><p>规则页后续复用同一套表格、抽屉和确认弹窗组件。</p></section>

    <DrawerPanel :open="drawer.open" :title="drawer.title" @close="closeDrawer">
      <div v-if="drawer.type === 'createTag'" class="form-stack">
        <input v-model="tagForm.name" class="input" placeholder="标签名称" />
        <select v-model="tagForm.group" class="input"><option v-for="group in tagGroups" :key="group.name">{{ group.name }}</option></select>
        <button class="btn primary" @click="saveTag">保存标签</button>
      </div>
      <div v-else-if="drawer.type === 'renameTag'" class="form-stack">
        <input v-model="tagForm.name" class="input" />
        <button class="btn primary" @click="saveRename">保存名称</button>
      </div>
      <div v-else-if="drawer.type === 'customers'">
        <p>这里展示标签下客户列表，后续可接客户列表 API。</p>
      </div>
    </DrawerPanel>
  </div>
</template>

<script setup>
import { reactive, ref } from 'vue'
import { api } from '../api/client'
import PageHeader from '../components/PageHeader.vue'
import DataTable from '../components/DataTable.vue'
import DrawerPanel from '../components/DrawerPanel.vue'
import StatusTag from '../components/StatusTag.vue'
import Tabs from '../components/Tabs.vue'

const props = defineProps({ tags: { type: Array, required: true }, tagGroups: { type: Array, required: true } })
const emit = defineEmits(['refresh'])
const tab = ref('tags')
const tabs = [{ key: 'tags', label: '标签管理' }, { key: 'groups', label: '标签组管理' }, { key: 'stats', label: '标签统计' }, { key: 'rules', label: '管理规则' }]
const tagColumns = [{ key: 'name', label: '标签名称' }, { key: 'group', label: '所属标签组' }, { key: 'status', label: '状态' }, { key: 'customers', label: '客户数' }, { key: 'wecom', label: '企微标签' }, { key: 'actions', label: '操作' }]
const groupColumns = [{ key: 'name', label: '标签组' }, { key: 'stores', label: '适用门店' }, { key: 'tags', label: '组内标签' }, { key: 'actions', label: '操作' }]
const drawer = reactive({ open: false, title: '', type: '', current: null })
const tagForm = reactive({ name: '', group: '' })
function closeDrawer() { drawer.open = false }
function openCreateTag() { Object.assign(drawer, { open: true, title: '新增标签', type: 'createTag', current: null }); Object.assign(tagForm, { name: '', group: props.tagGroups[0]?.name || '' }) }
async function saveTag() { await api.createTag({ ...tagForm }); closeDrawer(); emit('refresh') }
function openRename(row) { Object.assign(drawer, { open: true, title: '修改标签名称', type: 'renameTag', current: row }); tagForm.name = row.name }
async function saveRename() { await api.renameTag(drawer.current.name, tagForm.name); closeDrawer(); emit('refresh') }
async function toggleTag(row) { await api.toggleTag(row.name); emit('refresh') }
function viewCustomers(row) { Object.assign(drawer, { open: true, title: `标签客户列表：${row.name}`, type: 'customers', current: row }) }
function editGroup(row) { Object.assign(drawer, { open: true, title: `编辑标签组：${row.name}`, type: 'group', current: row }) }
</script>
