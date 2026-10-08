<script setup lang="ts">
import { ref, computed, watch, onMounted, onBeforeUnmount } from 'vue'
import { useRoute } from 'vue-router'
import { NButton, NInput, NSelect } from 'naive-ui'
import { useClusterStore } from '../stores/cluster'
import { catalogApi, type EsIndex } from '../api/catalog'

const route = useRoute()
const clusterStore = useClusterStore()

const clusterId = computed(() => route.params.id as string)
const cluster = computed(() => clusterStore.clusterById(clusterId.value))

const rows = ref<EsIndex[]>([])
const total = ref(0)
const updatedAt = ref('')
const loading = ref(false)
const refreshing = ref(false)
const error = ref('')

// 筛选条件：过滤与分页都在后端做（见 PRD §10）
const search = ref('')
const health = ref('')
const status = ref('')
const page = ref(1)
const pageSize = ref(20)
const sort = ref('index')
const order = ref<'asc' | 'desc'>('asc')

const now = ref(Date.now())
let ticker: number | undefined
let pollTimer: number | undefined
let searchTimer: number | undefined

const healthOptions = [
  { label: '全部 health', value: '' },
  { label: 'green', value: 'green' },
  { label: 'yellow', value: 'yellow' },
  { label: 'red', value: 'red' },
]

const statusOptions = [
  { label: '全部状态', value: '' },
  { label: 'open', value: 'open' },
  { label: 'close', value: 'close' },
]

const pageSizeOptions = [
  { label: '20 / 页', value: 20 },
  { label: '50 / 页', value: 50 },
  { label: '100 / 页', value: 100 },
]

const sortOptions = [
  { label: '按索引名', value: 'index' },
  { label: '按存储', value: 'store' },
  { label: '按文档数', value: 'docs' },
  { label: '按 Health', value: 'health' },
]

async function load(silent = false, force = false, retried = false) {
  const id = clusterId.value
  if (!id) return
  if (!silent) loading.value = true
  try {
    const res = await catalogApi.indices(id, {
      search: search.value.trim() || undefined,
      health: health.value || undefined,
      status: status.value || undefined,
      sort: sort.value || undefined,
      order: order.value,
      page: page.value,
      pageSize: pageSize.value,
    }, force)

    // 过滤条件变化或索引被删除后，当前页可能越界，回落到最后一页
    const pages = Math.max(1, Math.ceil(res.total / pageSize.value))
    if (!retried && page.value > pages) {
      page.value = pages
      return load(silent, force, true)
    }

    rows.value = res.data ?? []
    total.value = res.total
    updatedAt.value = res.updated_at
    error.value = ''
  } catch (e) {
    error.value = e instanceof Error ? e.message : '加载失败'
  } finally {
    loading.value = false
  }
}

async function refresh() {
  refreshing.value = true
  try {
    await load(true, true)
  } finally {
    refreshing.value = false
  }
}

const totalPages = computed(() => Math.max(1, Math.ceil(total.value / pageSize.value)))
const rangeText = computed(() => {
  if (total.value === 0) return '共 0 个索引'
  const start = (page.value - 1) * pageSize.value + 1
  const end = Math.min(total.value, page.value * pageSize.value)
  return `第 ${start}-${end} 个，共 ${total.value} 个索引`
})

const goPrev = () => { if (page.value > 1) { page.value--; load() } }
const goNext = () => { if (page.value < totalPages.value) { page.value++; load() } }

// 搜索防抖 300ms；筛选变化时回到第一页
watch(search, () => {
  if (searchTimer) window.clearTimeout(searchTimer)
  searchTimer = window.setTimeout(() => { page.value = 1; load() }, 300)
})
watch([health, status, pageSize, sort, order], () => { page.value = 1; load() })

const toggleOrder = () => { order.value = order.value === 'asc' ? 'desc' : 'asc' }

onMounted(() => {
  ticker = window.setInterval(() => { now.value = Date.now() }, 5000)
  load()
  // 轮询同时充当「页面仍在查看」的心跳（见 PRD §6.3）
  pollTimer = window.setInterval(() => load(true), 15000)
})

onBeforeUnmount(() => {
  if (ticker) window.clearInterval(ticker)
  if (pollTimer) window.clearInterval(pollTimer)
  if (searchTimer) window.clearTimeout(searchTimer)
})

watch(clusterId, (id) => {
  if (id) {
    page.value = 1
    load()
  }
})

// ---------- 展示辅助 ----------
const HEALTH_CLASS: Record<string, string> = {
  green: 'border-green-border bg-green-bg color-green',
  yellow: 'border-yellow-border bg-yellow-bg color-yellow',
  red: 'border-red-border bg-red-bg color-red',
}
const healthClass = (h: string) => HEALTH_CLASS[(h || '').toLowerCase()] || 'border-border bg-bg-3 text-text-3'

const fmtInt = (value: string) => {
  const n = Number(value)
  return Number.isFinite(n) ? n.toLocaleString('en-US') : (value || '-')
}

