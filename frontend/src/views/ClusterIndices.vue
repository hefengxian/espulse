<script setup lang="ts">
import { ref, computed, watch, onMounted, onBeforeUnmount, nextTick } from 'vue'
import { useRoute } from 'vue-router'
import { NButton, NInput, NSelect, NCheckbox, NTooltip } from 'naive-ui'
import { useClusterStore } from '../stores/cluster'
import { catalogApi, type EsAlias, type EsIndex } from '../api/catalog'
import { formatAge } from '../utils/freshness'

const route = useRoute()
const clusterStore = useClusterStore()

const clusterId = computed(() => route.params.id as string)
const cluster = computed(() => clusterStore.clusterById(clusterId.value))

const rows = ref<EsIndex[]>([])
const total = ref(0)
const updatedAt = ref('')
// rateWindowMs 是后端算这批速率所用的差分窗口（见 PRD §11）。
// 它随页面打开时长从 5s 长到 60s，因此必须如实展示，不能把短窗口的毛刺当稳定速率。
const rateWindowMs = ref(0)
const loading = ref(false)
const refreshing = ref(false)
const error = ref('')

// 筛选条件：过滤与分页都在后端做（见 PRD §10）
const search = ref('')
const health = ref('')
const status = ref('')
// 系统索引（`.` 开头，如 .security-7 / .kibana_1）多数场景不关心，默认隐藏，可切换（同 Cerebro）
const hideSystem = ref(true)
const page = ref(1)
const pageSize = ref(20)
const sort = ref('index')
const order = ref<'asc' | 'desc'>('asc')

// 索引与别名是同一个「清单」主题的两个视角，合并在同一页，不新开栏目（见 PRD §2.4）
const view = ref<'indices' | 'aliases'>('indices')
const aliases = ref<EsAlias[]>([])
const aliasSearch = ref('')

const VIEW_TABS = [
  { label: '索引', value: 'indices' as const },
  { label: '别名', value: 'aliases' as const },
]

// 从别名视图跳到该索引的索引视图：同一页面内跳转，而不是两处重复展示（见 PRD §6.4）
let skipSearchWatch = false

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
  { label: '按写入速率', value: 'index_rate' },
  { label: '按查询速率', value: 'search_rate' },
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
      hideSystem: hideSystem.value || undefined,
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
    rateWindowMs.value = res.rate_window_ms ?? 0
    error.value = ''
  } catch (e) {
    error.value = e instanceof Error ? e.message : '加载失败'
  } finally {
    loading.value = false
  }
}

// 别名数量有上界，后端一次返回全量，过滤在本地做
async function loadAliases(force = false) {
  const id = clusterId.value
  if (!id) return
  loading.value = true
  try {
    const res = await catalogApi.aliases(id, force)
    aliases.value = res.data ?? []
    updatedAt.value = res.updated_at
    error.value = ''
  } catch (e) {
    error.value = e instanceof Error ? e.message : '加载失败'
  } finally {
    loading.value = false
  }
}

// 两个视图互斥，共用同一份 updatedAt，刷新按钮按当前视图分发
async function refresh() {
  refreshing.value = true
  try {
    if (view.value === 'aliases') await loadAliases(true)
    else await load(true, true)
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
  // 由 showIndex 触发的赋值已由视图切换负责加载，跳过以免重复请求
  if (skipSearchWatch) {
    skipSearchWatch = false
    return
  }
  if (searchTimer) window.clearTimeout(searchTimer)
  searchTimer = window.setTimeout(() => { page.value = 1; load() }, 300)
})
watch([health, status, pageSize, sort, order, hideSystem], () => { page.value = 1; load() })

const toggleOrder = () => { order.value = order.value === 'asc' ? 'desc' : 'asc' }

// 在别名视图里点索引名：切回索引视图并预置过滤条件
const showIndex = (indexName: string) => {
  skipSearchWatch = true
  search.value = indexName
  // 目标是系统索引时先关掉隐藏开关，否则跳过去只会看到「没有匹配的索引」
  if (indexName.startsWith('.')) hideSystem.value = false
  page.value = 1
  view.value = 'indices'
  // watch(search) 是异步 flush 的。无论这次赋值是否真的改变了 search（值相同则 watcher 不触发），
  // 都在本轮 flush 之后清掉标记，避免它残留下来吞掉用户的下一次搜索
  nextTick(() => { skipSearchWatch = false })
}

onMounted(() => {
  ticker = window.setInterval(() => { now.value = Date.now() }, 5000)
  load()
  // 轮询同时充当「页面仍在查看」的心跳（见 PRD §6.3）
  pollTimer = window.setInterval(() => {
    if (view.value === 'aliases') loadAliases()
    else load(true)
  }, 15000)
})

