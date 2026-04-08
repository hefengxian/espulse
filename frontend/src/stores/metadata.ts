import { defineStore } from 'pinia'
import { ref } from 'vue'
import { useClusterStore } from './cluster'

export const useMetadataStore = defineStore('metadata', () => {
  const clusterStore = useClusterStore()
  const indices = ref<string[]>([])
  const fields = ref<Record<string, string[]>>({})
  const isLoading = ref(false)

  async function fetchIndices() {
    if (!clusterStore.currentClusterId) return
    
    isLoading.value = true
    try {
      const response = await fetch(`/api/proxy/_cat/indices?format=json`, {
        headers: {
          'X-Cluster-ID': clusterStore.currentClusterId
        }
      })
      if (response.ok) {
        const data = await response.json()
        indices.value = data.map((idx: any) => idx.index).sort()
      }
    } catch (err) {
      console.error('Failed to fetch indices:', err)
    } finally {
      isLoading.value = false
    }
  }

  async function fetchFields(indexName: string) {
    if (!clusterStore.currentClusterId || fields.value[indexName]) return
    
    try {
      const response = await fetch(`/api/proxy/${indexName}/_mapping`, {
        headers: {
          'X-Cluster-ID': clusterStore.currentClusterId
        }
      })
      if (response.ok) {
        const data = await response.json()
        const indexMapping = data[indexName] || Object.values(data)[0]
        if (indexMapping && indexMapping.mappings && indexMapping.mappings.properties) {
          fields.value[indexName] = Object.keys(indexMapping.mappings.properties).sort()
        }
      }
    } catch (err) {
      console.error(`Failed to fetch fields for ${indexName}:`, err)
    }
  }

  return {
    indices,
    fields,
    isLoading,
    fetchIndices,
    fetchFields
  }
})