const fmtBytes = (value: string) => {
  const bytes = Number(value)
  if (!Number.isFinite(bytes)) return value || '-'
  if (bytes === 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB', 'PB']
  let v = bytes
  let i = 0
  while (v >= 1024 && i < units.length - 1) {
    v /= 1024
    i++
  }
  return `${v >= 100 || i === 0 ? Math.round(v) : v.toFixed(1)} ${units[i]}`
}

const freshnessText = computed(() => {
  if (!updatedAt.value) return '尚未采集'
  const ms = new Date(updatedAt.value).getTime()
  if (!Number.isFinite(ms) || ms < Date.parse('2000-01-01')) return '尚未采集'
  const seconds = Math.max(0, Math.round((now.value - ms) / 1000))
  return `更新于 ${seconds} 秒前`
})
</script>

<template>
  <div class="flex flex-col gap-4">
    <!-- 页头 -->
    <div class="flex items-center gap-2.5 flex-wrap">
      <div class="flex-1 min-w-0">
        <div class="text-18px font-600 tracking--0.4px truncate">索引</div>
        <div class="text-12px font-mono text-text-3 truncate">
          {{ cluster?.name || '集群' }} · {{ cluster?.hosts?.join(', ') || '-' }}
        </div>
      </div>
      <div class="text-11.5px text-text-3">{{ freshnessText }}</div>
      <n-button size="small" :loading="refreshing" @click="refresh">刷新</n-button>
    </div>

    <div v-if="error" class="border border-red-border bg-red-bg rounded-10px px-4 py-3 text-12.5px color-red font-mono">
      {{ error }}
    </div>

    <!-- 工具栏 -->
    <div class="flex items-center gap-2.5 flex-wrap">
      <n-input v-model:value="search" size="small" clearable placeholder="过滤索引名" class="w-64" />
      <n-select v-model:value="health" size="small" :options="healthOptions" class="w-36" />
      <n-select v-model:value="status" size="small" :options="statusOptions" class="w-32" />
      <n-select v-model:value="sort" size="small" :options="sortOptions" class="w-32" />
      <button
        class="btn-icon"
        :title="order === 'asc' ? '当前升序，点击切换为降序' : '当前降序，点击切换为升序'"
        @click="toggleOrder"
      >
        <div v-if="order === 'asc'" class="w-4 h-4 i-lucide-arrow-up"></div>
        <div v-else class="w-4 h-4 i-lucide-arrow-down"></div>
      </button>
      <div class="ml-auto text-11.5px text-text-3">{{ rangeText }}</div>
    </div>

    <!-- 列表 -->
    <div v-if="loading && rows.length === 0" class="border border-border rounded-10px bg-bg-2 p-6 text-13px text-text-2">
      加载中…
    </div>
    <div v-else-if="rows.length === 0" class="border border-border rounded-10px bg-bg-2 p-6 text-13px text-text-2">
      {{ total === 0 && !search && !health && !status ? '该集群暂无索引。' : '没有匹配的索引。' }}
    </div>
    <div v-else class="border border-border rounded-10px bg-bg-2 overflow-hidden">
      <div class="overflow-x-auto">
        <table class="w-full text-12.5px">
          <thead>
            <tr class="text-text-3 text-left">
              <th class="px-4 py-2.5 font-500">索引</th>
              <th class="px-3 py-2.5 font-500">Health</th>
              <th class="px-3 py-2.5 font-500">状态</th>
              <th class="px-3 py-2.5 font-500 text-right">主</th>
              <th class="px-3 py-2.5 font-500 text-right">副</th>
              <th class="px-3 py-2.5 font-500 text-right">文档</th>
              <th class="px-4 py-2.5 font-500 text-right">存储</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="idx in rows" :key="idx.index" class="border-t border-border hover:bg-bg-3">
              <td class="px-4 py-2 font-mono max-w-100 truncate" :title="idx.index">{{ idx.index }}</td>
              <td class="px-3 py-2">
                <span
                  class="inline-flex items-center gap-1.25 px-1.5 py-0.5 rounded-4px border text-11px font-600 uppercase"
                  :class="healthClass(idx.health)"
                >
                  <span class="w-1.5 h-1.5 rounded-full bg-current"></span>{{ idx.health || '-' }}
                </span>
              </td>
              <td class="px-3 py-2 font-mono text-text-2">{{ idx.status || '-' }}</td>
              <td class="px-3 py-2 font-mono text-right">{{ idx.pri || '-' }}</td>
              <td class="px-3 py-2 font-mono text-right">{{ idx.rep || '-' }}</td>
              <td class="px-3 py-2 font-mono text-right">{{ fmtInt(idx['docs.count']) }}</td>
              <td class="px-4 py-2 font-mono text-right text-text-2">{{ fmtBytes(idx['store.size']) }}</td>
            </tr>
          </tbody>
        </table>
      </div>

      <!-- 分页 -->
      <div class="border-t border-border px-4 py-2.5 flex items-center gap-3 flex-wrap">
        <span class="text-11.5px text-text-3">第 {{ page }} / {{ totalPages }} 页</span>
        <div class="ml-auto flex items-center gap-2">
          <n-select v-model:value="pageSize" size="tiny" :options="pageSizeOptions" class="w-24" />
          <n-button size="tiny" :disabled="page <= 1" @click="goPrev">上一页</n-button>
          <n-button size="tiny" :disabled="page >= totalPages" @click="goNext">下一页</n-button>
        </div>
      </div>
    </div>
  </div>
</template>
