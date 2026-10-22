<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { useClusterStore } from '../stores/cluster'
import { overviewApi, type Overview } from '../api/overview'
import { allocationApi, catalogApi, type AllocationEnable, type EsIndex, type EsRecovery, type EsShard } from '../api/catalog'
import { useTheme } from '../composables/useTheme'
import { useLocale } from '../composables/useLocale'
import { ageParts, NEVER_COLLECTED } from '../utils/freshness'
import { fmtBytes, fmtDuration, fmtInt } from '../utils/format'
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
// 工作台样式：作用域限定在 .esp-wb，与 UnoCSS 页面互不影响
import '../styles/workbench.less'

const route = useRoute()
const router = useRouter()
const { t } = useI18n()
const clusterStore = useClusterStore()
const { isDark, toggleTheme } = useTheme()
const { toggleLocale } = useLocale()

const clusterId = computed(() => route.params.id as string)

// ---------- 数据 ----------
const overview = ref<Overview | null>(null)
const indices = ref<EsIndex[]>([])
const shards = ref<EsShard[]>([])
const recoveries = ref<EsRecovery[]>([])
const rateWindowMs = ref(0)
const updatedAt = ref('')
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
// 骨架屏占位节点列数：仅为撑出网格的列节奏，与实际节点数无关（此时还拿不到数据）
const SKEL_NODE_COLS = 4

// 加载「代际」：每次发起加载自增，只有最新一代的结果允许落盘。
// 切换集群 / 手动刷新 / 轮询都会开启新的一代，使在途的旧请求（典型是大集群 A 的慢响应）
// 无法覆盖已经切过去的 B 集群数据。
let loadSeq = 0

// 纯取数：不读 clusterId.value、不写状态，由 loadAll 校验代际后统一落盘。
// 否则切换发生在 await 期间时，会出现 overview=A / indices=B 的混合状态。
async function fetchIndices(id: string, force: boolean): Promise<{ data: EsIndex[]; rateWindowMs: number }> {
  const first = await catalogApi.indices(id, { page: 1, pageSize: INDICES_PAGE_SIZE }, force)
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
  return { data: all, rateWindowMs: first.rate_window_ms ?? 0 }
}

