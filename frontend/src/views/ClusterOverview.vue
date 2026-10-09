<script setup lang="ts">
import { ref, computed, watch, onActivated, onDeactivated } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { NTooltip } from 'naive-ui'
import { useClusterStore } from '../stores/cluster'
import { overviewApi, type Overview } from '../api/overview'
import { registerRefresh, unregisterRefresh, setUpdatedAt } from '../composables/useGlobalRefresh'

const route = useRoute()
const router = useRouter()
const clusterStore = useClusterStore()

// 活动集群由 URL 决定
const clusterId = computed(() => route.params.id as string)
const cluster = computed(() => clusterStore.clusterById(clusterId.value))

const overview = ref<Overview | null>(null)
const loading = ref(false)
const error = ref('')

// 「更新于 x 秒前」由顶栏统一展示，页面自己不再维护刷新按钮与计时器
let pollTimer: number | undefined

async function load(silent = false, force = false) {
  const id = clusterId.value
  if (!id) return
  if (!silent) loading.value = true
  try {
    overview.value = await overviewApi.get(id, force)
    setUpdatedAt(overview.value?.updated_at)
    error.value = ''
  } catch (e) {
    error.value = e instanceof Error ? e.message : '加载失败'
  } finally {
    loading.value = false
  }
}

async function refresh() {
  await load(true, true)
}

// 顶栏的刷新是全局单入口：由当前激活的模块把自己「刷新当前集群」的实现注册进去
onActivated(() => {
  registerRefresh(refresh)
  load()
  // 轮询同时充当「页面仍在查看」的心跳，后端据此维持采集（见 PRD §6.3）
  pollTimer = window.setInterval(() => load(true), 15000)
})

// 模块被 keep-alive 保活，轮询必须在失活时停掉，否则切走的页面会在后台一直请求
onDeactivated(() => {
  unregisterRefresh(refresh)
  if (pollTimer) window.clearInterval(pollTimer)
  pollTimer = undefined
})

watch(clusterId, (id) => { if (id) load() })

// ---------- 展示辅助 ----------
const HEALTH_STYLE: Record<string, { text: string; border: string; bg: string; bar: string; label: string }> = {
  green: { text: 'color-green', border: 'border-green-border', bg: 'bg-green-bg', bar: 'bg-green', label: 'GREEN' },
  yellow: { text: 'color-yellow', border: 'border-yellow-border', bg: 'bg-yellow-bg', bar: 'bg-yellow', label: 'YELLOW' },
  red: { text: 'color-red', border: 'border-red-border', bg: 'bg-red-bg', bar: 'bg-red', label: 'RED' },
  unknown: { text: 'text-text-3', border: 'border-border', bg: 'bg-bg-3', bar: 'bg-text-3', label: 'UNKNOWN' },
}

const healthKey = computed(() => {
  const s = overview.value?.health.status
  return s === 'green' || s === 'yellow' || s === 'red' ? s : 'unknown'
})
const hs = computed(() => HEALTH_STYLE[healthKey.value])

const pct = computed(() => {
  const v = overview.value?.health.active_shards_percent ?? 0
  return Math.max(0, Math.min(100, v))
})

const fmtInt = (n: number) => n.toLocaleString('en-US')

const NODE_ROLE_LABELS: Record<string, string> = {
  c: '冷节点',
  d: '数据节点',
  f: '冻结节点',
  h: '热节点',
  i: '摄取节点',
  l: '机器学习节点',
  m: '主节点',
  r: '远程集群客户端',
  s: '内容节点',
  t: '转换节点',
  v: '仅投票节点',
  w: '温节点',
}

const formatNodeRole = (role: string) => {
  if (!role) return '-'
  if (role === '-') return '协调节点'
  const labels = Array.from(role, code => NODE_ROLE_LABELS[code])
  return labels.every(Boolean) ? labels.join(' · ') : role
}

const fmtBytes = (bytes: number) => {
  if (!bytes) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB', 'PB']
  let value = bytes
  let i = 0
  while (value >= 1024 && i < units.length - 1) {
    value /= 1024
    i++
  }
  return `${value >= 100 || i === 0 ? Math.round(value) : value.toFixed(1)} ${units[i]}`
}

const countCards = computed(() => {
  const c = overview.value?.counts
  if (!c) return []
  return [
    { label: '节点', value: fmtInt(c.nodes) },
    { label: '索引', value: fmtInt(c.indices) },
    { label: '主分片', value: fmtInt(c.primary_shards) },
    { label: '总分片', value: fmtInt(c.total_shards) },
    { label: '文档', value: fmtInt(c.docs) },
    { label: '存储', value: fmtBytes(c.store_bytes) },
  ]
})

