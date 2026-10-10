<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { useClusterStore } from '../stores/cluster'
import { overviewApi, type Overview } from '../api/overview'
import { allocationApi, catalogApi, type AllocationEnable, type EsIndex, type EsShard } from '../api/catalog'
import { useTheme } from '../composables/useTheme'
import { useLocale } from '../composables/useLocale'
import { formatAge } from '../utils/freshness'
import { fmtBytes, fmtInt } from '../utils/format'
import ClusterSwitcher from '../components/ClusterSwitcher.vue'
import WorkbenchGrid from '../components/workbench/WorkbenchGrid.vue'
import WorkbenchConsole from '../components/workbench/WorkbenchConsole.vue'
import type {
  HealthKey,
  SortKey,
  SortOrder,
  ViewMode,
  WbGroup,
  WbIndexCol,
  WbIndexRow,
  WbNodeCol,
  WbNodeRow,
} from '../components/workbench/types'
// v5 工作台样式：作用域限定在 .esp-wb，与现有 UnoCSS 页面互不影响
import '../styles/workbench-v5.less'

const route = useRoute()
const { t } = useI18n()
const clusterStore = useClusterStore()
const { isDark, toggleTheme } = useTheme()
const { toggleLocale } = useLocale()

const clusterId = computed(() => route.params.id as string)

// ---------- 数据 ----------
const overview = ref<Overview | null>(null)
const indices = ref<EsIndex[]>([])
const shards = ref<EsShard[]>([])
const rateWindowMs = ref(0)
const updatedAt = ref('')
const loading = ref(false)
const refreshing = ref(false)
const error = ref('')

// 后端 indices 分页上限 200；工作台需要整表（内存过滤 / 排序 / 分组），逐页取全量
const INDICES_PAGE_SIZE = 200
// 抓取上限：必须覆盖整表，否则高写入的索引可能排在后面而根本没被拉进来（表现为「排序漏掉第一名」）。
// 500 页 × 200 = 10 万个索引，足够覆盖现实规模，同时避免极端情况下请求失控。
const MAX_INDICES_PAGES = 500
const NODE_PAGE_SIZE = 30
// 索引视角分页：行数直接决定 DOM 规模，用分页把它钉死在每页固定行数
const INDEX_PAGE_SIZES = [50, 100, 200]
const INDEX_DEFAULT_PAGE_SIZE = 100

async function loadIndices(force: boolean) {
  const id = clusterId.value
  if (!id) return
  const first = await catalogApi.indices(id, { page: 1, pageSize: INDICES_PAGE_SIZE }, force)
  rateWindowMs.value = first.rate_window_ms ?? 0
  const all = [...(first.data ?? [])]
  const pages = Math.min(Math.max(1, Math.ceil((first.total || 0) / INDICES_PAGE_SIZE)), MAX_INDICES_PAGES)
  if (pages > 1) {
    const rest = await Promise.all(
      Array.from({ length: pages - 1 }, (_, i) =>
        catalogApi.indices(id, { page: i + 2, pageSize: INDICES_PAGE_SIZE }, false),
      ),
    )
    for (const res of rest) all.push(...(res.data ?? []))
  }
  indices.value = all
}

async function loadAll(force = false, silent = false) {
  const id = clusterId.value
  if (!id) return
  if (!silent) loading.value = true
  try {
    const [ov, shardRes] = await Promise.all([
      overviewApi.get(id, force),
      catalogApi.shards(id, force),
    ])
    overview.value = ov
    shards.value = shardRes.data ?? []
    updatedAt.value = ov.updated_at || shardRes.updated_at || ''
    await loadIndices(force)
    error.value = ''
  } catch (e) {
    error.value = e instanceof Error ? e.message : t('common.error')
  } finally {
    loading.value = false
  }
}

async function refresh() {
  if (refreshing.value) return
  refreshing.value = true
  try {
    await loadAll(true)
    await loadAllocation()
  } finally {
    refreshing.value = false
  }
}

// ---------- 分片分配开关 ----------
const ALLOC_OPTIONS: AllocationEnable[] = ['all', 'primaries', 'new_primaries', 'none']
const allocLabels: Record<AllocationEnable, string> = {
  all: 'alloc.all',
  primaries: 'alloc.primaries',
  new_primaries: 'alloc.newPrimaries',
  none: 'alloc.noneValue',
}
const allocation = ref<AllocationEnable>('all')
const allocationSaving = ref(false)

async function loadAllocation() {
  const id = clusterId.value
  if (!id) return
  try {
    allocation.value = await allocationApi.get(id)
  } catch {
    // 读取失败保持 all，不打断页面
  }
}

