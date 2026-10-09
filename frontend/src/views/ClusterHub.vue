<script setup lang="ts">
import { ref, computed, onActivated, onDeactivated } from 'vue'
import { storeToRefs } from 'pinia'
import { useRouter } from 'vue-router'
import {
  NButton,
  NModal,
  NForm,
  NFormItem,
  NInput,
  NSelect,
  NSpace,
  NPopconfirm,
  useMessage
} from 'naive-ui'
import { useClusterStore } from '../stores/cluster'
import { clusterApi, type Cluster } from '../api/clusters'
import { formatAge } from '../utils/freshness'
import { registerRefresh, unregisterRefresh } from '../composables/useGlobalRefresh'

const router = useRouter()
const message = useMessage()
const clusterStore = useClusterStore()
const { clusters, loading } = storeToRefs(clusterStore)

// 让「更新于 x 秒前」自行跳动，避免打开页面后时间就静止
const now = ref(Date.now())
let ticker: number | undefined

// ---------- 新增 / 编辑表单 ----------
const showModal = ref(false)
const editingId = ref<string | null>(null)
const saving = ref(false)
const probing = ref(false)
const probeResult = ref<{ reachable: boolean; message: string } | null>(null)
const formError = ref('')

const emptyForm = () => ({
  name: '',
  hosts: 'http://localhost:9200',
  auth_type: 'none',
  username: '',
  password: '',
  api_key: '',
  color: 'green',
  notes: '',
})

const form = ref(emptyForm())

const authOptions = [
  { label: '无', value: 'none' },
  { label: 'Basic Auth', value: 'basic' },
  { label: 'API Key', value: 'api_key' },
]

const colorOptions = [
  { label: 'Green', value: 'green' },
  { label: 'Yellow', value: 'yellow' },
  { label: 'Red', value: 'red' },
  { label: 'Blue', value: 'accent' },
]

const openCreate = () => {
  editingId.value = null
  form.value = emptyForm()
  probeResult.value = null
  formError.value = ''
  showModal.value = true
}

const openEdit = (cluster: Cluster) => {
  editingId.value = cluster.id
  form.value = {
    name: cluster.name,
    hosts: cluster.hosts.join(', '),
    auth_type: cluster.auth_type || 'none',
    username: cluster.username || '',
    // 密钥留空表示不修改，避免把密码回传给前端
    password: '',
    api_key: '',
    color: cluster.color || 'green',
    notes: cluster.notes || '',
  }
  probeResult.value = null
  formError.value = ''
  showModal.value = true
}

const payload = () => ({
  ...form.value,
  hosts: form.value.hosts.split(',').map(h => h.trim()).filter(Boolean),
})

const handleProbe = async () => {
  probing.value = true
  probeResult.value = null
  try {
    const res = await clusterApi.probe(payload())
    if (res.reachable && res.info) {
      probeResult.value = {
        reachable: true,
        message: `发现集群 ${res.info.name} · ES ${res.info.version} · ${res.info.health} · ${res.info.node_count} 个节点`
      }
    } else {
      probeResult.value = { reachable: false, message: res.error || '无法连接到该集群' }
    }
  } catch {
    probeResult.value = { reachable: false, message: '探测请求失败' }
  } finally {
    probing.value = false
  }
}

const handleSave = async () => {
  formError.value = ''

  if (!form.value.name.trim()) {
    formError.value = '请填写集群名称'
    return
  }
  if (!form.value.hosts.trim()) {
    formError.value = '请填写至少一个 host'
    return
  }

  saving.value = true
  try {
    const result = editingId.value
      ? await clusterStore.updateCluster(editingId.value, payload())
      : await clusterStore.addCluster(payload())

    if (!result.saved) {
      // 连接信息校验未通过：保留弹窗并展示原因
      formError.value = result.error || '保存失败'
      return
    }

    message.success(editingId.value ? '集群已更新' : '集群已添加')
    showModal.value = false
  } catch (err) {
    formError.value = err instanceof Error ? err.message : '保存请求失败'
  } finally {
    saving.value = false
  }
}

const handleDelete = async (cluster: Cluster) => {
  try {
    await clusterStore.removeCluster(cluster.id)
    message.success(`已删除 ${cluster.name}`)
  } catch {
    message.error('删除失败')
  }
}

const handleRefresh = async () => {
  try {
    await clusterStore.refreshAll()
  } catch {
    message.error('刷新失败')
  }
}

// ---------- 状态展示 ----------
type HealthKey = 'green' | 'yellow' | 'red' | 'unreachable' | 'unknown'

