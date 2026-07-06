<template>
  <div>
    <PageHeader title="企业微信门店码试点" description="绑定企业微信已有联系我 config_id，用 update_contact_way 轮询切换导购，不生成新的二维码。" />

    <div v-if="notice" class="notice" :class="{ error: noticeType === 'error' }">{{ notice }}</div>

    <div class="grid two">
      <section class="panel">
        <div class="panel-head">
          <h3>企业微信配置</h3>
          <button class="btn" @click="testConnection" :disabled="loading">测试连接</button>
        </div>
        <div class="form-grid">
          <label>Corp ID<input v-model="configForm.corpId" autocomplete="off" /></label>
          <label>Agent ID<input v-model="configForm.agentId" autocomplete="off" /></label>
          <label>Secret<input v-model="configForm.secret" type="password" :placeholder="secretPlaceholder" autocomplete="new-password" /></label>
          <label>回调 Token<input v-model="configForm.token" type="password" autocomplete="new-password" /></label>
          <label>EncodingAESKey<input v-model="configForm.encodingAesKey" type="password" autocomplete="new-password" /></label>
          <label class="wide">回调 URL<input v-model="configForm.callbackUrl" autocomplete="off" /></label>
        </div>
        <div class="actions">
          <button class="btn primary" @click="saveConfig" :disabled="loading">保存配置</button>
          <span class="muted">{{ configState }}</span>
        </div>
      </section>

      <section class="panel">
        <div class="panel-head">
          <h3>门店店长码包装</h3>
          <button class="btn primary" @click="bindExistingContactWay" :disabled="loading || !contactWayForm.configId">绑定原二维码</button>
        </div>
        <div class="form-grid">
          <label>门店编码<input v-model="contactWayForm.storeCode" /></label>
          <label>门店名称<input v-model="contactWayForm.storeName" /></label>
          <label>店长 userid<input v-model="contactWayForm.managerUserid" /></label>
          <label>原二维码 config_id<input v-model="contactWayForm.configId" /></label>
          <label class="wide">原二维码图片 URL<input v-model="contactWayForm.qrCodeUrl" /></label>
          <label class="wide">备注<input v-model="contactWayForm.remark" /></label>
        </div>
        <div class="actions">
          <button class="btn" @click="saveGuidePool" :disabled="loading || !selectedBinding || contactWayForm.guideUserids.length === 0">保存导购池</button>
          <span class="muted">从下方成员表勾选导购，绑定后可写入该门店轮询池。</span>
        </div>
        <div class="chips">
          <label v-for="user in selectedRoleUsers" :key="user.userid" class="check-chip">
            <input v-model="contactWayForm.guideUserids" type="checkbox" :value="user.userid" />
            {{ user.name || user.userid }} · {{ roleLabel(user.roleType) }}
          </label>
        </div>
      </section>
    </div>

    <section class="panel">
      <div class="panel-head">
        <h3>部门成员</h3>
        <div class="toolbar">
          <select v-model="filters.roleType" @change="loadUsers">
            <option value="">全部角色</option>
            <option value="sales">销售</option>
            <option value="guide">导购</option>
            <option value="admin">管理员</option>
            <option value="unknown">未分类</option>
          </select>
          <input v-model="filters.keyword" placeholder="搜索姓名 / userid" @keyup.enter="loadUsers" />
          <button class="btn" @click="loadUsers">查询</button>
          <button class="btn primary" @click="syncUsers" :disabled="loading">同步指定部门</button>
        </div>
      </div>
      <table class="data-table">
        <thead>
          <tr>
            <th>成员</th>
            <th>部门</th>
            <th>联系方式</th>
            <th>企微状态</th>
            <th>业务角色</th>
            <th>同步时间</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="user in users" :key="user.userid">
            <td><strong>{{ user.name || '-' }}</strong><span class="sub">{{ user.userid }}</span></td>
            <td>{{ user.departmentName || user.departmentId || '-' }}</td>
            <td><span>{{ user.mobile || '-' }}</span><span class="sub">{{ user.email }}</span></td>
            <td>{{ user.status }}</td>
            <td>
              <select :value="user.roleType" @change="updateRole(user, $event.target.value)">
                <option value="unknown">未分类</option>
                <option value="sales">销售</option>
                <option value="guide">导购</option>
                <option value="admin">管理员</option>
              </select>
            </td>
            <td>{{ user.syncedAt || '-' }}</td>
          </tr>
          <tr v-if="users.length === 0"><td colspan="6" class="empty">暂无成员，先保存配置并同步指定部门。</td></tr>
        </tbody>
      </table>
    </section>

    <section class="panel">
      <div class="panel-head">
        <h3>店长码包装列表</h3>
        <button class="btn" @click="loadContactWays">刷新</button>
      </div>
      <div class="contact-way-grid">
        <article v-for="way in contactWays" :key="way.id" class="contact-way-card" @click="selectContactWay(way)">
          <img v-if="way.qrCodeUrl" :src="way.qrCodeUrl" alt="企业微信活码" />
          <div>
            <strong>{{ way.storeName }}</strong>
            <span class="sub">当前导购: {{ way.currentGuideUserid || '未激活' }}</span>
            <span class="sub">config_id: {{ way.configId }}</span>
            <span class="metric">{{ way.customerCount || 0 }} 新增客户 · next {{ way.nextIndex }}</span>
          </div>
        </article>
        <div v-if="contactWays.length === 0" class="empty">暂无绑定。先绑定企业微信已有二维码的 config_id。</div>
      </div>

      <div v-if="selectedBinding" class="actions">
        <button class="btn primary" @click="activateBinding" :disabled="loading">激活到第一位导购</button>
        <button class="btn" @click="switchNextBinding" :disabled="loading">手动切到下一位</button>
        <span class="muted">{{ selectedBinding.lastError || `已选择 ${selectedBinding.storeName}` }}</span>
      </div>

      <div v-if="stats" class="stats-grid">
        <section>
          <h4>绑定人员</h4>
          <div class="list-row" v-for="user in stats.boundUsers" :key="user.userid">
            {{ user.name || user.userid }}<span>{{ roleLabel(user.roleType) }}</span>
          </div>
        </section>
        <section>
          <h4>按跟进人</h4>
          <div class="list-row" v-for="item in stats.byFollowUserid" :key="item.key">
            {{ item.key || '未识别' }}<span>{{ item.count }}</span>
          </div>
        </section>
        <section>
          <h4>按日期</h4>
          <div class="list-row" v-for="item in stats.byDate" :key="item.key">
            {{ item.key }}<span>{{ item.count }}</span>
          </div>
        </section>
        <section>
          <h4>最近添加</h4>
          <div class="list-row" v-for="event in stats.recentCustomers" :key="event.id">
            {{ event.externalUserid || '-' }}<span>{{ event.followUserid || '-' }}</span>
          </div>
        </section>
      </div>
    </section>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import PageHeader from '../components/PageHeader.vue'