async function changeAllocation(value: string) {
  const next = value as AllocationEnable
  if (next === allocation.value) return
  const previous = allocation.value
  allocation.value = next // 乐观更新，失败回滚
  allocationSaving.value = true
  try {
    await allocationApi.set(clusterId.value, next)
  } catch {
    allocation.value = previous
  } finally {
    allocationSaving.value = false
  }
}

// ---------- 视图状态 ----------
const view = ref<ViewMode>('index')
const search = ref('')
const hideSystem = ref(true)
const onlyProblems = ref(false)
const grouped = ref(false)
const expanded = ref<string[]>([])
// 默认按「写入/s」降序（最忙的索引在前），而不是按索引名
const sortKey = ref<SortKey>('write')
const sortOrder = ref<SortOrder>('desc')
const nodePage = ref(1)
const indexPage = ref(1)
const indexPageSize = ref(INDEX_DEFAULT_PAGE_SIZE)

function setView(next: ViewMode) {
  view.value = next
}

function toggleHideSystem() {
  hideSystem.value = !hideSystem.value
}

function toggleOnlyProblems() {
  onlyProblems.value = !onlyProblems.value
  // 「只看有问题的」与「隐藏系统索引」互斥：有问题往往是系统索引，藏起来就白开了
  if (onlyProblems.value) hideSystem.value = false
}

function toggleGroup() {
  grouped.value = !grouped.value
}

function setSort(key: SortKey) {
  if (sortKey.value === key) {
    sortOrder.value = sortOrder.value === 'asc' ? 'desc' : 'asc'
  } else {
    sortKey.value = key
    sortOrder.value = key === 'index' ? 'asc' : 'desc'
  }
}

function toggleGroupRow(prefix: string) {
  expanded.value = expanded.value.includes(prefix)
    ? expanded.value.filter(p => p !== prefix)
    : [...expanded.value, prefix]
}

function filterToIndex(name: string) {
  search.value = name
  hideSystem.value = false
  view.value = 'index'
}

function changeNodePage(delta: number) {
  nodePage.value = Math.min(Math.max(1, nodePage.value + delta), nodePages.value)
}

function changeIndexPage(delta: number) {
  indexPage.value = Math.min(Math.max(1, currentIndexPage.value + delta), indexPages.value)
}

// 过滤 / 排序 / 视图 / 页大小变化时回到第一页
watch([search, hideSystem, onlyProblems, sortKey, sortOrder, indexPageSize, view], () => {
  nodePage.value = 1
  indexPage.value = 1
})

// ---------- 顶栏模式：工作台 / 开发控制台 ----------
// 控制台重（Monaco），首次切入才挂载；之后保持挂载、只切显隐，避免来回切丢状态与重复初始化。
// 两侧都不销毁：工作台用 visibility 隐藏（保留网格布局与滚动位置），控制台以覆盖层叠在顶栏之下。
const mode = ref<'workbench' | 'console'>('workbench')
const consoleMounted = ref(false)
function openConsole() {
  consoleMounted.value = true
  mode.value = 'console'
}

// ---------- 整形：索引 / 分片映射 ----------
const shardMaps = computed(() => {
  const byIndexNode = new Map<string, EsShard[]>()
  const byIndex = new Map<string, EsShard[]>()
  for (const s of shards.value) {
    const key = s.index + '\u0000' + s.node
    const a = byIndexNode.get(key)
    if (a) a.push(s)
    else byIndexNode.set(key, [s])
    const b = byIndex.get(s.index)
    if (b) b.push(s)
    else byIndex.set(s.index, [s])
  }
  return { byIndexNode, byIndex }
})

// 前缀归并：去掉结尾的日期 / 序号段（logs-2024.01.01 → logs，metrics-000012 → metrics）
function familyOf(name: string): string {
  const stripped = name.replace(/[-_.]?\d[\d._-]*$/, '')
  return stripped || name
}

const filteredIndices = computed<EsIndex[]>(() => {
  const q = search.value.trim().toLowerCase()
  const list = indices.value.filter(i =>
    (!hideSystem.value || !i.index.startsWith('.')) &&
    (!onlyProblems.value || (i.health || '').toLowerCase() !== 'green') &&
    (!q || i.index.toLowerCase().includes(q)),
  )
  const dir = sortOrder.value === 'asc' ? 1 : -1
  const valueOf = (i: EsIndex): number | string => {
    switch (sortKey.value) {
      case 'health':
        return { red: 0, yellow: 1, green: 2 }[(i.health || '').toLowerCase()] ?? 3
      case 'docs':
        return Number(i['docs.count']) || 0
      case 'store':
        return Number(i['store.size']) || 0
      case 'write':
        return i.index_rate ?? 0
      case 'read':
        return i.search_rate ?? 0
      default:
        return i.index
    }
  }
  return [...list].sort((a, b) => {
    // 纯按所选指标排序（与 Indices 页一致）；不再叠加「非 Green 优先」，否则按写入排序时第一名会被顶下去
    const va = valueOf(a)
    const vb = valueOf(b)
    if (va < vb) return -dir
    if (va > vb) return dir
    return 0
  })
})