async function loadAll(force = false) {
  const id = clusterId.value
  if (!id) return
  const seq = ++loadSeq
  try {
    const [ov, shardRes, recRes] = await Promise.all([
      overviewApi.get(id, force),
      catalogApi.shards(id, force),
      catalogApi.recovery(id, force),
    ])
    const idx = await fetchIndices(id, force)
    // 代际不符：期间已发起新的加载，整批结果丢弃（连同 error 也不许回写）
    if (seq !== loadSeq) return
    overview.value = ov
    shards.value = shardRes.data ?? []
    indices.value = idx.data
    recoveries.value = recRes.data ?? []
    rateWindowMs.value = idx.rateWindowMs
    updatedAt.value = ov.updated_at || shardRes.updated_at || ''
    error.value = ''
  } catch (e) {
    if (seq === loadSeq) error.value = e instanceof Error ? e.message : t('common.error')
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
  const seq = loadSeq
  try {
    const value = await allocationApi.get(id)
    // 与数据同源：切集群后旧集群的响应不得写回，否则 B 的开关会显示 A 的值
    if (seq === loadSeq) allocation.value = value
  } catch {
    // 读取失败保持 all，不打断页面
  }
}

async function changeAllocation(value: string) {
  const next = value as AllocationEnable
  if (next === allocation.value) return
  const id = clusterId.value
  const seq = loadSeq
  const previous = allocation.value
  allocation.value = next // 乐观更新，失败回滚
  allocationSaving.value = true
  try {
    await allocationApi.set(id, next)
  } catch {
    // 已切集群：页面上显示的是另一个集群的开关，回滚会写错对象
    if (seq === loadSeq) allocation.value = previous
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
// 索引视角只展示 data 节点：master-only / coordinating 节点没有分片，占一整列只会挤压分片矩阵。
// 角色码不能只看 d：使用 data tier 的节点在 _cat/nodes 里是 h / w / c / f / s，只看 d 会把它们全部漏掉。
const DATA_ROLE_CODES = new Set(['d', 'h', 'w', 'c', 'f', 's'])
const isDataNode = (role: string) => Array.from(role || '').some(code => DATA_ROLE_CODES.has(code))

const nodeColumns = computed<WbNodeCol[]>(() =>
  (overview.value?.nodes ?? [])
    .filter(n => isDataNode(n.role))
    .map(n => ({
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
      // close / frozen 索引的副本数不代表在线冗余，不参与告警
      repZero: rows.filter(
        r => (r.index.status || 'open').toLowerCase() === 'open' && Number(r.index.rep) === 0,
      ).length,
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

const nodeRows = computed<WbNodeRow[]>(() =>
  (overview.value?.nodes ?? []).map(n => ({
    name: n.name,
    // _cat/nodes 缺列时是 "-"：宁可显示占位符，也不要把 "-:-" 拼给用户看
    addr: n.ip && n.port ? `${n.ip}:${n.port}` : '-',
    master: n.master === '*',
    heap: n.heap_percent,
    heapMax: n.heap_max && n.heap_max !== '-' ? n.heap_max : '',
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
  const { green, yellow, red } = healthCounts.value
  const total = green + yellow + red
  // 没有索引时（刚搭好的空集群）健康计数全为 0，按占比画会落成满圈红；
  // 此时环形图跟随集群自身状态，绿色的空集群就该是绿圈。
  if (!total) {
    const color = healthKey.value === 'red' ? 'var(--red)' : healthKey.value === 'yellow' ? 'var(--yellow)' : 'var(--green)'
    return { background: color }
  }
  const g = (green / total) * 100
  const gy = ((green + yellow) / total) * 100
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

// ---------- 活动区 · 恢复（同时供指标条的吞吐 / 剩余） ----------
// total 为 0 表示这一步没有工作量（如本地恢复的 translog）：显示 "—"，而不是假的 100%
function recoveryPct(done: number, total: number): number | null {
  if (!total || total <= 0) return null
  return Math.max(0, Math.min(100, Math.round((done / total) * 100)))
}

// 按已耗时降序：恢复是瞬时状态，最该先看一眼的是「耗了很久还没完」的那些，
// 不设阈值去猜「卡住」—— 多慢算慢是集群自己的事，排序把判断权交回给看图的人。
// 也不截断行数：没人恢复时这张表本来就是空的，真有几十上百条时它才是有价值的时刻；
// 活动区本身有 40vh 上限并内部滚动，行多只是多滚两下。
const recoveryRows = computed(() =>
  recoveries.value
    .map(r => {
      const bytesPct = recoveryPct(r.bytes_recovered, r.bytes_total)
      return {
        r,
        bytesPct,
        filesPct: recoveryPct(r.files_recovered, r.files_total),
        translogPct: recoveryPct(r.translog_recovered, r.translog_total),
        // 进度条只画「快慢」，具体量级放进 title：表格要保住密度，
        // 绝对值是按需查看的信息，不值得为它把每一行撑成两行高。
        filesText: r.files_total ? `${fmtInt(r.files_recovered)} / ${fmtInt(r.files_total)}` : '',
        bytesText: r.bytes_total ? `${fmtBytes(r.bytes_recovered)} / ${fmtBytes(r.bytes_total)}` : '',
        translogText: r.translog_total ? `${fmtInt(r.translog_recovered)} / ${fmtInt(r.translog_total)}` : '',
      }
    })
    .sort((a, b) => b.r.elapsed_ms - a.r.elapsed_ms),
)

// 吞吐 = Σ(单个分片的已恢复字节 / 它的耗时)，即各分片平均速率之和：
// 并行恢复时这才是真实的总吞吐（改用 Σ字节 / Σ耗时 会随并行度被稀释）。
// 一次快照即可算出，不需要像索引速率那样维护样本环；代价是它对「先快后停」的恢复不敏感，
// 判断某个恢复是否停住交给按耗时降序的表与它自己的进度（见 PRD §6.1 活动区）。
const recoverySummary = computed(() => {
  const rows = recoveries.value
  if (!rows.length) return null
  let bytesTotal = 0
  let bytesRecovered = 0
  let throughput = 0
  for (const r of rows) {
    bytesTotal += r.bytes_total
    bytesRecovered += r.bytes_recovered
    if (r.elapsed_ms > 0) throughput += (r.bytes_recovered / r.elapsed_ms) * 1000
  }
  const remaining = Math.max(0, bytesTotal - bytesRecovered)
  return {
    bytesTotal,
    bytesRecovered,
    remaining,
    percent: recoveryPct(bytesRecovered, bytesTotal) ?? 0,
    throughput,
    etaMs: throughput > 0 ? (remaining / throughput) * 1000 : null,
  }
})

// 剩余 = 按当前总吞吐把剩下的字节跑完还要多久；吞吐为 0（全卡住 / 尚未开始搬字节）时留空
const recoveryEta = computed(() =>
  recoverySummary.value?.etaMs != null ? `~${fmtDuration(recoverySummary.value.etaMs)}` : null,
)

// ---------- 活动区 · 问题 ----------
const unassignedList = computed(() => overview.value?.problems?.unassigned_shards ?? [])
const unhealthyList = computed(() => overview.value?.problems?.unhealthy_indices ?? [])
const unassignedTotal = computed(() => overview.value?.problems?.unassigned_shards_total ?? shardTotals.value.byState.unassigned)
const unhealthyTotal = computed(() => overview.value?.problems?.unhealthy_indices_total ?? 0)

// 活动区是否展开：恢复与问题任一非空即为「活动」。
// 恢复一侧以 _recovery 为准（表头计数、表格、指标条吞吐都同源），不再用 _cat/shards 的
// RELOCATING / INITIALIZING 拼一份 —— 两处口径不同会出现「计数 3、表格空」的矛盾。
const bandActive = computed(() =>
  recoveryRows.value.length > 0 || unassignedTotal.value > 0 || unhealthyTotal.value > 0,
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
// 拆成「前缀 / 数字 / 单位」渲染，数字用定宽槽承载，避免位数变化把右侧元素顶来顶去
const freshnessParts = computed(() => ageParts(updatedAt.value, now.value, t('freshness.updated')))

const unreachable = computed(() => !!overview.value && overview.value.reachable === false)

// ---------- 网格区状态 ----------
// 「等待数据」与「确实没有数据」必须分开，否则两者都只是「一张空表」。
const gridState = computed<'error' | 'skeleton' | 'empty' | 'ready'>(() => {
  if (unreachable.value || error.value) return 'error'
  if (!overview.value) return 'skeleton'
  // 节点视角的行来自 overview.nodes，与索引无关：集群没有索引时节点仍然要能看
  const hasRows = view.value === 'node' ? nodeRows.value.length > 0 : filteredIndices.value.length > 0
  return hasRows ? 'ready' : 'empty'
})

// 空态文案：只要有任何过滤条件在起作用，就该说「没有匹配」而不是「暂无索引」
const emptyText = computed(() =>
  view.value === 'node'
    ? t('grid.emptyNoNode')
    : hideSystem.value || onlyProblems.value || !!search.value.trim()
      ? t('grid.emptyNoIndex')
      : t('grid.emptyNoIndexClean'),
)

// 无数据时指标一律降级为占位符，避免把「还没拿到数据」读成 0
const shown = (value: string | number): string | number => (overview.value ? value : '—')

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
  pollTimer = window.setInterval(() => loadAll(true), 15000)
})

onBeforeUnmount(() => {
  loadSeq++ // 离场后到达的响应不再写回
  if (ticker) window.clearInterval(ticker)
  if (pollTimer) window.clearInterval(pollTimer)
})

watch(clusterId, (id) => {
  if (!id) return
  // 作废在途请求并清空上一个集群的数据：宁可短暂空白，也不能让 B 的页面显示 A 的节点 / 分片
  loadSeq++
  overview.value = null
  indices.value = []
  shards.value = []
  recoveries.value = []
  rateWindowMs.value = 0
  updatedAt.value = ''
  error.value = ''
  nodePage.value = 1
  indexPage.value = 1
  loadAll()
  loadAllocation()
})

// 集群已被删除、URL 又是历史 / 书签时收敛回 Hub：工作台只服务真实存在的集群，
// 否则会停在一个永远加载不出来的页面上（见 PRD §4）
watch(
  [() => clusterStore.loaded, clusterId, () => clusterStore.clusters.length],
  () => {
    if (clusterStore.loaded && !clusterStore.clusterById(clusterId.value)) router.replace('/')
  },
)
</script>

<template>
  <div class="esp-wb" :class="{ 'mode-console': mode === 'console' }">
    <!-- ===== top bar ===== -->
    <header class="topbar">
      <div class="brand">
        <img class="mark" src="/logo-v4-workbench.svg" alt="ESPulse" />
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
    <!-- 数据未到时整体降级为占位（.ph）：不摆 0 和 GREEN，否则会被读成「集群很健康」 -->
    <div class="vitals" :class="{ ph: !overview }">
      <!-- 健康：状态字 + 索引健康环形 + active 占比（不再单画进度条，避免与环形重复） -->
      <div class="vgrp">
        <!-- 版本是集群身份（静态），叠在胶囊上方并居中，借这一格已有的竖向空间，不新占横向位置 -->
        <div class="vstack">
          <span
            v-if="overview?.version"
            class="vver"
            :title="t('vitals.versionTitle', { version: overview.version })"
          >{{ overview.version }}</span>
          <span
            class="hchip"
            :style="overview ? { color: `var(--${healthKey})`, background: `var(--${healthKey}-bg)`, border: `1px solid var(--${healthKey}-bd)` } : undefined"
          >
            <span class="pip"></span>{{ overview ? healthKey.toUpperCase() : '—' }}
          </span>
        </div>
        <div class="donut sm" :style="overview ? donutStyle : undefined" :title="t('vitals.healthTitle')"></div>
        <div class="vstat">
          <span class="k">{{ t('vitals.activeShards') }}</span>
          <span class="v">{{ overview ? (overview.health.active_shards_percent ?? 0).toFixed(1) : '—' }}<small v-if="overview">%</small></span>
        </div>
      </div>

      <!-- 规模：数字等宽均分 -->
      <div class="vgrp vgfill">
        <div class="vstat"><span class="k">{{ t('vitals.nodes') }}</span><span class="v">{{ shown(counts.nodes) }}</span></div>
        <div class="vstat"><span class="k">{{ t('vitals.indices') }}</span><span class="v">{{ shown(fmtInt(counts.indices)) }}</span></div>
        <div class="vstat"><span class="k">{{ t('vitals.primaryShards') }}</span><span class="v">{{ shown(fmtInt(counts.pri)) }}</span></div>
        <div class="vstat"><span class="k">{{ t('vitals.totalShards') }}</span><span class="v">{{ shown(fmtInt(counts.shards)) }}</span></div>
        <div class="vstat"><span class="k">{{ t('vitals.docs') }}</span><span class="v">{{ shown(fmtInt(counts.docs)) }}</span></div>
        <div class="vstat"><span class="k">{{ t('vitals.store') }}</span><span class="v">{{ shown(fmtBytes(counts.store)) }}</span></div>
      </div>

      <!-- 分片构成：一条堆叠条（按状态分色，实心=主 / 半透明=副本），图例即数字 -->
      <div class="vgrp vgcol vggrow" style="min-width: 280px">
        <div class="vstat"><span class="k">{{ t('vitals.shardComposition') }}</span><span class="v">{{ shown(fmtInt(shardTotal)) }}</span></div>
        <div class="stack"><i v-for="(s, i) in shardBarSegments" :key="i" :style="s"></i></div>
        <div class="stackleg">
          <span><i style="background: var(--green)"></i>{{ t('toolbar.legendPri') }} <b>{{ shown(fmtInt(shardTotals.pri)) }}</b></span>
          <span><i style="background: var(--green); opacity: .42"></i>{{ t('toolbar.legendRep') }} <b>{{ shown(fmtInt(shardTotals.rep)) }}</b></span>
          <span><i style="background: var(--yellow)"></i>{{ t('toolbar.legendRelo') }} <b>{{ shown(shardTotals.byState.relocating) }}</b></span>
          <span><i style="background: var(--blue)"></i>{{ t('toolbar.legendInit') }} <b>{{ shown(shardTotals.byState.initializing) }}</b></span>
          <span><i style="background: repeating-linear-gradient(45deg, var(--red), var(--red) 2px, transparent 2px, transparent 4px)"></i>{{ t('toolbar.legendUn') }} <b>{{ shown(fmtInt(shardTotals.byState.unassigned)) }}</b></span>
        </div>
      </div>

      <!-- 恢复：吞吐 = Σ分片平均速率，剩余 = 按该吞吐跑完剩余字节的 ETA。
           空闲时留空而不摆 0 —— 0 会被读成「恢复停滞」 -->
      <div class="vgrp">
        <div class="vstat">
          <span class="k">{{ t('vitals.pendingTasks') }}</span>
          <span class="v">{{ shown(fmtInt(overview?.health.pending_tasks ?? 0)) }}</span>
        </div>
        <div class="vstat">
          <span class="k">{{ t('vitals.recoveryThroughput') }}</span>
          <span
            class="v"
            :class="{ dim: !recoverySummary }"
            :title="recoverySummary ? t('vitals.recoveryProgress', { done: fmtBytes(recoverySummary.bytesRecovered), total: fmtBytes(recoverySummary.bytesTotal) }) : undefined"
          >{{ recoverySummary ? `${fmtBytes(recoverySummary.throughput)}/s` : '—' }}</span>
        </div>
        <div class="vstat">
          <span class="k">{{ t('vitals.remaining') }}</span>
          <span
            class="v"
            :class="{ dim: !recoveryEta }"
            :title="recoverySummary ? t('vitals.recoveryLeft', { size: fmtBytes(recoverySummary.remaining) }) : undefined"
          >{{ recoveryEta ?? '—' }}</span>
        </div>
      </div>
    </div>

    <!-- ===== 活动区 ===== -->
    <div class="band" :class="{ on: bandShowActive }">
      <!-- 常驻头：左摘要随状态切换，右侧「健康 / 恢复」开关固定不动 -->
      <div class="bandhead">
        <div class="bsum">
          <template v-if="!bandShowActive">
            <template v-if="unreachable || error">
              <span class="ok" style="color: var(--red)">
                {{ unreachable ? t('common.unreachable') : error }}
              </span>
            </template>
            <template v-else-if="!overview">
              <!-- 数据未到：此时「全部正常」是假的，不能照常显示绿色结论 -->
              <span class="ok" style="color: var(--text-3)">{{ t('common.loading') }}</span>
            </template>
            <template v-else-if="bandCollapsedWithActivity">
              <span class="ok" style="color: var(--yellow)">
                {{ t('band.recovering') }} <b>{{ recoveryRows.length }}</b>
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
          </template>
          <template v-else>
            <span class="cnt">
              {{ t('band.recovering') }} <b>{{ recoveryRows.length }}</b>
              · {{ t('band.unassigned') }} <b>{{ unassignedTotal }}</b>
              · {{ t('band.nonGreen') }} <b>{{ unhealthyTotal }}</b>
            </span>
            <!-- 恢复规模：总量与平均进度。吞吐 / 剩余不放这里（指标条已有），避免同屏两处重复 -->
            <span v-if="recoverySummary" class="cnt" style="color: var(--text-3)">
              · {{ t('band.recoveryTotal', { size: fmtBytes(recoverySummary.bytesTotal) }) }}
              · {{ t('band.recoveryAvg', { pct: recoverySummary.percent }) }}
            </span>
          </template>
        </div>
        <div class="stateSw" :title="t('stateSw.title')">
          <div class="o" :class="{ on: !bandShowActive }" @click="bandForce = 'healthy'">
            <span class="d" style="color: var(--green)"></span>{{ t('stateSw.healthy') }}
          </div>
          <div class="o" :class="{ on: bandShowActive }" @click="bandForce = 'recovering'">
            <span class="d" style="color: var(--yellow)"></span>{{ t('stateSw.recovering') }}
          </div>
        </div>
      </div>

      <!-- 活动明细：仅展开态显示 -->
      <div class="active">
        <div class="eb-cols">
          <section class="eb-sec">
            <h4>
              {{ t('band.shardRecovery') }} <span class="n">{{ recoveryRows.length }}</span>
              <span class="hint">{{ t('band.recoveryHint') }}</span>
            </h4>
            <table class="rt">
              <colgroup>
                <col style="width: 150px" /><col style="width: 46px" />
                <col style="width: 60px" /><col style="width: 140px" />
                <col style="width: 86px" /><col style="width: 96px" /><col style="width: 86px" />
                <col style="width: 52px" />
              </colgroup>
              <thead>
                <tr>
                  <th>{{ t('grid.index') }}</th><th>{{ t('grid.priRep') }}</th>
                  <th>{{ t('band.recStage') }}</th><th>{{ t('band.recFromTo') }}</th><th>{{ t('band.recFiles') }}</th>
                  <th>{{ t('band.recBytes') }}</th><th>{{ t('band.recTranslog') }}</th><th>{{ t('band.recElapsed') }}</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="(row, i) in recoveryRows" :key="i">
                  <td class="ix" @click="filterToIndex(row.r.index)" :title="row.r.index">{{ row.r.index }}</td>
                  <td>
                    <!-- 复用网格那套分片方块：实心 = 主、描边 = 副，编号写在方块里。
                         类型（PEER / SNAPSHOT / EXISTING_STORE）放 title —— 它多数时候是 PEER，
                         只有节点重启 / 快照恢复时才是有解释力的信息 -->
                    <span
                      class="shard rec"
                      :class="{ rep: !row.r.primary }"
                      :title="`${row.r.primary ? 'P' : 'R'}${row.r.shard} · ${row.r.type}`"
                    >{{ row.r.shard }}</span>
                  </td>
                  <td class="stage" :class="row.r.stage.toLowerCase()">{{ row.r.stage }}</td>
                  <td><span class="dim">{{ row.r.source || '-' }}</span><span class="arrow">→</span>{{ row.r.target || '-' }}</td>
                  <td><div class="prog" :title="row.filesText || '—'"><span class="track"><i :style="{ width: (row.filesPct ?? 0) + '%' }"></i></span><span class="pct">{{ row.filesPct === null ? '—' : row.filesPct + '%' }}</span></div></td>
                  <td><div class="prog" :title="row.bytesText || '—'"><span class="track"><i :style="{ width: (row.bytesPct ?? 0) + '%' }"></i></span><span class="pct">{{ row.bytesPct === null ? '—' : row.bytesPct + '%' }}</span></div></td>
                  <td><div class="prog" :title="row.translogText || '—'"><span class="track"><i :style="{ width: (row.translogPct ?? 0) + '%' }"></i></span><span class="pct">{{ row.translogPct === null ? '—' : row.translogPct + '%' }}</span></div></td>
                  <td style="color: var(--text-3)">{{ fmtDuration(row.r.elapsed_ms) }}</td>
                </tr>
                <tr v-if="recoveryRows.length === 0">
                  <td colspan="8" class="dim">{{ t('common.none') }}</td>
                </tr>
              </tbody>
            </table>
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
        v-if="gridState === 'ready'"
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
      <!-- 骨架屏：沿用真实网格的列宽节奏，数据到位后不跳布局 -->
      <div v-else-if="gridState === 'skeleton'" class="gridskel">
        <div v-for="r in 9" :key="r" class="skrow">
          <span class="sk w-name"></span>
          <span class="sk w-num"></span>
          <span class="sk w-num"></span>
          <span v-for="c in SKEL_NODE_COLS" :key="c" class="sk w-cell"></span>
        </div>
      </div>
      <div v-else-if="gridState === 'empty'" class="gridmsg">{{ emptyText }}</div>
      <div v-else class="gridmsg err">{{ unreachable ? t('common.unreachable') : error }}</div>
    </div>

    <!-- ===== status line：状态信息、索引分页、数据新鲜度同处一行 ===== -->
    <div class="statusline">
      <span>{{ statusCount }}</span>
      <span>{{ statusHint }}</span>
      <span class="sp">{{ statusSum }}</span>
      <div v-if="view === 'index'" class="pager">
        <span>{{ t('grid.pageInfoShort', { page: currentIndexPage, pages: indexPages }) }}</span>
        <select :value="indexPageSize" @change="indexPageSize = Number(($event.target as HTMLSelectElement).value)">
          <option v-for="size in INDEX_PAGE_SIZES" :key="size" :value="size">{{ t('grid.perPage', { n: size }) }}</option>
        </select>
        <button :disabled="currentIndexPage <= 1" @click="changeIndexPage(-1)">{{ t('grid.prevPage') }}</button>
        <button :disabled="currentIndexPage >= indexPages" @click="changeIndexPage(1)">{{ t('grid.nextPage') }}</button>
      </div>
      <div class="fresh">
        <span class="live" :class="{ act: bandActive }"></span>
        <span v-if="bandActive">{{ t('freshness.recoveringSuffix') }}</span><template v-if="freshnessParts">{{ freshnessParts.prefix }} <span class="numslot">{{ freshnessParts.value }}</span> {{ freshnessParts.unit }}</template><template v-else>{{ NEVER_COLLECTED }}</template>
      </div>
    </div>
  </div>
</template>
