import { create } from 'zustand'
import type { MaterialItem } from '@/types'
import { PurchaseStatus } from '@/types/enums'
import { listMaterials, updateMaterialStatus } from '@/api/material'
import { useBudgetStore } from '@/stores/budgetStore'

interface MaterialState {
  materials: MaterialItem[]
  loading: boolean
  advancingId: number | null
  fetchMaterials: (projectId?: number) => Promise<void>
  advanceMaterial: (id: number, next: PurchaseStatus) => Promise<MaterialItem>
}

export const useMaterialStore = create<MaterialState>((set, get) => ({
  materials: [],
  loading: false,
  advancingId: null,
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
  // 推进采购状态。交付成功后在本地同步该行，并刷新预算数据，
  // 保证材料页与预算页无需手动刷新即可看到同一笔入账费用。
  advanceMaterial: async (id, next) => {
    if (get().advancingId !== null) {
      throw new Error('another material is advancing')
    }
    set({ advancingId: id })
    try {
      const updated = await updateMaterialStatus(id, next)
      set((state) => ({
        materials: state.materials.map((item) => (item.id === id ? updated : item)),
        advancingId: null,
      }))
      if (next === PurchaseStatus.Delivered) {
        await useBudgetStore.getState().fetchBudgets()
      }
      return updated
    } catch (error) {
      set({ advancingId: null })
      throw error
    }
  },
}))