// ---------- 索引视角 ----------
const nodeColumns = computed<WbNodeCol[]>(() =>
  (overview.value?.nodes ?? []).map(n => ({
    name: n.name,
    master: n.master === '*',
    heap: n.heap_percent,
    cpu: n.cpu,
    disk: n.disk_used_percent,
  })),
)

// 当前页切片：分页后每页渲染固定行数，DOM 规模与集群大小解耦
const indexPages = computed(() => Math.max(1, Math.ceil(filteredIndices.value.length / indexPageSize.value)))
const currentIndexPage = computed(() => Math.min(indexPage.value, indexPages.value))
const pagedIndices = computed(() => {
  const start = (currentIndexPage.value - 1) * indexPageSize.value
  return filteredIndices.value.slice(start, start + indexPageSize.value)
})

const indexRows = computed<WbIndexRow[]>(() =>
  pagedIndices.value.map(index => ({
    index,
    cells: nodeColumns.value.map(nc => shardMaps.value.byIndexNode.get(index.index + '\u0000' + nc.name) ?? null),
    unassigned: (shardMaps.value.byIndex.get(index.index) ?? []).filter(
      s => (s.state || '').toUpperCase() === 'UNASSIGNED',
    ),
  })),
)

const groups = computed<WbGroup[]>(() => {
  const map = new Map<string, WbIndexRow[]>()
  for (const row of indexRows.value) {
    const fam = familyOf(row.index.index)
    const arr = map.get(fam)
    if (arr) arr.push(row)
    else map.set(fam, [row])
  }
  return [...map.entries()].map(([prefix, rows]) => {
    const worst: HealthKey = rows.some(r => (r.index.health || '').toLowerCase() === 'red')
      ? 'red'
      : rows.some(r => (r.index.health || '').toLowerCase() === 'yellow')
        ? 'yellow'
        : 'green'
    return {
      prefix,
      rows,
      worst,
      pri: rows.reduce((a, r) => a + (Number(r.index.pri) || 0), 0),
      rep: rows.reduce((a, r) => a + (Number(r.index.rep) || 0), 0),
      docs: rows.reduce((a, r) => a + (Number(r.index['docs.count']) || 0), 0),
      store: rows.reduce((a, r) => a + (Number(r.index['store.size']) || 0), 0),
    }
  })
})

// ---------- 节点视角 ----------
const nodePages = computed(() => Math.max(1, Math.ceil(filteredIndices.value.length / NODE_PAGE_SIZE)))
const indexColumns = computed<WbIndexCol[]>(() => {
  const page = Math.min(nodePage.value, nodePages.value)
  const start = (page - 1) * NODE_PAGE_SIZE
  return filteredIndices.value.slice(start, start + NODE_PAGE_SIZE).map(i => ({ name: i.index, health: i.health }))
})

const ROLE_CODES = ['c', 'd', 'f', 'h', 'i', 'l', 'm', 'r', 's', 't', 'v', 'w']
function formatRole(role: string): string {
  if (!role) return '-'
  if (role === '-') return t('role.coord')
  const labels = Array.from(role, code => (ROLE_CODES.includes(code) ? t(`role.${code}`) : null))
  return labels.every(Boolean) ? (labels as string[]).join(' · ') : role
}

const nodeRows = computed<WbNodeRow[]>(() =>
  (overview.value?.nodes ?? []).map(n => ({
    name: n.name,
    role: formatRole(n.role),
    master: n.master === '*',
    heap: n.heap_percent,
    cpu: n.cpu,
    disk: n.disk_used_percent,
    load1: n.load_1m,
    load5: n.load_5m,
    load15: n.load_15m,
    shards: n.shards ?? null,
    cells: indexColumns.value.map(c => shardMaps.value.byIndexNode.get(c.name + '\u0000' + n.name) ?? null),
  })),
)

// ---------- 指标条 ----------
const healthKey = computed<HealthKey>(() => {
  const s = (overview.value?.health.status || '').toLowerCase()
  return s === 'green' || s === 'yellow' || s === 'red' ? s : 'green'
})

const healthCounts = computed(() => {
  const hc = { green: 0, yellow: 0, red: 0 }
  for (const i of indices.value) {
    const k = (i.health || '').toLowerCase()
    if (k === 'green' || k === 'yellow' || k === 'red') hc[k]++
  }
  return hc
})

const donutStyle = computed(() => {
  const total = healthCounts.value.green + healthCounts.value.yellow + healthCounts.value.red || 1
  const g = (healthCounts.value.green / total) * 100
  const gy = ((healthCounts.value.green + healthCounts.value.yellow) / total) * 100
  return { background: `conic-gradient(var(--green) 0 ${g}%, var(--yellow) ${g}% ${gy}%, var(--red) ${gy}% 100%)` }
})

