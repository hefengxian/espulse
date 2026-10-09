import { defineStore } from 'pinia'
import { ref } from 'vue'
import { clusterApi, type Cluster, type SaveResult } from '../api/clusters'

export const useClusterStore = defineStore('cluster', () => {
  const clusters = ref<Cluster[]>([])
  const loading = ref(false)
  const error = ref<string | null>(null)
  // 是否已经成功拉到过列表：用来区分「还没加载完」和「这个集群确实不存在」
  const loaded = ref(false)

  // 活动集群由 URL 决定（/cluster/:id/...），store 只提供按 id 查询
  function clusterById(id: string | undefined) {
    if (!id) return null
    return clusters.value.find(c => c.id === id) || null
  }

  async function fetchClusters() {
    loading.value = true
    error.value = null
    try {
      clusters.value = await clusterApi.list()
      loaded.value = true
    } catch (err) {
      error.value = err instanceof Error ? err.message : 'Failed to fetch clusters'
      throw err
    } finally {
      loading.value = false
    }
  }

  async function addCluster(cluster: Partial<Cluster>): Promise<SaveResult> {
    const result = await clusterApi.create(cluster)
    if (result.saved) await fetchClusters()
    return result
  }

  async function updateCluster(id: string, cluster: Partial<Cluster>): Promise<SaveResult> {
    const result = await clusterApi.update(id, cluster)
    if (result.saved) await fetchClusters()
    return result
  }

  async function removeCluster(id: string) {
    await clusterApi.remove(id)
    await fetchClusters()
  }

  // 手动刷新：触发后端重采，再拉取最新快照
  async function refreshAll() {
    await clusterApi.refresh()
    await fetchClusters()
  }

  return {
    clusters,
    loading,
    error,
    loaded,
    clusterById,
    fetchClusters,
    addCluster,
    updateCluster,
    removeCluster,
    refreshAll,
  }
})
