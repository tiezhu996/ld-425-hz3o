import type { PurchaseStatus } from './enums'
import type { BudgetItem } from './budget'

export interface MaterialItem {
  id: number
  project_id: number
  name: string
  category: string
  spec: string
  brand: string
  quantity: number
  unit: string
  unit_price: number
  total_price: number
  purchase_status: PurchaseStatus
  supplier: string
  space: string
  created_at: string
  updated_at: string
}

export interface CreateMaterialRequest {
  project_id: number
  name: string
  category: string
  spec?: string
  brand?: string
  quantity: number
  unit: string
  unit_price?: number
  supplier?: string
  space?: string
}

// 批量确认交付请求：一批材料从已订货推进到已交付。
export interface DeliverMaterialsRequest {
  ids: number[]
}

// 批量交付入账结果：材料状态与预算实际金额同事务更新后的快照。
export interface DeliverMaterialsResult {
  materials: MaterialItem[]
  budgets: BudgetItem[]
  booked_total: number
}