import { api } from '../api/client'

const loading = ref(false)
const notice = ref('')
const noticeType = ref('info')
const config = ref(null)
const users = ref([])
const contactWays = ref([])
const stats = ref(null)
const selectedBinding = ref(null)

const configForm = reactive({
  corpId: '',
  agentId: '',
  secret: '',
  token: '',
  encodingAesKey: '',
  callbackUrl: '',
  testDepartmentId: ''
})

const filters = reactive({ roleType: '', keyword: '' })
const contactWayForm = reactive({
  storeCode: 'S001',
  storeName: '深圳南山万象天地店',
  managerUserid: '',
  configId: '',
  qrCodeUrl: '',
  remark: '门店店长码包装',
  guideUserids: []
})

const secretPlaceholder = computed(() => config.value?.secretConfigured ? '已配置，留空则不更新' : '请输入 Secret')
const configState = computed(() => config.value?.corpId ? '已配置，成员范围由企业微信后台权限限制' : '尚未保存配置')
const selectedRoleUsers = computed(() => users.value.filter((user) => ['sales', 'guide'].includes(user.roleType)))

function setNotice(message, type = 'info') {
  notice.value = message
  noticeType.value = type
}

function roleLabel(role) {
  return { sales: '销售', guide: '导购', admin: '管理员', unknown: '未分类' }[role] || role
}