// 规模指标统一取后端聚合值（与 Overview 页同源）：
// 前端自行对分页索引列表做 reduce 会在索引数超过单次拉取上限时少算，必须用后端整表聚合值。
const counts = computed(() => {
  const c = overview.value?.counts
  return {
    nodes: c?.nodes ?? 0,
    indices: c?.indices ?? 0,
    pri: c?.primary_shards ?? 0,
    shards: c?.total_shards ?? 0,
    docs: c?.docs ?? 0,
    store: c?.store_bytes ?? 0,
  }
})

// 分片构成：按状态分色，实心=主分片 / 半透明=副本分片（与网格分片方块同源编码）
function shardStateKey(state: string): 'started' | 'relocating' | 'initializing' | 'unassigned' {
  const k = (state || '').toUpperCase()
  if (k === 'RELOCATING') return 'relocating'
  if (k === 'INITIALIZING') return 'initializing'
  if (k === 'UNASSIGNED') return 'unassigned'
  return 'started'
}

const shardComposition = computed(() => {
  const st = {
    started: { p: 0, r: 0 },
    relocating: { p: 0, r: 0 },
    initializing: { p: 0, r: 0 },
    unassigned: { p: 0, r: 0 },
  }
  for (const s of shards.value) {
    const slot = st[shardStateKey(s.state)]
    if (s.prirep === 'p') slot.p++
    else slot.r++
  }
  return st
})

const shardTotals = computed(() => {
  const c = shardComposition.value
  const groups = [c.started, c.relocating, c.initializing, c.unassigned]
  return {
    byState: {
      started: c.started.p + c.started.r,
      relocating: c.relocating.p + c.relocating.r,
      initializing: c.initializing.p + c.initializing.r,
      unassigned: c.unassigned.p + c.unassigned.r,
    },
    pri: groups.reduce((a, o) => a + o.p, 0),
    rep: groups.reduce((a, o) => a + o.r, 0),
  }
})

const shardTotal = computed(() => {
  const s = shardTotals.value.byState
  return s.started + s.relocating + s.initializing + s.unassigned
})

// 每个「状态 × 主/副」的分段底色：主 = 实色，副 = 同色降透明度（未分配用斜纹）
const SHARD_STATE_SEG: Record<string, { p: Record<string, string>; r: Record<string, string> }> = {
  started: { p: { background: 'var(--green)' }, r: { background: 'var(--green)', opacity: '0.42' } },
  relocating: { p: { background: 'var(--yellow)' }, r: { background: 'var(--yellow)', opacity: '0.42' } },
  initializing: { p: { background: 'var(--blue)' }, r: { background: 'var(--blue)', opacity: '0.42' } },
  unassigned: {
    p: { background: 'repeating-linear-gradient(45deg, var(--red), var(--red) 2px, transparent 2px, transparent 4px)' },
    r: { background: 'repeating-linear-gradient(45deg, rgba(255,107,107,.42), rgba(255,107,107,.42) 2px, transparent 2px, transparent 4px)' },
  },
}

const shardBarSegments = computed(() => {
  const c = shardComposition.value
  const total = shardTotal.value || 1
  const out: Record<string, string>[] = []
  for (const state of ['started', 'relocating', 'initializing', 'unassigned'] as const) {
    for (const kind of ['p', 'r'] as const) {
      const n = c[state][kind]
      if (!n) continue
      out.push({ ...SHARD_STATE_SEG[state][kind], width: `${((n / total) * 100).toFixed(3)}%` })
    }
  }
  return out
})

// ---------- 活动区 ----------
const relocatingShards = computed(() =>
  shards.value.filter(s => {
    const k = (s.state || '').toUpperCase()
    return k === 'RELOCATING' || k === 'INITIALIZING'
  }),
)

const unassignedList = computed(() => overview.value?.problems?.unassigned_shards ?? [])
const unhealthyList = computed(() => overview.value?.problems?.unhealthy_indices ?? [])
const unassignedTotal = computed(() => overview.value?.problems?.unassigned_shards_total ?? shardTotals.value.byState.unassigned)
const unhealthyTotal = computed(() => overview.value?.problems?.unhealthy_indices_total ?? 0)

const bandActive = computed(() =>
  relocatingShards.value.length > 0 || unassignedTotal.value > 0 || unhealthyTotal.value > 0,
)

// 顶栏「健康 / 恢复」：默认跟随真实状态；用户点击后手动指定活动区展示模式。
// 强制「健康」时不隐藏问题，只把活动区收成一行摘要。
const bandForce = ref<'auto' | 'healthy' | 'recovering'>('auto')
const bandShowActive = computed(() =>
  bandForce.value === 'auto' ? bandActive.value : bandForce.value === 'recovering',
)
const bandCollapsedWithActivity = computed(() => !bandShowActive.value && bandActive.value)