const problemSections = computed(() => {
  const p = overview.value?.problems
  if (!p) return []
  return [
    {
      key: 'unassigned',
      title: '未分配分片',
      total: p.unassigned_shards_total,
      columns: ['索引', '分片', '主/副', '原因'],
      rows: p.unassigned_shards.map(s => [s.index, `#${s.shard}`, s.prirep, s.reason || '-']),
    },
    {
      key: 'relocating',
      title: '正在迁移',
      total: p.relocating_shards_total,
      columns: ['索引', '分片', '主/副', '目标节点'],
      rows: p.relocating_shards.map(s => [s.index, `#${s.shard}`, s.prirep, s.node || '-']),
    },
    {
      key: 'unhealthy',
      title: '非 Green 索引',
      total: p.unhealthy_indices_total,
      columns: ['索引', 'Health', '状态'],
      rows: p.unhealthy_indices.map(i => [i.index, i.health, i.status]),
    },
  ]
})

const problemsAllClear = computed(() => {
  const o = overview.value
  if (!o || !o.shards_loaded || !o.indices_loaded) return false
  const p = o.problems
  return p.unassigned_shards_total === 0 && p.relocating_shards_total === 0 && p.unhealthy_indices_total === 0
})

const showNodeShards = computed(() => overview.value?.nodes.some(n => n.shards !== undefined) ?? false)

// 问题清单是诊断入口：点索引名直接跳到分片分布并带上过滤条件（见 PRD §2.5）
const openShards = (index: string) => {
  if (!clusterId.value) return
  router.push({
    name: 'ClusterShards',
    params: { id: clusterId.value },
    query: { search: index, onlyProblem: '1' },
  })
}

// 资源使用率着色：超过 high 标红，超过 mid 标黄
const usageClass = (value: string | undefined, high: number, mid: number) => {
  const n = Number(value)
  if (!Number.isFinite(n)) return ''
  if (n >= high) return 'color-red'
  if (n >= mid) return 'color-yellow'
  return ''
}
</script>