async function loadConfig() {
  const res = await api.wecomConfig()
  config.value = res.config || null
  if (config.value) {
    Object.assign(configForm, {
      corpId: config.value.corpId || '',
      agentId: config.value.agentId || '',
      secret: '',
      token: '',
      encodingAesKey: '',
      callbackUrl: config.value.callbackUrl || '',
      testDepartmentId: config.value.testDepartmentId || ''
    })
  }
}

async function saveConfig() {
  await run(async () => {
    const res = await api.saveWecomConfig(configForm)
    config.value = res.config
    configForm.secret = ''
    setNotice('企业微信配置已保存。')
  })
}

async function testConnection() {
  await run(async () => {
    await api.testWecomConnection()
    setNotice('企业微信连接测试成功。')
  })
}

async function syncUsers() {
  await run(async () => {
    const res = await api.syncWecomUsers()
    setNotice(`同步完成：新增 ${res.added}，更新 ${res.updated}，失败 ${res.failed}。`)
    await loadUsers()
  })
}

async function loadUsers() {
  const params = new URLSearchParams()
  if (filters.roleType) params.set('role_type', filters.roleType)
  if (filters.keyword) params.set('keyword', filters.keyword)
  users.value = await api.wecomUsers(params.toString() ? `?${params}` : '')
}

async function updateRole(user, roleType) {
  await run(async () => {
    const next = await api.updateWecomUserRole(user.userid, roleType)
    Object.assign(user, next)
    setNotice(`${user.name || user.userid} 已设置为${roleLabel(roleType)}。`)
  })
}

async function bindExistingContactWay() {
  await run(async () => {
    const way = await api.bindScrmContactWay(contactWayForm)
    setNotice('已绑定企业微信原二维码 config_id，没有生成新二维码。')
    contactWays.value = [way, ...contactWays.value.filter((item) => item.id !== way.id)]
    await selectContactWay(way)
  })
}

async function saveGuidePool() {
  if (!selectedBinding.value) return
  await run(async () => {
    const selected = new Set(contactWayForm.guideUserids)
    const guides = users.value
      .filter((user) => selected.has(user.userid))
      .map((user, index) => ({
        guideUserid: user.userid,
        guideName: user.name || user.userid,
        sortOrder: index + 1,
        status: 'active'
      }))
    await api.saveScrmStoreGuides(selectedBinding.value.storeId, { guides })
    setNotice(`导购池已保存：${guides.length} 人。`)
  })
}

async function loadContactWays() {
  contactWays.value = await api.scrmContactWayBindings()
  if (!selectedBinding.value && contactWays.value.length > 0) {
    selectedBinding.value = contactWays.value[0]
  }
}

async function selectContactWay(way) {
  selectedBinding.value = way
  stats.value = null
}

async function activateBinding() {
  if (!selectedBinding.value) return
  await run(async () => {
    selectedBinding.value = await api.activateScrmContactWayBinding(selectedBinding.value.id)
    setNotice(`已调用 update_contact_way，当前导购：${selectedBinding.value.currentGuideUserid}。`)
    await loadContactWays()
  })
}

async function switchNextBinding() {
  if (!selectedBinding.value) return
  await run(async () => {
    selectedBinding.value = await api.switchNextScrmContactWayBinding(selectedBinding.value.id)
    setNotice(`已切换到下一位导购：${selectedBinding.value.currentGuideUserid}。`)
    await loadContactWays()
  })
}

async function run(task) {
  loading.value = true
  try {
    await task()
  } catch (error) {
    setNotice(error.message, 'error')
  } finally {
    loading.value = false
  }
}

onMounted(async () => {
  await run(async () => {
    await loadConfig()
    await Promise.all([loadUsers().catch(() => {}), loadContactWays().catch(() => {})])
  })
})
</script>
