import { create } from 'zustand'
import type { DeliverMaterialsResult, MaterialItem } from '@/types'
import { deliverMaterials, listMaterials } from '@/api/material'

interface MaterialState {
  materials: MaterialItem[]
  loading: boolean
  delivering: boolean
  fetchMaterials: (projectId?: number) => Promise<void>
  deliverBatch: (ids: number[]) => Promise<DeliverMaterialsResult>
}

export const useMaterialStore = create<MaterialState>((set) => ({
  materials: [],
  loading: false,
  delivering: false,
  fetchMaterials: async (projectId) => {
    set({ loading: true })
    try {
      const result = await listMaterials(projectId ? { project_id: projectId } : { page: 1, page_size: 100 })
      if (Array.isArray(result)) {
        set({ materials: result, loading: false })
      } else {
        set({ materials: result.list, loading: false })
      }
    } catch {
      set({ loading: false })
    }
  },
  deliverBatch: async (ids) => {
    set({ delivering: true })
    try {
      const result = await deliverMaterials(ids)
      set({ delivering: false })
      return result
    } catch (error) {
      set({ delivering: false })
      throw error
    }
  },
}))