// ---------- 新鲜度 ----------
const now = ref(Date.now())
let ticker: number | undefined
let pollTimer: number | undefined
const freshnessText = computed(() => formatAge(updatedAt.value, now.value, t('freshness.updated')))

const unreachable = computed(() => !!overview.value && overview.value.reachable === false)

// ---------- 状态栏 ----------
const statusCount = computed(() =>
  `${t('status.totalIndices', { total: indices.value.length })} · ${t('status.showing', { n: indexRows.value.length })}`,
)

const statusHint = computed(() =>
  view.value === 'node' ? t('status.hintNode') : t('status.hintIndex'),
)

const statusSum = computed(() => {
  const store = indexRows.value.reduce((a, r) => a + (Number(r.index['store.size']) || 0), 0)
  const window = rateWindowMs.value ? t('status.rateWindow', { s: Math.round(rateWindowMs.value / 1000) }) : ''
  return `${t('status.matchedStore', { size: fmtBytes(store) })} · ${window || t('status.collectionCycle')}`
})

// ---------- 生命周期 ----------
onMounted(async () => {
  ticker = window.setInterval(() => { now.value = Date.now() }, 5000)
  try {
    await clusterStore.fetchClusters()
  } catch {
    // 集群列表拉取失败不阻塞工作台（下方会显示不可达 / 空态）
  }
  await loadAll()
  await loadAllocation()
  pollTimer = window.setInterval(() => loadAll(true, true), 15000)
})

onBeforeUnmount(() => {
  if (ticker) window.clearInterval(ticker)
  if (pollTimer) window.clearInterval(pollTimer)
})

watch(clusterId, (id) => {
  if (!id) return
  nodePage.value = 1
  indexPage.value = 1
  loadAll()
  loadAllocation()
})
</script>