const HEALTH_LABEL: Record<HealthKey, string> = {
  green: 'GREEN',
  yellow: 'YELLOW',
  red: 'RED',
  unreachable: 'UNREACHABLE',
  unknown: 'UNKNOWN',
}

const HEALTH_CLASS: Record<HealthKey, string> = {
  green: 'border-green-border bg-green-bg color-green',
  yellow: 'border-yellow-border bg-yellow-bg color-yellow',
  red: 'border-red-border bg-red-bg color-red',
  unreachable: 'border-border-2 bg-bg-3 color-text-3 border-dashed',
  unknown: 'border-border bg-bg-3 color-text-3',
}

// 排序权重：连不上 → 红 → 黄 → 绿。这与「谁红了」的使用意图一致
const HEALTH_RANK: Record<HealthKey, number> = {
  unreachable: 0,
  red: 1,
  yellow: 2,
  green: 3,
  unknown: 4,
}

const healthOf = (cluster: Cluster): HealthKey => {
  if (!cluster.status) return 'unknown'
  if (!cluster.status.reachable) return 'unreachable'
  const health = cluster.status.health
  if (health === 'green' || health === 'yellow' || health === 'red') return health
  return 'unknown'
}

const sortedClusters = computed(() =>
  [...clusters.value].sort((a, b) => {
    const rankDiff = HEALTH_RANK[healthOf(a)] - HEALTH_RANK[healthOf(b)]
    if (rankDiff !== 0) return rankDiff

    const unassignedDiff = (b.status?.unassigned_shards ?? 0) - (a.status?.unassigned_shards ?? 0)
    if (unassignedDiff !== 0) return unassignedDiff

    return a.name.localeCompare(b.name)
  })
)

const freshnessText = (cluster: Cluster) =>
  formatAge(
    cluster.status?.collected_at,
    now.value,
    cluster.status?.reachable ? '更新于' : '采集失败于',
  )

const enterCluster = (cluster: Cluster) => {
  router.push(`/cluster/${cluster.id}/overview`)
}

// 历史数据的 color 可能是十六进制值（如 #18a058），也可能是指令色名（green）
const colorOf = (cluster: Cluster) => {
  const color = cluster.color || 'green'
  return color.startsWith('#') ? color : `var(--esp-${color})`
}

// 在 Hub 页，顶栏的刷新作用于集群列表本身（没有活动集群可言）
onActivated(async () => {
  registerRefresh(handleRefresh)
  ticker = window.setInterval(() => { now.value = Date.now() }, 5000)
  try {
    await clusterStore.fetchClusters()
  } catch {
    message.error('加载集群列表失败')
  }
})

onDeactivated(() => {
  unregisterRefresh(handleRefresh)
  if (ticker) window.clearInterval(ticker)
  ticker = undefined
})
</script>