onBeforeUnmount(() => {
  if (ticker) window.clearInterval(ticker)
  if (pollTimer) window.clearInterval(pollTimer)
  if (searchTimer) window.clearTimeout(searchTimer)
})

watch(clusterId, (id) => {
  if (!id) return
  page.value = 1
  if (view.value === 'aliases') loadAliases()
  else load()
})

// 切换视图时按需加载（切回索引视图会重新走一遍后端过滤，条件保持不变）
watch(view, (v) => {
  if (v === 'aliases') loadAliases()
  else load()
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

const freshnessText = computed(() => formatAge(updatedAt.value, now.value))

// fmtRate 渲染每秒速率。没有足够样本时后端不给这个字段，显示 "—" 而不是编一个 0。
const fmtRate = (value?: number) => {
  if (value === undefined || value === null || !Number.isFinite(value)) return '—'
  if (value === 0) return '0'
  if (value >= 1000000) return `${(value / 1000000).toFixed(1)}M`
  if (value >= 1000) return `${(value / 1000).toFixed(1)}k`
  if (value >= 10) return String(Math.round(value))
  return value.toFixed(1)
}

// 速率是就地差分出来的派生值，窗口多长必须说清楚（见 PRD §11）
const rateHint = computed(() => {
  if (!rateWindowMs.value) {
    return '尚未积累到足够的样本，速率需要至少两次采集才能算出（约 5 秒后出现）。'
  }
  return `速率 = 累计计数之差 / 窗口长度，当前窗口约 ${Math.round(rateWindowMs.value / 1000)} 秒（最长 60 秒）。样本不足的索引显示为 —。`
})

// 空列表的原因不同，提示也不同：别让「被隐藏掉了」读成「这个集群没有索引」
const emptyText = computed(() => {
  const filtered = !!search.value || !!health.value || !!status.value
  if (total.value === 0 && !filtered) {
    return hideSystem.value ? '没有匹配的索引（. 开头的索引已隐藏）。' : '该集群暂无索引。'
  }
  return '没有匹配的索引。'
})

// ---------- 别名展示辅助 ----------
const filteredAliases = computed(() => {
  const q = aliasSearch.value.trim().toLowerCase()
  if (!q) return aliases.value
  return aliases.value.filter(a =>
    (a.alias || '').toLowerCase().includes(q) || (a.index || '').toLowerCase().includes(q))
})

// 一个别名可指向多个索引，所以三个数各不相同：别名数 / 覆盖索引数 / 指向对数
const aliasStats = computed(() => {
  const names = new Set<string>()
  const indices = new Set<string>()
  for (const a of aliases.value) {
    if (a.alias) names.add(a.alias)
    if (a.index) indices.add(a.index)
  }
  return { aliases: names.size, indices: indices.size, pairs: aliases.value.length }
})

const isWriteAlias = (a: EsAlias) => (a.is_write_index || '').toLowerCase() === 'true'

// _cat/aliases 用 "-" 表示该可选列无值
const cellOrDash = (value?: string) => (!value || value === '-' ? '-' : value)

const routingText = (a: EsAlias) => {
  const parts: string[] = []
  const indexRouting = cellOrDash(a['routing.index'])
  const searchRouting = cellOrDash(a['routing.search'])
  if (indexRouting !== '-') parts.push(`写 ${indexRouting}`)
  if (searchRouting !== '-') parts.push(`读 ${searchRouting}`)
  return parts.join(' · ') || '-'
}
</script>

<template>
  <div class="flex flex-col gap-4">
    <!-- 页头 -->
    <div class="flex items-center gap-2.5 flex-wrap">
      <div class="flex-1 min-w-0">
        <div class="flex items-center gap-2.5">
          <div class="text-18px font-600 tracking--0.4px">索引</div>
          <div class="flex items-center gap-0.5 p-0.5 rounded-7px bg-bg-3">
            <button
              v-for="tab in VIEW_TABS"
              :key="tab.value"
              class="px-2.5 py-0.5 rounded-5px text-12px font-500 border-none bg-transparent cursor-pointer transition-all"
              :class="view === tab.value ? 'bg-bg-2 text-accent' : 'text-text-3 hover:text-text'"
              @click="view = tab.value"
            >{{ tab.label }}</button>
          </div>
        </div>
        <div class="text-12px font-mono text-text-3 truncate mt-0.5">
          {{ cluster?.name || '集群' }} · {{ cluster?.hosts?.join(', ') || '-' }}
        </div>
      </div>
      <div class="text-11.5px text-text-3">{{ freshnessText }}</div>
      <n-button size="small" :loading="refreshing" @click="refresh">刷新</n-button>
    </div>

    <div v-if="error" class="border border-red-border bg-red-bg rounded-10px px-4 py-3 text-12.5px color-red font-mono">
      {{ error }}
    </div>

    <!-- 索引视图 -->
    <template v-if="view === 'indices'">
    <!-- 工具栏 -->
    <div class="flex items-center gap-2.5 flex-wrap">
      <n-input v-model:value="search" size="small" clearable placeholder="过滤索引名" class="w-64" />
      <n-select v-model:value="health" size="small" :options="healthOptions" class="w-36" />
      <n-select v-model:value="status" size="small" :options="statusOptions" class="w-32" />
      <n-checkbox v-model:checked="hideSystem">隐藏 . 开头的索引</n-checkbox>
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
      {{ emptyText }}
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
              <th class="px-3 py-2.5 font-500 text-right">
                <n-tooltip trigger="hover">
                  <template #trigger>
                    <span class="border-b border-dashed border-current cursor-help">写入/s</span>
                  </template>
                  {{ rateHint }}
                </n-tooltip>
              </th>
              <th class="px-4 py-2.5 font-500 text-right">
                <n-tooltip trigger="hover">
                  <template #trigger>
                    <span class="border-b border-dashed border-current cursor-help">查询/s</span>
                  </template>
                  {{ rateHint }}
                </n-tooltip>
              </th>
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
              <td class="px-3 py-2 font-mono text-right" :class="idx.index_rate === undefined ? 'text-text-3' : ''">{{ fmtRate(idx.index_rate) }}</td>
              <td class="px-4 py-2 font-mono text-right text-text-2">{{ fmtRate(idx.search_rate) }}</td>
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
    </template>

    <!-- 别名视图：与索引同一页，切视图即可，不另开栏目（见 PRD §2.4） -->
    <template v-else>
      <div class="flex items-center gap-2.5 flex-wrap">
        <n-input
          v-model:value="aliasSearch"
          size="small"
          clearable
          placeholder="过滤别名 / 索引名"
          class="w-64"
        />
        <div class="ml-auto text-11.5px text-text-3">
          {{ aliasStats.aliases }} 个别名 · 覆盖 {{ aliasStats.indices }} 个索引 · {{ aliasStats.pairs }} 条映射
        </div>
      </div>

      <div
        v-if="loading && aliases.length === 0"
        class="border border-border rounded-10px bg-bg-2 p-6 text-13px text-text-2"
      >
        加载中…
      </div>
      <div
        v-else-if="aliases.length === 0"
        class="border border-border rounded-10px bg-bg-2 p-6 text-13px text-text-2"
      >
        该集群暂无别名。
      </div>
      <div
        v-else-if="filteredAliases.length === 0"
        class="border border-border rounded-10px bg-bg-2 p-6 text-13px text-text-2"
      >
        没有匹配的别名。
      </div>
      <div
        v-else
        class="border border-border rounded-10px bg-bg-2 overflow-hidden"
      >
        <div class="overflow-x-auto">
          <table class="w-full text-12.5px">
            <thead>
              <tr class="text-text-3 text-left">
                <th class="px-4 py-2.5 font-500">别名</th>
                <th class="px-3 py-2.5 font-500">索引</th>
                <th class="px-3 py-2.5 font-500">写别名</th>
                <th class="px-3 py-2.5 font-500">Filter</th>
                <th class="px-4 py-2.5 font-500">Routing</th>
              </tr>
            </thead>
            <tbody>
              <tr
                v-for="(a, i) in filteredAliases"
                :key="`${a.alias}-${a.index}-${i}`"
                class="border-t border-border hover:bg-bg-3"
              >
                <td
                  class="px-4 py-2 font-mono max-w-60 truncate"
                  :title="a.alias"
                >{{ a.alias }}</td>
                <td class="px-3 py-2 font-mono">
                  <button
                    class="border-none bg-transparent p-0 font-mono text-12.5px text-accent cursor-pointer hover:underline"
                    :title="`在索引视图中查看 ${a.index}`"
                    @click="showIndex(a.index)"
                  >{{ a.index }}</button>
                </td>
                <td class="px-3 py-2">
                  <span
                    v-if="isWriteAlias(a)"
                    class="inline-flex items-center px-1.5 py-0.5 rounded-4px border text-11px font-600 border-accent-glow bg-accent-glow text-accent"
                  >WRITE</span>
                  <span
                    v-else
                    class="text-text-3"
                  >-</span>
                </td>
                <td
                  class="px-3 py-2 font-mono text-text-2 max-w-80 truncate"
                  :title="a.filter"
                >{{ cellOrDash(a.filter) }}</td>
                <td class="px-4 py-2 font-mono text-text-2">{{ routingText(a) }}</td>
              </tr>
            </tbody>
          </table>
        </div>
        <div class="border-t border-border px-4 py-2.5 text-11.5px text-text-3">
          别名数量有上界，一次展示全部（{{ filteredAliases.length }} / {{ aliases.length }} 行），不分页
        </div>
      </div>
    </template>
  </div>
</template>
