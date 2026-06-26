const baseURL = process.env.BASE_URL || 'http://127.0.0.1:8080'
const durationSeconds = Number(process.env.DURATION_SECONDS || 30)
const concurrency = Number(process.env.CONCURRENCY || 50)
const timeoutMs = Number(process.env.TIMEOUT_MS || 5000)
const targetRPS = Number(process.env.TARGET_RPS || 0)
const perWorkerDelayMs = targetRPS > 0 ? Math.max(0, (concurrency * 1000) / targetRPS) : 0

const scenarios = [
  { method: 'GET', path: '/api/summary', weight: 20 },
  { method: 'GET', path: '/api/customers?page=1&pageSize=20', weight: 25 },
  { method: 'GET', path: '/api/customers/c1', weight: 10 },
  { method: 'GET', path: '/api/tags?page=1&pageSize=20', weight: 20 },
  { method: 'GET', path: '/api/tag-groups?page=1&pageSize=20', weight: 10 },
  { method: 'GET', path: '/api/tags/%E9%AB%98%E6%84%8F%E5%90%91/customers?page=1&pageSize=20', weight: 10 },
  { method: 'GET', path: '/api/health', weight: 5 }
]

const weighted = scenarios.flatMap((scenario) => Array.from({ length: scenario.weight }, () => scenario))
const deadline = Date.now() + durationSeconds * 1000
const latencies = []
const statuses = new Map()
const cache = { hit: 0, miss: 0, none: 0 }
let requests = 0
let failures = 0

function pickScenario() {
  return weighted[Math.floor(Math.random() * weighted.length)]
}

async function requestOnce(workerID) {
  const scenario = pickScenario()
  const controller = new AbortController()
  const timer = setTimeout(() => controller.abort(), timeoutMs)
  const started = performance.now()
  try {
    const response = await fetch(baseURL + scenario.path, {
      method: scenario.method,
      signal: controller.signal,
      headers: {
        'X-Request-ID': `load-${workerID}-${Date.now()}-${Math.random().toString(16).slice(2)}`
      }
    })
    await response.arrayBuffer()
    const duration = performance.now() - started
    latencies.push(duration)
    requests++
    statuses.set(response.status, (statuses.get(response.status) || 0) + 1)
    if (response.status >= 400) failures++
    const cacheHeader = response.headers.get('x-scrm-cache')
    if (cacheHeader === 'HIT') cache.hit++
    else if (cacheHeader === 'MISS') cache.miss++
    else cache.none++
  } catch {
    const duration = performance.now() - started
    latencies.push(duration)
    requests++
    failures++
    statuses.set('error', (statuses.get('error') || 0) + 1)
  } finally {
    clearTimeout(timer)
  }
}

function sleep(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms))
}

async function worker(workerID) {
  while (Date.now() < deadline) {
    await requestOnce(workerID)
    if (perWorkerDelayMs > 0) {
      await sleep(perWorkerDelayMs)
    }
  }
}

function percentile(values, p) {
  if (values.length === 0) return 0
  const index = Math.min(values.length - 1, Math.ceil((p / 100) * values.length) - 1)
  return values[index]
}

function fmt(value) {
  return Number(value).toFixed(2)
}

console.log(`load test baseURL=${baseURL} duration=${durationSeconds}s concurrency=${concurrency} targetRPS=${targetRPS || 'unlimited'}`)
const started = Date.now()
await Promise.all(Array.from({ length: concurrency }, (_, index) => worker(index + 1)))
const elapsedSeconds = (Date.now() - started) / 1000
latencies.sort((a, b) => a - b)

const statusSummary = [...statuses.entries()]
  .sort(([a], [b]) => String(a).localeCompare(String(b)))
  .map(([status, count]) => `${status}:${count}`)
  .join(' ')

console.log(`requests=${requests}`)
console.log(`qps=${fmt(requests / elapsedSeconds)}`)
console.log(`failures=${failures}`)
console.log(`failureRate=${fmt((failures / Math.max(1, requests)) * 100)}%`)
console.log(`p50=${fmt(percentile(latencies, 50))}ms`)
console.log(`p95=${fmt(percentile(latencies, 95))}ms`)
console.log(`p99=${fmt(percentile(latencies, 99))}ms`)
console.log(`max=${fmt(latencies[latencies.length - 1] || 0)}ms`)
console.log(`cacheHit=${cache.hit}`)
console.log(`cacheMiss=${cache.miss}`)
console.log(`cacheNone=${cache.none}`)
console.log(`cacheHitRate=${fmt((cache.hit / Math.max(1, cache.hit + cache.miss)) * 100)}%`)
console.log(`statuses=${statusSummary}`)
