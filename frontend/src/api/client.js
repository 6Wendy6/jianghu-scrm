const jsonHeaders = { 'Content-Type': 'application/json' }

async function request(path, options = {}) {
  const res = await fetch(path, options)
  if (!res.ok) {
    const message = await res.text()
    throw new Error(message || `HTTP ${res.status}`)
  }
  return res.json()
}

const postJSON = (path, payload = {}) => request(path, { method: 'POST', headers: jsonHeaders, body: JSON.stringify(payload) })
const patchJSON = (path, payload = {}) => request(path, { method: 'PATCH', headers: jsonHeaders, body: JSON.stringify(payload) })

export const api = {
  health: () => request('/api/health'),
  bootstrap: () => request('/api/bootstrap'),
  summary: () => request('/api/summary'),
  stores: (params = '') => request(`/api/stores${params}`),
  saveStoreConfig: (storeId, payload) => patchJSON(`/api/stores/${storeId}/config`, payload),
  storeGuides: (storeId) => request(`/api/stores/${storeId}/guides`),
  addStoreGuide: (storeId, payload) => postJSON(`/api/stores/${storeId}/guides`, payload),
  syncHandover: (storeId) => postJSON(`/api/stores/${storeId}/handover/sync`),
  submitHandover: (storeId, payload) => postJSON(`/api/stores/${storeId}/handover/submit`, payload),
  customers: (params = '') => request(`/api/customers${params}`),
  createCustomer: (payload) => postJSON('/api/customers', payload),
  batchTagCustomers: (payload) => postJSON('/api/customers/batch-tags', payload),
  customerDetail: (id) => request(`/api/customers/${id}`),
  touchCustomer: (id, payload) => postJSON(`/api/customers/${id}/touch`, payload),
  addCustomerFollowup: (id, payload) => postJSON(`/api/customers/${id}/followups`, payload),
  addCustomerRelation: (id, payload) => postJSON(`/api/customers/${id}/relations`, payload),
  setMainRelation: (id, relationId) => postJSON(`/api/customers/${id}/relations/${relationId}/set-main`),
  endCustomerRelation: (id, relationId) => postJSON(`/api/customers/${id}/relations/${relationId}/end`),
  createLead: (id, payload) => postJSON(`/api/customers/${id}/lead`, payload),
  recordOrder: (id, payload) => postJSON(`/api/customers/${id}/order`, payload),
  guides: (params = '') => request(`/api/guides${params}`),
  pauseGuide: (id) => postJSON(`/api/guides/${id}/pause`),
  removeGuide: (id) => postJSON(`/api/guides/${id}/remove`),
  guideLifecycle: (id) => request(`/api/guides/${id}/lifecycle`),
  downloadGuideQr: (id) => postJSON(`/api/guides/${id}/qr-download`),
  groups: () => request('/api/groups'),
  createGroup: (payload) => postJSON('/api/groups', payload),
  groupMassTasks: () => request('/api/groups/mass'),
  createGroupMassTask: (payload) => postJSON('/api/groups/mass', payload),
  groupWelcomes: () => request('/api/groups/welcomes'),
  saveGroupWelcome: (payload) => postJSON('/api/groups/welcomes', payload),
  groupSOPs: () => request('/api/groups/sops'),
  saveGroupSOP: (payload) => postJSON('/api/groups/sops', payload),
  groupCalendar: () => request('/api/groups/calendar'),
  saveGroupCalendar: (payload) => postJSON('/api/groups/calendar', payload),
  groupReminders: () => request('/api/groups/reminders'),
  saveGroupReminder: (payload) => postJSON('/api/groups/reminders', payload),
  groupTagGroups: () => request('/api/groups/tag-groups'),
  saveGroupTagGroup: (payload) => postJSON('/api/groups/tag-groups', payload),
  touches: () => request('/api/touches'),
  createTouch: (payload) => postJSON('/api/touches', payload),
  tags: () => request('/api/tags'),
  tagGroups: () => request('/api/tag-groups'),
  createTag: (payload) => postJSON('/api/tags', payload),
  renameTag: (name, nextName) => patchJSON(`/api/tags/${encodeURIComponent(name)}/rename`, { name: nextName }),
  toggleTag: (name) => request(`/api/tags/${encodeURIComponent(name)}/toggle`, { method: 'PATCH' }),
  tagCustomers: (name) => request(`/api/tags/${encodeURIComponent(name)}/customers`),
  saveTagGroup: (payload) => postJSON('/api/tag-groups', payload),
  autoTagRules: () => request('/api/tag-rules/auto'),
  saveAutoTagRule: (payload) => postJSON('/api/tag-rules/auto', payload),
  preTagRules: () => request('/api/tag-rules/pre'),
  savePreTagRule: (payload) => postJSON('/api/tag-rules/pre', payload),
  wecomConfig: () => request('/api/wecom/config'),
  saveWecomConfig: (payload) => postJSON('/api/wecom/config', payload),
  testWecomConnection: () => postJSON('/api/wecom/test-connection'),
  syncWecomUsers: () => postJSON('/api/wecom/users/sync'),
  wecomUsers: (params = '') => request(`/api/wecom/users${params}`),
  updateWecomUserRole: (userid, roleType) => patchJSON(`/api/wecom/users/${encodeURIComponent(userid)}/role`, { roleType }),
  wecomContactWays: () => request('/api/wecom/contact-way'),
  createWecomContactWay: (payload) => postJSON('/api/wecom/contact-way', payload),
  wecomContactWayStats: (id) => request(`/api/wecom/contact-way/${encodeURIComponent(id)}/stats`),
  scrmContactWayBindings: () => request('/api/scrm/contact-way-bindings'),
  bindScrmContactWay: (payload) => postJSON('/api/scrm/contact-way-bindings', payload),
  saveScrmStoreGuides: (storeId, payload) => postJSON(`/api/scrm/stores/${encodeURIComponent(storeId)}/guides`, payload),
  scrmStoreGuides: (storeId) => request(`/api/scrm/stores/${encodeURIComponent(storeId)}/guides`),
  activateScrmContactWayBinding: (bindingId) => postJSON(`/api/scrm/contact-way-bindings/${encodeURIComponent(bindingId)}/activate`),
  switchNextScrmContactWayBinding: (bindingId) => postJSON(`/api/scrm/contact-way-bindings/${encodeURIComponent(bindingId)}/switch-next`)
}