<template>
  <div class="flex flex-col gap-5">
    <!-- 页头 -->
    <div class="flex items-center gap-2.5 flex-wrap">
      <div class="flex-1 min-w-0">
        <div class="text-18px font-600 tracking--0.4px truncate">{{ cluster?.name || '集群' }}</div>
        <div class="text-12px font-mono text-text-3 truncate">{{ cluster?.hosts?.join(', ') || '-' }}</div>
      </div>
    </div>

    <div v-if="error" class="border border-red-border bg-red-bg rounded-10px px-4 py-3 text-12.5px color-red font-mono">
      {{ error }}
    </div>

    <div v-if="loading && !overview" class="border border-border rounded-10px bg-bg-2 p-6 text-13px text-text-2">
      加载中…
    </div>

    <template v-else-if="overview">
      <!-- 集群不可达 -->
      <div
        v-if="!overview.reachable"
        class="border border-red-border bg-red-bg rounded-10px p-4 text-12.5px color-red font-mono"
      >
        集群不可达：{{ overview.error || '未知错误' }}
      </div>

      <template v-else>
        <!-- 健康条 -->
        <div class="border rounded-10px bg-bg-2 p-4 px-5 flex items-center gap-6 flex-wrap" :class="hs.border">
          <div class="flex items-center gap-2.5">
            <span class="w-3 h-3 rounded-full flex-shrink-0" :class="hs.bar"></span>
            <span class="text-19px font-700 tracking--0.4px" :class="hs.text">{{ hs.label }}</span>
          </div>
          <div class="flex-1 min-w-56 flex items-center gap-3">
            <div class="h-1.5 flex-1 rounded-full bg-bg-4 overflow-hidden">
              <div class="h-full rounded-full transition-all duration-300" :class="hs.bar" :style="{ width: pct + '%' }"></div>
            </div>
            <span class="text-12px font-mono text-text-2 whitespace-nowrap">{{ pct.toFixed(1) }}%</span>
          </div>
          <div class="flex items-center gap-4 text-12px font-mono">
            <span>未分配 <b :class="overview.health.unassigned_shards ? 'color-red' : ''">{{ overview.health.unassigned_shards }}</b></span>
            <span>迁移中 <b :class="overview.health.relocating_shards ? 'color-yellow' : ''">{{ overview.health.relocating_shards }}</b></span>
            <span>初始化 <b>{{ overview.health.initializing_shards }}</b></span>
          </div>
        </div>

        <!-- 关键计数 -->
        <div class="grid grid-cols-2 md:grid-cols-3 xl:grid-cols-6 gap-3">
          <div v-for="card in countCards" :key="card.label" class="border border-border rounded-10px bg-bg-2 p-3.5">
            <div class="text-11px text-text-3">{{ card.label }}</div>
            <div class="text-18px font-600 font-mono mt-0.5">{{ card.value }}</div>
          </div>
        </div>

        <!-- 节点 -->
        <section v-if="overview.nodes_loaded" class="border border-border rounded-10px bg-bg-2 overflow-hidden">
          <header class="px-4 py-3 border-b border-border flex items-center gap-2">
            <span class="text-13px font-600">节点</span>
            <span class="text-11.5px text-text-3">{{ overview.nodes.length }}</span>
          </header>
          <div class="overflow-x-auto">
            <table class="w-full text-12.5px">
              <thead>
                <tr class="text-text-3 text-left">
                  <th class="px-4 py-2 font-500">名称</th>
                  <th class="px-3 py-2 font-500">Heap</th>
                  <th class="px-3 py-2 font-500">CPU</th>
                  <th class="px-3 py-2 font-500" title="1 分钟 / 5 分钟 / 15 分钟系统平均负载">负载 (1/5/15m)</th>
                  <th class="px-3 py-2 font-500">磁盘</th>
                  <th v-if="showNodeShards" class="px-3 py-2 font-500">分片</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="node in overview.nodes" :key="node.name" class="border-t border-border">
                  <td class="px-4 py-2 font-mono">
                    <n-tooltip trigger="hover">
                      <template #trigger>
                        <span class="cursor-default">{{ node.name }}</span>
                      </template>
                      {{ formatNodeRole(node.role) }}
                    </n-tooltip>
                    <span v-if="node.master === '*'" class="color-yellow font-600" title="当前 Master 节点" aria-label="当前 Master 节点">★</span>
                  </td>
                  <td class="px-3 py-2 font-mono" :class="usageClass(node.heap_percent, 85, 70)">{{ node.heap_percent || '-' }}%</td>
                  <td class="px-3 py-2 font-mono" :class="usageClass(node.cpu, 90, 70)">{{ node.cpu || '-' }}%</td>
                  <td class="px-3 py-2 font-mono" title="1 分钟 / 5 分钟 / 15 分钟系统平均负载">{{ node.load_1m || '-' }} / {{ node.load_5m || '-' }} / {{ node.load_15m || '-' }}</td>
                  <td class="px-3 py-2 font-mono" :class="usageClass(node.disk_used_percent, 90, 75)">{{ node.disk_used_percent || '-' }}%</td>
                  <td v-if="showNodeShards" class="px-3 py-2 font-mono">{{ node.shards ?? '-' }}</td>
                </tr>
              </tbody>
            </table>
          </div>
        </section>

        <!-- 问题清单 -->
        <section class="flex flex-col gap-3">
          <div class="text-13px font-600">问题清单</div>

          <div
            v-if="problemsAllClear"
            class="border border-green-border bg-green-bg rounded-10px px-4 py-3 text-12.5px color-green"
          >
            ✓ 未发现未分配分片、迁移中的分片或非 Green 索引
          </div>

          <template v-else>
            <div
              v-for="sec in problemSections"
              :key="sec.key"
              class="border border-border rounded-10px bg-bg-2 overflow-hidden"
            >
              <header class="px-4 py-2.5 border-b border-border flex items-center gap-2">
                <span class="text-13px font-600">{{ sec.title }}</span>
                <span
                  class="text-11px px-1.5 py-0.5 rounded-4px font-mono"
                  :class="sec.total ? 'color-yellow bg-yellow-bg' : 'text-text-3 bg-bg-3'"
                >{{ sec.total }}</span>
                <span v-if="sec.total > sec.rows.length" class="text-11px text-text-3 ml-auto">
                  仅显示前 {{ sec.rows.length }} 条
                </span>
              </header>
              <div v-if="sec.rows.length === 0" class="px-4 py-3 text-12px text-text-3">无</div>
              <div v-else class="overflow-x-auto">
                <table class="w-full text-12px font-mono">
                  <thead>
                    <tr class="text-text-3 text-left">
                      <th v-for="col in sec.columns" :key="col" class="px-4 py-2 font-500">{{ col }}</th>
                    </tr>
                  </thead>
                  <tbody>
                    <tr v-for="(row, i) in sec.rows" :key="i" class="border-t border-border">
                      <td v-for="(cell, j) in row" :key="j" class="px-4 py-1.75" :class="j === 0 ? '' : 'text-text-2'">
                        <button
                          v-if="j === 0"
                          class="border-none bg-transparent p-0 text-left transition-all hover:color-accent hover:underline"
                          title="在分片分布中查看"
                          @click="openShards(cell)"
                        >{{ cell }}</button>
                        <template v-else>{{ cell }}</template>
                      </td>
                    </tr>
                  </tbody>
                </table>
              </div>
            </div>

            <div
              v-if="!overview.shards_loaded || !overview.indices_loaded"
              class="text-11.5px text-text-3"
            >
              分片 / 索引列表尚未就绪，上述问题清单可能不完整，稍后会自动刷新。
            </div>
          </template>
        </section>
      </template>
    </template>
  </div>
</template>