<template>
  <div class="esp-wb" :class="{ 'mode-console': mode === 'console' }">
    <!-- ===== top bar ===== -->
    <header class="topbar">
      <div class="brand">
        <div class="mark">
          <svg width="13" height="13" viewBox="0 0 14 14" fill="none">
            <circle cx="7" cy="7" r="3" fill="#fff" opacity=".9" />
            <circle cx="7" cy="7" r="6" stroke="#fff" stroke-width="1" opacity=".45" />
          </svg>
        </div>
        <span>{{ t('common.appName') }}</span>
      </div>
      <div class="vsep"></div>
      <ClusterSwitcher />
      <div class="vsep"></div>
      <div class="alloc" :class="{ warn: allocation !== 'all' }" :title="t('alloc.hint')">
        <span class="alloc-lbl">{{ t('alloc.label') }}</span>
        <select :value="allocation" :disabled="allocationSaving" @change="changeAllocation(($event.target as HTMLSelectElement).value)">
          <option v-for="opt in ALLOC_OPTIONS" :key="opt" :value="opt">{{ t(allocLabels[opt]) }}</option>
        </select>
      </div>
      <div class="vsep"></div>
      <nav class="tabs">
        <span class="tab" :class="{ on: mode === 'workbench' }" @click="mode = 'workbench'">{{ t('common.workbench') }}</span>
        <span class="tab" :class="{ on: mode === 'console' }" @click="openConsole">{{ t('common.devConsole') }}</span>
      </nav>
      <div class="grow"></div>
      <div class="stateSw" :title="t('stateSw.title')">
        <div class="o" :class="{ on: !bandShowActive }" @click="bandForce = 'healthy'">
          <span class="d" style="color: var(--green)"></span>{{ t('stateSw.healthy') }}
        </div>
        <div class="o" :class="{ on: bandShowActive }" @click="bandForce = 'recovering'">
          <span class="d" style="color: var(--yellow)"></span>{{ t('stateSw.recovering') }}
        </div>
      </div>
      <div class="fresh">
        <span class="live" :class="{ act: bandActive }"></span>
        <span v-if="bandActive">{{ t('freshness.recoveringSuffix') }}</span>{{ freshnessText }}
      </div>
      <div class="vsep"></div>
      <button class="iconbtn" :title="t('common.refresh')" :disabled="refreshing" @click="refresh">
        <div v-if="refreshing" class="i-lucide-loader-circle animate-spin" style="width: 14px; height: 14px"></div>
        <svg v-else width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.9" stroke-linecap="round" stroke-linejoin="round">
          <polyline points="23 4 23 10 17 10" />
          <path d="M20.49 15a9 9 0 1 1-2.12-9.36L23 10" />
        </svg>
      </button>
      <button class="iconbtn" :title="t('common.language')" @click="toggleLocale">
        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.9" stroke-linecap="round" stroke-linejoin="round">
          <circle cx="12" cy="12" r="9" />
          <path d="M3 12h18" />
          <path d="M12 3a14 14 0 0 1 0 18a14 14 0 0 1 0-18z" />
        </svg>
      </button>
      <button class="iconbtn ph" :title="t('common.theme')">
        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.9" stroke-linecap="round" stroke-linejoin="round">
          <path d="M12 22a10 10 0 1 1 0-20c5.5 0 10 4 10 9 0 2.5-2 4.5-4.5 4.5H15a2 2 0 0 0-1.4 3.4A2 2 0 0 1 12 22z" />
          <circle cx="7.5" cy="10.5" r="1.2" />
          <circle cx="12" cy="7.5" r="1.2" />
          <circle cx="16.5" cy="10.5" r="1.2" />
        </svg>
      </button>
      <button class="iconbtn" :title="isDark ? t('common.toLight') : t('common.toDark')" @click="toggleTheme">
        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.9" stroke-linecap="round" stroke-linejoin="round">
          <circle cx="12" cy="12" r="5" />
          <line x1="12" y1="1" x2="12" y2="3" />
          <line x1="12" y1="21" x2="12" y2="23" />
          <line x1="4.2" y1="4.2" x2="5.6" y2="5.6" />
          <line x1="18.4" y1="18.4" x2="19.8" y2="19.8" />
          <line x1="1" y1="12" x2="3" y2="12" />
          <line x1="21" y1="12" x2="23" y2="12" />
          <line x1="4.2" y1="19.8" x2="5.6" y2="18.4" />
          <line x1="18.4" y1="5.6" x2="19.8" y2="4.2" />
        </svg>
      </button>
    </header>

    <!-- ===== dev console（顶栏模式切换；首次切入才挂载，之后保活） ===== -->
    <div v-if="consoleMounted" class="consolewrap">
      <WorkbenchConsole :cluster-id="clusterId" />
    </div>

    <!-- ===== vitals ===== -->
    <div class="vitals">
      <!-- 健康：状态字 + 索引健康环形 + active 占比（不再单画进度条，避免与环形重复） -->
      <div class="vgrp">
        <span class="hchip" :style="{ color: `var(--${healthKey})`, background: `var(--${healthKey}-bg)`, border: `1px solid var(--${healthKey}-bd)` }">
          <span class="pip"></span>{{ healthKey.toUpperCase() }}
        </span>
        <div class="donut sm" :style="donutStyle" :title="t('vitals.healthTitle')"></div>
        <div class="vstat">
          <span class="k">{{ t('vitals.activeShards') }}</span>
          <span class="v">{{ (overview?.health.active_shards_percent ?? 0).toFixed(1) }}<small>%</small></span>
        </div>
      </div>

      <!-- 规模：数字等宽均分 -->
      <div class="vgrp vgfill">
        <div class="vstat"><span class="k">{{ t('vitals.nodes') }}</span><span class="v">{{ counts.nodes }}</span></div>
        <div class="vstat"><span class="k">{{ t('vitals.indices') }}</span><span class="v">{{ fmtInt(counts.indices) }}</span></div>
        <div class="vstat"><span class="k">{{ t('vitals.primaryShards') }}</span><span class="v">{{ fmtInt(counts.pri) }}</span></div>
        <div class="vstat"><span class="k">{{ t('vitals.totalShards') }}</span><span class="v">{{ fmtInt(counts.shards) }}</span></div>
        <div class="vstat"><span class="k">{{ t('vitals.docs') }}</span><span class="v">{{ fmtInt(counts.docs) }}</span></div>
        <div class="vstat"><span class="k">{{ t('vitals.store') }}</span><span class="v">{{ fmtBytes(counts.store) }}</span></div>
      </div>

      <!-- 分片构成：一条堆叠条（按状态分色，实心=主 / 半透明=副本），图例即数字 -->
      <div class="vgrp vgcol vggrow" style="min-width: 280px">
        <div class="vstat"><span class="k">{{ t('vitals.shardComposition') }}</span><span class="v">{{ fmtInt(shardTotal) }}</span></div>
        <div class="stack"><i v-for="(s, i) in shardBarSegments" :key="i" :style="s"></i></div>
        <div class="stackleg">
          <span><i style="background: var(--green)"></i>{{ t('toolbar.legendPri') }} <b>{{ fmtInt(shardTotals.pri) }}</b></span>
          <span><i style="background: var(--green); opacity: .42"></i>{{ t('toolbar.legendRep') }} <b>{{ fmtInt(shardTotals.rep) }}</b></span>
          <span><i style="background: var(--yellow)"></i>{{ t('toolbar.legendRelo') }} <b>{{ shardTotals.byState.relocating }}</b></span>
          <span><i style="background: var(--blue)"></i>{{ t('toolbar.legendInit') }} <b>{{ shardTotals.byState.initializing }}</b></span>
          <span><i style="background: repeating-linear-gradient(45deg, var(--red), var(--red) 2px, transparent 2px, transparent 4px)"></i>{{ t('toolbar.legendUn') }} <b>{{ fmtInt(shardTotals.byState.unassigned) }}</b></span>
        </div>
      </div>

      <!-- 恢复：recovery API 未实现，留白 -->
      <div class="vgrp">
        <div class="vstat"><span class="k">{{ t('vitals.recoveryThroughput') }}</span><span class="v dim" :title="t('vitals.recoveryPending')">—</span></div>
        <div class="vstat"><span class="k">{{ t('vitals.remaining') }}</span><span class="v dim" :title="t('vitals.recoveryPending')">—</span></div>
      </div>
    </div>

    <!-- ===== 活动区 ===== -->
    <div class="band" :class="{ on: bandShowActive }">
      <!-- 空闲态：有活动但被强制收起时给出摘要，不隐藏问题 -->
      <div class="idle">
        <template v-if="unreachable || error">
          <span class="ok" style="color: var(--red)">
            {{ unreachable ? t('common.unreachable') : error }}
          </span>
        </template>
        <template v-else-if="bandCollapsedWithActivity">
          <span class="ok" style="color: var(--yellow)">
            {{ t('band.recovering') }} <b>{{ relocatingShards.length }}</b>
            · {{ t('band.unassigned') }} <b>{{ unassignedTotal }}</b>
            · {{ t('band.nonGreen') }} <b>{{ unhealthyTotal }}</b>
          </span>
        </template>
        <template v-else>
          <span class="ok">
            <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.6" stroke-linecap="round" stroke-linejoin="round"><polyline points="20 6 9 17 4 12" /></svg>
            {{ t('band.allOk') }}
          </span>
          <span class="sep">·</span><span>{{ t('band.noRecovery') }}</span>
          <span class="sep">·</span><span>{{ t('band.noUnassigned') }}</span>
          <span class="sep">·</span><span>{{ t('band.noUnhealthy') }}</span>
        </template>
        <span class="ts">{{ freshnessText }}</span>
      </div>

      <!-- 活动态 -->
      <div class="active">
        <div class="eb-head">
          <span class="ld"></span><span class="ttl">{{ t('band.active') }}</span>
          <span class="cnt">
            {{ t('band.recovering') }} <b>{{ relocatingShards.length }}</b>
            · {{ t('band.unassigned') }} <b>{{ unassignedTotal }}</b>
            · {{ t('band.nonGreen') }} <b>{{ unhealthyTotal }}</b>
          </span>
          <span class="sp">{{ freshnessText }}</span>
        </div>
        <div class="eb-cols">
          <section class="eb-sec">
            <h4>
              {{ t('band.shardRecovery') }} <span class="n">{{ relocatingShards.length }}</span>
              <span class="hint">{{ t('band.recoveryHint') }}</span>
            </h4>
            <table class="rt">
              <colgroup>
                <col style="width: 190px" /><col style="width: 60px" /><col style="width: 60px" />
                <col style="width: 110px" /><col style="width: 160px" />
              </colgroup>
              <thead>
                <tr>
                  <th>{{ t('grid.index') }}</th><th>{{ t('grid.priRep') }}</th>
                  <th>{{ t('grid.status') }}</th><th>{{ t('vitals.nodes') }}</th><th>{{ t('grid.reason') }}</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="(s, i) in relocatingShards.slice(0, 12)" :key="i">
                  <td class="ix" @click="filterToIndex(s.index)" :title="s.index">{{ s.index }}</td>
                  <td>{{ s.prirep === 'p' ? 'P' : 'R' }}{{ s.shard }}</td>
                  <td class="stage" :class="{ index: (s.state || '').toUpperCase() === 'RELOCATING' }">{{ (s.state || '').toUpperCase() }}</td>
                  <td><span class="dim">{{ s.node || '-' }}</span></td>
                  <td class="dim">{{ s['unassigned.reason'] || '-' }}</td>
                </tr>
                <tr v-if="relocatingShards.length === 0">
                  <td colspan="5" class="dim">{{ t('common.none') }}</td>
                </tr>
              </tbody>
            </table>
            <!-- recovery API 未实现：进度明细（文件 / 字节 / translog / 耗时）留白 -->
            <div class="done-strip">
              <span>{{ t('band.recoveryPlaceholder') }}</span>
            </div>
          </section>
          <section class="eb-sec">
            <h4>{{ t('band.problems') }} <span class="n">{{ unassignedTotal + unhealthyTotal }}</span><span class="hint">{{ t('band.problemsHint') }}</span></h4>
            <div class="psub">{{ t('band.unassignedShards') }} <span class="n">{{ unassignedTotal }}</span></div>
            <div class="plist">
              <div v-for="(s, i) in unassignedList.slice(0, 5)" :key="i" class="prow" @click="filterToIndex(s.index)">
                <span class="ix">{{ s.index }}</span><span>#{{ s.shard }}</span>
                <span class="rs">{{ s.reason || '-' }}</span>
              </div>
              <div v-if="unassignedList.length === 0" class="prow dim">{{ t('common.none') }}</div>
            </div>
            <div class="psub">{{ t('band.nonGreenIndices') }} <span class="n">{{ unhealthyTotal }}</span></div>
            <div class="plist">
              <div v-for="(idx, i) in unhealthyList.slice(0, 5)" :key="i" class="prow" @click="filterToIndex(idx.index)">
                <span class="hdot" :class="idx.health" style="width: 6px; height: 6px; border-radius: 50%; display: inline-block"></span>
                <span class="ix">{{ idx.index }}</span><span class="rs">{{ idx.health }}</span>
              </div>
              <div v-if="unhealthyList.length === 0" class="prow dim">{{ t('common.none') }}</div>
            </div>
          </section>
        </div>
      </div>
    </div>

    <!-- ===== toolbar ===== -->
    <div class="toolbar">
      <div class="seg">
        <span class="o" :class="{ on: view === 'index' }" @click="setView('index')">{{ t('toolbar.viewIndex') }}</span>
        <span class="o" :class="{ on: view === 'node' }" @click="setView('node')">{{ t('toolbar.viewNode') }}</span>
      </div>
      <div class="tsearch">
        <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" style="color: var(--text-3)">
          <circle cx="11" cy="11" r="8" /><line x1="21" y1="21" x2="16.65" y2="16.65" />
        </svg>
        <input v-model="search" :placeholder="t('toolbar.searchPlaceholder')" />
      </div>
      <button class="wchip" :class="{ on: hideSystem }" @click="toggleHideSystem">{{ t('toolbar.hideSystem') }}</button>
      <button class="wchip" :class="{ on: onlyProblems }" @click="toggleOnlyProblems">{{ t('toolbar.onlyProblems') }}</button>
      <button v-if="view === 'index'" class="wchip" :class="{ on: grouped }" @click="toggleGroup">{{ t('toolbar.groupByPrefix') }}</button>
      <div class="grow"></div>
      <div v-if="view === 'index'" class="legend">
        <span><span class="sq started"></span>{{ t('toolbar.legendPri') }}</span>
        <span><span class="sq rep"></span>{{ t('toolbar.legendRep') }}</span>
        <span><span class="sq relo"></span>{{ t('toolbar.legendRelo') }}</span>
        <span><span class="sq init"></span>{{ t('toolbar.legendInit') }}</span>
        <span><span class="sq un"></span>{{ t('toolbar.legendUn') }}</span>
      </div>
    </div>

    <!-- ===== 主网格 ===== -->
    <div class="gridwrap">
      <WorkbenchGrid
        :view="view"
        :node-columns="nodeColumns"
        :index-rows="indexRows"
        :grouped="grouped"
        :groups="groups"
        :expanded="expanded"
        :sort-key="sortKey"
        :sort-order="sortOrder"
        :index-columns="indexColumns"
        :node-rows="nodeRows"
        :node-page="Math.min(nodePage, nodePages)"
        :node-pages="nodePages"
        :node-total="filteredIndices.length"
        :node-page-size="NODE_PAGE_SIZE"
        @sort="setSort"
        @toggle-group-row="toggleGroupRow"
        @node-page="changeNodePage"
      />
      <div v-if="loading && indices.length === 0" class="dim" style="padding: 20px">{{ t('common.loading') }}</div>
    </div>

    <!-- ===== 索引分页 ===== -->
    <div v-if="view === 'index'" class="pager">
      <span>{{ t('grid.pageInfoShort', { page: currentIndexPage, pages: indexPages }) }}</span>
      <div class="grow"></div>
      <select :value="indexPageSize" @change="indexPageSize = Number(($event.target as HTMLSelectElement).value)">
        <option v-for="size in INDEX_PAGE_SIZES" :key="size" :value="size">{{ t('grid.perPage', { n: size }) }}</option>
      </select>
      <button :disabled="currentIndexPage <= 1" @click="changeIndexPage(-1)">{{ t('grid.prevPage') }}</button>
      <button :disabled="currentIndexPage >= indexPages" @click="changeIndexPage(1)">{{ t('grid.nextPage') }}</button>
    </div>

    <!-- ===== status line ===== -->
    <div class="statusline">
      <span>{{ statusCount }}</span>
      <span>{{ statusHint }}</span>
      <span class="sp">{{ statusSum }}</span>
    </div>
  </div>
</template>
