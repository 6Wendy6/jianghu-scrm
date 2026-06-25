const jsonHeaders = { 'Content-Type': 'application/json' }

async function request(path, options = {}) {
  const res = await fetch(path, options)
  if (!res.ok) {
    const message = await res.text()
    throw new Error(message || `HTTP ${res.status}`)
  }
  return res.json()
}

export const api = {
  summary: () => request('/api/summary'),
  stores: () => request('/api/stores'),
  customers: () => request('/api/customers'),
  guides: () => request('/api/guides'),
  groups: () => request('/api/groups'),
  touches: () => request('/api/touches'),
  tags: () => request('/api/tags'),
  tagGroups: () => request('/api/tag-groups'),
  createTag: (payload) => request('/api/tags', { method: 'POST', headers: jsonHeaders, body: JSON.stringify(payload) }),
  renameTag: (name, nextName) => request(`/api/tags/${encodeURIComponent(name)}/rename`, { method: 'PATCH', headers: jsonHeaders, body: JSON.stringify({ name: nextName }) }),
  toggleTag: (name) => request(`/api/tags/${encodeURIComponent(name)}/toggle`, { method: 'PATCH' }),
  saveTagGroup: (payload) => request('/api/tag-groups', { method: 'POST', headers: jsonHeaders, body: JSON.stringify(payload) })
}
