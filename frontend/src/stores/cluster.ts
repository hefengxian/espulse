import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { useLocalStorage } from '@vueuse/core'
import { clusterApi, type Cluster } from '../api/clusters'

export const useClusterStore = defineStore('cluster', () => {
  const clusters = ref<Cluster[]>([])
  const loading = ref(false)
  const error = ref<string | null>(null)

  // Use localStorage to persist the current cluster ID
  const currentClusterId = useLocalStorage<string | null>('espulse-current-cluster-id', null)

  const currentCluster = computed(() => {
    if (!currentClusterId.value) return null
    return clusters.value.find(c => c.id === currentClusterId.value) || null
  })

  async function fetchClusters() {
    loading.value = true
    error.value = null
    try {
      const data = await clusterApi.list()
      clusters.value = data
      
      // If no current cluster is selected, or the selected cluster no longer exists,
      // select the first one in the list.
      if (data.length > 0) {
        const stillExists = data.some(c => c.id === currentClusterId.value)
        if (!currentClusterId.value || !stillExists) {
          currentClusterId.value = data[0].id
        }
      } else {
        currentClusterId.value = null
      }
    } catch (err) {
      error.value = err instanceof Error ? err.message : 'Failed to fetch clusters'
      throw err
    } finally {
      loading.value = false
    }
  }

  function selectCluster(id: string) {
    const cluster = clusters.value.find(c => c.id === id)
    if (cluster) {
      currentClusterId.value = id
    }
  }

  async function addCluster(cluster: Partial<Cluster>) {
    const created = await clusterApi.create(cluster)
    clusters.value.push(created)
    currentClusterId.value = created.id
    return created
  }

  return {
    clusters,
    loading,
    error,
    currentClusterId,
    currentCluster,
    fetchClusters,
    selectCluster,
    addCluster
  }
})
