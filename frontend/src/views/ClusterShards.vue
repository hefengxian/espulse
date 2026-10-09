<script setup lang="ts">
import { ref, computed, watch, onMounted, onBeforeUnmount } from 'vue'
import { useRoute } from 'vue-router'
import { NButton, NInput, NCheckbox, NRadioGroup, NRadioButton } from 'naive-ui'
import { useClusterStore } from '../stores/cluster'
import { catalogApi, allocationApi, type EsShard, type AllocationEnable } from '../api/catalog'
import { formatAge } from '../utils/freshness'

const route = useRoute()
const clusterStore = useClusterStore()

const clusterId = computed(() => route.params.id as string)
const cluster = computed(() => clusterStore.clusterById(clusterId.value))

const shards = ref<EsShard[]>([])
const updatedAt = ref('')
const loading = ref(false)
const refreshing = ref(false)
const error = ref('')

// ---------- 视图控制 ----------
type ViewMode = 'node' | 'index'
const viewMode = ref<ViewMode>('node')
const search = ref('')
const onlyProblem = ref(false)
// 系统索引（`.` 开头）多数场景不关心，默认隐藏，可切换（同 Cerebro）
const hideSystem = ref(true)
const limit = ref(30)

// 过滤条件变化时重置分页
watch([search, onlyProblem, viewMode, hideSystem], () => { limit.value = 30 })

const now = ref(Date.now())
let ticker: number | undefined
let pollTimer: number | undefined

