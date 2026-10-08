import { defineStore } from 'pinia'
import { ref } from 'vue'

export const useMetadataStore = defineStore('metadata', () => {
  const indices = ref<string[]>([])
  const fields = ref<Record<string, string[]>>({})
  const isLoading = ref(false)

  async function fetchIndices(clusterId: string) {
    if (!clusterId) return

    isLoading.value = true
    try {
      const response = await fetch(`/api/proxy/_cat/indices?format=json`, {
        headers: { 'X-Cluster-ID': clusterId }
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

  async function fetchFields(clusterId: string, indexName: string) {
    if (!clusterId || fields.value[indexName]) return

    try {
      const response = await fetch(`/api/proxy/${indexName}/_mapping`, {
        headers: { 'X-Cluster-ID': clusterId }
      })
      if (response.ok) {
        const data = await response.json()
        const indexMapping = data[indexName] || Object.values(data)[0] as any
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
