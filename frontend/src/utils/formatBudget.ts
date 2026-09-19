// 预算金额格式化和差异计算。
import { PurchaseStatus } from '@/types/enums'
import type { MaterialItem } from '@/types'

export function formatCurrency(value: number | undefined | null): string {
  const num = Number(value ?? 0)
  return num.toLocaleString('zh-CN', {
    style: 'currency',
    currency: 'CNY',
    minimumFractionDigits: 2,
    maximumFractionDigits: 2,
  })
}

export function calcVariance(budget: number, actual: number): number {
  return Math.round((actual - budget) * 100) / 100
}

export function isOverBudget(variance: number): boolean {
  return variance > 0
}

export function budgetProgress(budget: number, actual: number): number {
  if (budget <= 0) return 0
  return Math.min(100, Math.round((actual / budget) * 100))
}

// 已交付（含已安装）材料的入账费用合计。
// 材料页与预算页共用此函数，保证两侧刷新后展示同一笔费用。
export function deliveredMaterialCost(materials: MaterialItem[], projectId?: number): number {
  const total = materials
    .filter(
      (item) =>
        (projectId === undefined || item.project_id === projectId) &&
        (item.purchase_status === PurchaseStatus.Delivered || item.purchase_status === PurchaseStatus.Installed),
    )
    .reduce((sum, item) => sum + item.total_price, 0)
  return Math.round(total * 100) / 100
}