async function load(silent = false, force = false) {
  const id = clusterId.value
  if (!id) return
  if (!silent) loading.value = true
  try {
    const res = await catalogApi.shards(id, force)
    shards.value = res.data ?? []
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

onMounted(() => {
  ticker = window.setInterval(() => { now.value = Date.now() }, 5000)
  load()
  loadAllocation()
  // 轮询同时充当「页面仍在查看」的心跳（见 PRD §6.3）
  pollTimer = window.setInterval(() => load(true), 15000)
})

onBeforeUnmount(() => {
  if (ticker) window.clearInterval(ticker)
  if (pollTimer) window.clearInterval(pollTimer)
})

watch(clusterId, (id) => { if (id) { load(); loadAllocation() } })

// ---------- 分片分配开关 ----------
// cluster.routing.allocation.enable：控制集群允许分配哪些分片，滚动重启等场景的常用开关。
const ALLOCATION_OPTIONS: { value: AllocationEnable; label: string; hint: string }[] = [
  { value: 'all', label: '全部分配', hint: '允许分配所有分片（默认）' },
  { value: 'primaries', label: '仅主分片', hint: '只分配主分片，副本暂停分配' },
  { value: 'new_primaries', label: '仅新主分片', hint: '只分配新创建索引的主分片' },
  { value: 'none', label: '禁止分配', hint: '暂停一切分片分配' },
]

const allocationEnable = ref<AllocationEnable>('all')
const allocationLoading = ref(false)
const allocationSaving = ref(false)
const allocationError = ref('')

const allocationHint = computed(
  () => ALLOCATION_OPTIONS.find(o => o.value === allocationEnable.value)?.hint || '',
)

async function loadAllocation() {
  const id = clusterId.value
  if (!id) return
  allocationLoading.value = true
  try {
    allocationEnable.value = await allocationApi.get(id)
    allocationError.value = ''
  } catch (e) {
    allocationError.value = e instanceof Error ? e.message : '读取失败'
  } finally {
    allocationLoading.value = false
  }
}

async function changeAllocation(value: string | number | boolean) {
  const next = value as AllocationEnable
  if (next === allocationEnable.value) return
  const previous = allocationEnable.value
  allocationEnable.value = next // 乐观更新，失败时回滚
  allocationSaving.value = true
  allocationError.value = ''
  try {
    await allocationApi.set(clusterId.value, next)
  } catch (e) {
    allocationEnable.value = previous
    allocationError.value = e instanceof Error ? e.message : '设置失败'
  } finally {
    allocationSaving.value = false
  }
}

// ---------- 数据整形 ----------
const UNASSIGNED_NODE = '(未分配)'

const stateOf = (s: EsShard) => (s.state || '').toUpperCase()

// 每个索引的分片统计，用于排序与「只看有问题」过滤
const indexStats = computed(() => {
  const map = new Map<string, { total: number; unassigned: number; relocating: number }>()
  for (const s of shards.value) {
    const stat = map.get(s.index) || { total: 0, unassigned: 0, relocating: 0 }
    stat.total++
    if (stateOf(s) === 'UNASSIGNED') stat.unassigned++
    if (stateOf(s) === 'RELOCATING') stat.relocating++
    map.set(s.index, stat)
  }
  return map
})

const hasProblem = (index: string) => {
  const stat = indexStats.value.get(index)
  return !!stat && (stat.unassigned > 0 || stat.relocating > 0)
}

// 索引轴：过滤 → 问题优先排序 → 分页
const indexKeys = computed(() => {
  const q = search.value.trim().toLowerCase()
  let list = [...indexStats.value.keys()]
  if (hideSystem.value) list = list.filter(i => !i.startsWith('.'))
  if (q) list = list.filter(i => i.toLowerCase().includes(q))
  if (onlyProblem.value) list = list.filter(hasProblem)

  list.sort((a, b) => {
    const pa = hasProblem(a) ? 0 : 1
    const pb = hasProblem(b) ? 0 : 1
    if (pa !== pb) return pa - pb
    const sa = indexStats.value.get(a)!
    const sb = indexStats.value.get(b)!
    if (sa.unassigned !== sb.unassigned) return sb.unassigned - sa.unassigned
    if (sa.relocating !== sb.relocating) return sb.relocating - sa.relocating
    return a.localeCompare(b)
  })
  return list
})

const visibleIndexKeys = computed(() => indexKeys.value.slice(0, limit.value))
const remainingIndexes = computed(() => Math.max(0, indexKeys.value.length - visibleIndexKeys.value.length))

// 顶部汇总仍是整个集群的数字，被隐藏的系统索引数量需要说明，避免与矩阵里的索引数对不上
const hiddenSystemCount = computed(() =>
  hideSystem.value ? [...indexStats.value.keys()].filter(k => k.startsWith('.')).length : 0)

// 节点轴：有界，不翻页；未分配的伪节点排在最后
const nodeKeys = computed(() => {
  const set = new Set<string>()
  for (const s of shards.value) set.add(s.node || UNASSIGNED_NODE)
  const list = [...set].filter(n => n !== UNASSIGNED_NODE).sort()
  if (set.has(UNASSIGNED_NODE)) list.push(UNASSIGNED_NODE)
  return list
})

const isPseudo = (key: string) => key === UNASSIGNED_NODE

// 真实节点数（不含「(未分配)」伪节点），用于汇总展示
const realNodeCount = computed(() => nodeKeys.value.filter(n => !isPseudo(n)).length)

// (索引, 节点) → 分片列表
const cellMap = computed(() => {
  const map = new Map<string, EsShard[]>()
  for (const s of shards.value) {
    const key = s.index + '\u0000' + (s.node || UNASSIGNED_NODE)
    const list = map.get(key)
    if (list) list.push(s)
    else map.set(key, [s])
  }
  return map
})

const rows = computed(() => (viewMode.value === 'node' ? nodeKeys.value : visibleIndexKeys.value))
const cols = computed(() => (viewMode.value === 'node' ? visibleIndexKeys.value : nodeKeys.value))
const rowAxisLabel = computed(() => (viewMode.value === 'node' ? '节点 \\ 索引' : '索引 \\ 节点'))

const grid = computed(() => {
  const byNode = viewMode.value === 'node'
  return rows.value.map(rowKey => ({
    key: rowKey,
    cells: cols.value.map(colKey => {
      const index = byNode ? colKey : rowKey
      const node = byNode ? rowKey : colKey
      return cellMap.value.get(index + '\u0000' + node) || []
    }),
  }))
})

const stats = computed(() => {
  let active = 0, unassigned = 0, relocating = 0, initializing = 0
  for (const s of shards.value) {
    switch (stateOf(s)) {
      case 'UNASSIGNED': unassigned++; break
      case 'RELOCATING': relocating++; break
      case 'INITIALIZING': initializing++; break
      default: active++
    }
  }
  return { total: shards.value.length, active, unassigned, relocating, initializing }
})

// ---------- 展示辅助 ----------
// 颜色表示状态；实心=主分片，空心=副本分片（用形状而非仅颜色区分，兼顾色盲）
const CHIP_STYLE: Record<string, { primary: string; replica: string }> = {
  STARTED: { primary: 'bg-green color-white', replica: 'border border-green color-green' },
  RELOCATING: { primary: 'bg-yellow color-white', replica: 'border border-yellow color-yellow' },
  INITIALIZING: { primary: 'bg-accent color-white', replica: 'border border-accent color-accent' },
  UNASSIGNED: { primary: 'bg-red color-white border border-dashed border-red', replica: 'border border-dashed border-red color-red' },
}

const chipClass = (s: EsShard) => {
  const style = CHIP_STYLE[stateOf(s)] || CHIP_STYLE.STARTED
  return s.prirep === 'p' ? style.primary : style.replica
}

const chipTip = (s: EsShard) => {
  const parts = [s.index, `#${s.shard}`, s.prirep === 'p' ? '主分片' : '副本分片', stateOf(s)]
  if (s.node) parts.push('@ ' + s.node)
  const reason = s['unassigned.reason']
  if (reason) parts.push(`(${reason})`)
  return parts.join(' ')
}

const freshnessText = computed(() => formatAge(updatedAt.value, now.value))
</script>

<template>
  <div class="flex flex-col gap-4">
    <!-- 页头 -->
    <div class="flex items-center gap-2.5 flex-wrap">
      <div class="flex-1 min-w-0">
        <div class="text-18px font-600 tracking--0.4px truncate">分片分布</div>
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

    <!-- 汇总 -->
    <div class="flex items-center gap-3 flex-wrap text-12px font-mono border border-border rounded-10px bg-bg-2 px-4 py-2.5">
      <span class="text-text-2">总分片 <b class="text-text">{{ stats.total }}</b></span>
      <span class="text-text-2">已分配 <b class="text-text">{{ stats.active }}</b></span>
      <span :class="stats.unassigned ? 'color-red' : 'text-text-2'">未分配 <b>{{ stats.unassigned }}</b></span>
      <span :class="stats.relocating ? 'color-yellow' : 'text-text-2'">迁移中 <b>{{ stats.relocating }}</b></span>
      <span :class="stats.initializing ? 'color-accent' : 'text-text-2'">初始化 <b>{{ stats.initializing }}</b></span>
      <span class="text-text-3">索引 <b class="text-text-2">{{ indexStats.size }}</b> · 节点 <b class="text-text-2">{{ realNodeCount }}</b></span>
      <span v-if="hiddenSystemCount" class="text-text-3">已隐藏 {{ hiddenSystemCount }} 个系统索引</span>
    </div>

    <!-- 分片分配开关 -->
    <div class="border border-border rounded-10px bg-bg-2 px-4 py-3 flex items-center gap-4 flex-wrap">
      <div class="min-w-0">
        <div class="text-12.5px font-500 text-text">分片分配</div>
        <div class="text-11px text-text-3 font-mono">cluster.routing.allocation.enable</div>
      </div>
      <n-radio-group
        :value="allocationEnable"
        :disabled="allocationLoading || allocationSaving"
        size="small"
        @update:value="changeAllocation"
      >
        <n-radio-button v-for="opt in ALLOCATION_OPTIONS" :key="opt.value" :value="opt.value">
          {{ opt.label }}
        </n-radio-button>
      </n-radio-group>
      <span class="text-11.5px text-text-3">{{ allocationHint }}</span>
      <span v-if="allocationSaving" class="text-11.5px text-text-3">保存中…</span>
      <span v-else-if="allocationError" class="text-11.5px color-red">{{ allocationError }}</span>
    </div>

    <!-- 工具栏 -->
    <div class="flex items-center gap-2.5 flex-wrap">
      <div class="flex items-center border border-border rounded-7px overflow-hidden">
        <button
          class="px-3 py-1.25 text-12.5px transition-all"
          :class="viewMode === 'node' ? 'bg-accent-glow color-accent font-500' : 'text-text-2 hover:bg-bg-3'"
          @click="viewMode = 'node'"
        >按节点看</button>
        <button
          class="px-3 py-1.25 text-12.5px transition-all"
          :class="viewMode === 'index' ? 'bg-accent-glow color-accent font-500' : 'text-text-2 hover:bg-bg-3'"
          @click="viewMode = 'index'"
        >按索引看</button>
      </div>

      <n-input v-model:value="search" size="small" clearable placeholder="过滤索引名" class="w-56" />
      <n-checkbox v-model:checked="hideSystem">隐藏 . 开头的索引</n-checkbox>
      <n-checkbox v-model:checked="onlyProblem">只看有问题的索引</n-checkbox>

      <div class="ml-auto flex items-center gap-3 flex-wrap text-11.5px text-text-3">
        <span class="flex items-center gap-1"><span class="chip bg-green color-white">0</span> 主分片</span>
        <span class="flex items-center gap-1"><span class="chip border border-green color-green">0</span> 副本分片</span>
        <span class="text-border-2">|</span>
        <span class="flex items-center gap-1"><span class="chip bg-green color-white"></span>STARTED</span>
        <span class="flex items-center gap-1"><span class="chip bg-yellow color-white"></span>RELOCATING</span>
        <span class="flex items-center gap-1"><span class="chip bg-accent color-white"></span>INITIALIZING</span>
        <span class="flex items-center gap-1"><span class="chip border border-dashed border-red color-red"></span>UNASSIGNED</span>
      </div>
    </div>

    <!-- 空 / 加载 -->
    <div v-if="loading && shards.length === 0" class="border border-border rounded-10px bg-bg-2 p-6 text-13px text-text-2">
      加载中…
    </div>
    <div v-else-if="shards.length === 0" class="border border-border rounded-10px bg-bg-2 p-6 text-13px text-text-2">
      该集群暂无分片。
    </div>
    <div v-else-if="indexKeys.length === 0" class="border border-border rounded-10px bg-bg-2 p-6 text-13px text-text-2">
      没有匹配的索引{{ hideSystem ? '（. 开头的索引已隐藏）' : '' }}。
    </div>

    <!-- 矩阵 -->
    <div v-else class="border border-border rounded-10px bg-bg-2 overflow-hidden">
      <div class="overflow-auto" style="max-height: calc(100vh - 320px)">
        <table class="border-collapse select-none">
          <thead>
            <tr>
              <th class="sticky left-0 top-0 z-30 bg-bg-3 border-b border-r border-border px-3 py-2 text-left text-11.5px font-500 text-text-3 whitespace-nowrap">
                {{ rowAxisLabel }}
              </th>
              <th
                v-for="col in cols"
                :key="col"
                class="sticky top-0 z-20 bg-bg-3 border-b border-r border-border px-2 py-2 text-11.5px font-500 whitespace-nowrap"
                :class="isPseudo(col) ? 'color-red' : 'text-text-2'"
              >
                <div class="max-w-36 truncate" :title="col">{{ col }}</div>
              </th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="row in grid" :key="row.key" class="hover:bg-bg-3">
              <th
                class="sticky left-0 z-10 bg-bg-2 border-b border-r border-border px-3 py-1.5 text-left text-11.5px font-500 whitespace-nowrap"
                :class="isPseudo(row.key) ? 'color-red' : 'text-text-2'"
              >
                <div class="max-w-52 truncate" :title="row.key">{{ row.key }}</div>
              </th>
              <td
                v-for="(cell, ci) in row.cells"
                :key="ci"
                class="border-b border-r border-border px-1.5 py-1 align-top"
              >
                <div class="flex flex-wrap gap-0.5 min-w-16">
                  <span
                    v-for="(s, si) in cell"
                    :key="si"
                    class="chip"
                    :class="chipClass(s)"
                    :title="chipTip(s)"
                  >{{ s.shard }}</span>
                </div>
              </td>
            </tr>
          </tbody>
        </table>
      </div>

      <!-- 索引轴分页 -->
      <div v-if="remainingIndexes > 0" class="border-t border-border px-4 py-2.5 flex items-center gap-3">
        <n-button size="tiny" @click="limit += 30">显示更多</n-button>
        <span class="text-11.5px text-text-3">还有 {{ remainingIndexes }} 个索引未显示（筛选后共 {{ indexKeys.length }} 个）</span>
      </div>
    </div>
  </div>
</template>
