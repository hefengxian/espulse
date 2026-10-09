<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { NInput, NPopover } from 'naive-ui'
import { storeToRefs } from 'pinia'
import { useClusterStore } from '../stores/cluster'
import type { Cluster } from '../api/clusters'

const RECENT_KEY = 'espulse:recent-clusters'

// 超过这个数量才需要搜索框：少量集群时滚动比打字快
const SEARCH_THRESHOLD = 8

const route = useRoute()
const router = useRouter()
const clusterStore = useClusterStore()
const { clusters } = storeToRefs(clusterStore)

const open = ref(false)
const keyword = ref('')
const searchRef = ref<InstanceType<typeof NInput> | null>(null)

const clusterId = computed(() => route.params.id as string | undefined)
const current = computed(() => clusterStore.clusterById(clusterId.value))

// 最近使用优先：几乎免费地覆盖了「收藏 / pin」的绝大部分收益
const recents = ref<string[]>(readRecents())

function readRecents(): string[] {
  try {
    const raw = localStorage.getItem(RECENT_KEY)
    const parsed = raw ? JSON.parse(raw) : []
    return Array.isArray(parsed) ? parsed.filter((id): id is string => typeof id === 'string') : []
  } catch {
    return []
  }
}

function rememberRecents(id: string) {
  const next = [id, ...recents.value.filter(item => item !== id)].slice(0, 20)
  recents.value = next
  try {
    localStorage.setItem(RECENT_KEY, JSON.stringify(next))
  } catch {
    // 写不进去不影响切换本身
  }
}

const visibleClusters = computed(() => {
  const q = keyword.value.trim().toLowerCase()
  const list: Cluster[] = q
    ? clusters.value.filter(c =>
        c.name.toLowerCase().includes(q) ||
        (c.hosts ?? []).some(host => host.toLowerCase().includes(q)))
    : [...clusters.value]

  const rank = (c: Cluster) => {
    const index = recents.value.indexOf(c.id)
    return index === -1 ? Number.MAX_SAFE_INTEGER : index
  }
  return list.sort((a, b) => rank(a) - rank(b) || a.name.localeCompare(b.name))
})

const colorOf = (cluster: Cluster) => {
  const color = cluster.color || 'green'
  return color.startsWith('#') ? color : `var(--esp-${color})`
}

const hostOf = (cluster: Cluster) => (cluster.hosts ?? []).join(', ')

// 切集群保持当前模块：/cluster/A/shards → /cluster/B/shards（见 PRD §2.5）
function targetPath(id: string) {
  if (/^\/cluster\/[^/]+\//.test(route.path)) return route.path.replace(/^\/cluster\/[^/]+\//, `/cluster/${id}/`)
  return `/cluster/${id}/overview`
}

function select(cluster: Cluster) {
  open.value = false
  keyword.value = ''
  rememberRecents(cluster.id)
  // 不带 query：过滤条件挂在集群上，切过去应该是目标集群自己的条件
  router.push(targetPath(cluster.id))
}

function manage() {
  open.value = false
  keyword.value = ''
  if (route.path !== '/') router.push('/')
}

watch(open, async (isOpen) => {
  if (!isOpen || clusters.value.length <= SEARCH_THRESHOLD) return
  await nextTick()
  searchRef.value?.focus()
})
</script>

<template>
  <n-popover v-model:show="open" trigger="click" placement="bottom-start" :show-arrow="false" raw>
    <template #trigger>
      <button
        class="flex items-center gap-2 p-1.25 px-2.5 rounded-7px border border-border bg-bg cursor-pointer transition-all hover:border-border-2 max-w-64"
      >
        <span
          class="w-2 h-2 rounded-full flex-shrink-0"
          :style="{ backgroundColor: current ? colorOf(current) : 'var(--esp-text-3)' }"
        ></span>
        <span class="text-13px font-500 truncate">{{ current?.name || '全部集群' }}</span>
        <div class="w-3.5 h-3.5 text-text-3 flex-shrink-0" :class="open ? 'i-lucide-chevron-up' : 'i-lucide-chevron-down'"></div>
      </button>
    </template>

    <div class="w-76 border border-border rounded-10px bg-bg-2 shadow-lg overflow-hidden">
      <div class="px-3 py-2.5 border-b border-border flex items-center gap-2">
        <div class="w-3.5 h-3.5 text-text-3 flex-shrink-0 i-lucide-layers"></div>
        <span class="text-12px font-600">{{ clusterId ? '切换集群' : '全部集群' }}</span>
        <span class="text-11px text-text-3 ml-auto">{{ clusters.length }}</span>
      </div>

      <div v-if="clusters.length > SEARCH_THRESHOLD" class="px-2 pt-2">
        <n-input ref="searchRef" v-model:value="keyword" size="small" clearable placeholder="搜索名称或地址" />
      </div>

      <div class="max-h-80 overflow-y-auto p-1.5">
        <div v-if="clusters.length === 0" class="px-2.5 py-3 text-12px text-text-3">还没有配置任何集群</div>
        <div v-else-if="visibleClusters.length === 0" class="px-2.5 py-3 text-12px text-text-3">没有匹配的集群</div>
        <button
          v-for="cluster in visibleClusters"
          :key="cluster.id"
          class="w-full flex items-center gap-2 p-1.75 px-2.5 rounded-6px text-left border-none bg-transparent cursor-pointer transition-all hover:bg-bg-3"
          :class="{ 'bg-accent-glow': cluster.id === clusterId }"
          @click="select(cluster)"
        >
          <span class="w-2 h-2 rounded-full flex-shrink-0" :style="{ backgroundColor: colorOf(cluster) }"></span>
          <span class="text-13px font-500 truncate" :class="{ 'color-accent': cluster.id === clusterId }">{{ cluster.name }}</span>
          <span class="text-11px font-mono text-text-3 truncate ml-auto max-w-32">{{ hostOf(cluster) }}</span>
        </button>
      </div>

      <button
        class="w-full flex items-center gap-2 px-3 py-2 border-none border-t border-border bg-transparent text-text-2 cursor-pointer transition-all hover:bg-bg-3 hover:text-text"
        @click="manage"
      >
        <div class="w-3.5 h-3.5 flex-shrink-0 i-lucide-settings-2"></div>
        <span class="text-12.5px">管理集群</span>
      </button>
    </div>
  </n-popover>
</template>