<template>
  <div class="flex flex-col gap-5 p-6">
    <!-- 页头 -->
    <div class="flex items-center gap-2.5">
      <div class="flex-1">
        <div class="text-18px font-600 tracking--0.4px">集群</div>
        <div class="text-12.5px text-text-2">
          {{ clusters.length }} 个集群 · 状态由后端每 5 秒采集一次
        </div>
      </div>
      <n-button v-if="clusters.length > 0" type="primary" @click="openCreate">添加集群</n-button>
    </div>

    <!-- 空状态 -->
    <div
      v-if="!loading && clusters.length === 0"
      class="flex flex-col items-center gap-3 py-20 border border-border rounded-10px bg-bg-2"
    >
      <div class="text-16px font-600">还没有配置任何集群</div>
      <div class="text-13px text-text-2 max-w-140 text-center leading-relaxed">
        ESPulse 是一个零依赖的 Elasticsearch 集群管理工具，用来替代依赖 JDK 的 Cerebro。
        添加一个集群端点即可开始。
      </div>
      <n-button type="primary" @click="openCreate">添加第一个集群</n-button>
    </div>

    <!-- 集群卡片墙 -->
    <div v-else class="grid grid-cols-1 xl:grid-cols-2 2xl:grid-cols-3 gap-3">
      <div
        v-for="cluster in sortedClusters"
        :key="cluster.id"
        class="border border-border rounded-10px bg-bg-2 p-4 flex flex-col gap-3 transition-all hover:border-border-2"
      >
        <!-- ① 标识 + 操作 -->
        <div class="flex items-start gap-2.5">
          <div
            class="w-2.5 h-2.5 rounded-full mt-1.5 flex-shrink-0"
            :style="{ backgroundColor: colorOf(cluster) }"
          ></div>
          <div class="flex-1 min-w-0">
            <div
              class="text-14px font-600 truncate cursor-pointer hover:text-accent"
              @click="enterCluster(cluster)"
            >
              {{ cluster.name }}
            </div>
            <div class="text-11.5px font-mono text-text-3 truncate">{{ cluster.hosts.join(', ') }}</div>
          </div>
          <div class="flex items-center gap-1 flex-shrink-0">
            <button class="btn-icon" title="编辑" @click="openEdit(cluster)">
              <div class="w-3.5 h-3.5 i-lucide-pencil"></div>
            </button>
            <n-popconfirm @positive-click="handleDelete(cluster)">
              <template #trigger>
                <button class="btn-icon" title="删除">
                  <div class="w-3.5 h-3.5 i-lucide-trash-2"></div>
                </button>
              </template>
              确定删除集群「{{ cluster.name }}」？该操作不可撤销。
            </n-popconfirm>
          </div>
        </div>

        <!-- ③ 状态（来自采集快照） -->
        <div class="flex flex-wrap items-center gap-x-3 gap-y-2 border-t border-border pt-3 text-12px font-mono">
          <span
            class="flex items-center gap-1.25 px-1.5 py-0.5 rounded-4px border text-11px font-600"
            :class="HEALTH_CLASS[healthOf(cluster)]"
          >
            <span class="w-1.5 h-1.5 rounded-full bg-current"></span>{{ HEALTH_LABEL[healthOf(cluster)] }}
          </span>
          <span class="text-text-2">ES {{ cluster.status?.version || '-' }}</span>
          <span class="text-text-2">{{ cluster.status?.node_count ?? '-' }} 节点</span>
          <span class="text-text-2">{{ cluster.status?.index_count ?? '-' }} 索引</span>
          <span :class="(cluster.status?.unassigned_shards ?? 0) > 0 ? 'color-yellow' : 'text-text-3'">
            {{ cluster.status?.unassigned_shards ?? '-' }} 未分配
          </span>
        </div>

        <!-- ④ 数据新鲜度 -->
        <div class="text-11px text-text-3 flex items-center gap-1.5 min-w-0">
          <span class="flex-shrink-0">{{ freshnessText(cluster) }}</span>
          <span
            v-if="cluster.status && !cluster.status.reachable && cluster.status.error"
            class="truncate color-red"
            :title="cluster.status.error"
          >
            · {{ cluster.status.error }}
          </span>
        </div>
      </div>
    </div>

    <!-- 新增 / 编辑集群 -->
    <n-modal
      v-model:show="showModal"
      preset="card"
      :title="editingId ? '编辑集群' : '添加集群'"
      class="max-w-lg bg-bg-2"
    >
      <n-form :model="form" label-placement="top">
        <n-form-item label="集群名称">
          <n-input v-model:value="form.name" placeholder="例如：生产集群 A" />
        </n-form-item>
        <n-form-item label="Hosts（多个用逗号分隔）">
          <n-input v-model:value="form.hosts" placeholder="http://localhost:9200" />
        </n-form-item>
        <n-form-item label="认证方式">
          <n-select v-model:value="form.auth_type" :options="authOptions" />
        </n-form-item>
        <template v-if="form.auth_type === 'basic'">
          <n-form-item label="用户名">
            <n-input v-model:value="form.username" />
          </n-form-item>
          <n-form-item :label="editingId ? '密码（留空表示不修改）' : '密码'">
            <n-input v-model:value="form.password" type="password" />
          </n-form-item>
        </template>
        <template v-if="form.auth_type === 'api_key'">
          <n-form-item :label="editingId ? 'API Key（留空表示不修改）' : 'API Key'">
            <n-input v-model:value="form.api_key" type="password" />
          </n-form-item>
        </template>
        <n-form-item label="标识色">
          <n-select v-model:value="form.color" :options="colorOptions" />
        </n-form-item>
      </n-form>

      <div
        v-if="probeResult"
        class="text-12px font-mono mb-1"
        :class="probeResult.reachable ? 'color-green' : 'color-red'"
      >
        {{ probeResult.reachable ? '✓' : '✕' }} {{ probeResult.message }}
      </div>
      <div v-if="formError" class="text-12px font-mono color-red">✕ {{ formError }}</div>

      <template #footer>
        <n-space justify="end">
          <n-button @click="showModal = false">取消</n-button>
          <n-button :loading="probing" @click="handleProbe">测试连接</n-button>
          <n-button type="primary" :loading="saving" @click="handleSave">保存</n-button>
        </n-space>
      </template>
    </n-modal>
  </div>
</template>
