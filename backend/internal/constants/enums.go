package constants

// DecorStyle 装修风格。
const (
	DecorStyleModern        = "Modern"
	DecorStyleChinese       = "Chinese"
	DecorStyleNordic        = "Nordic"
	DecorStyleJapanese      = "Japanese"
	DecorStyleIndustrial    = "Industrial"
	DecorStyleMediterranean = "Mediterranean"
	DecorStyleMinimalist    = "Minimalist"
)

// PhaseStatus 设计阶段状态。
const (
	PhaseStatusNotStarted = "NotStarted"
	PhaseStatusInProgress = "InProgress"
	PhaseStatusRevision   = "Revision"
	PhaseStatusApproved   = "Approved"
)

// ConstructionPhase 施工节点名称。
const (
	ConstructionPhaseDemolition     = "Demolition"
	ConstructionPhasePlumbing       = "Plumbing"
	ConstructionPhaseCarpentry      = "Carpentry"
	ConstructionPhaseTiling         = "Tiling"
	ConstructionPhasePainting       = "Painting"
	ConstructionPhaseInstallation   = "Installation"
	ConstructionPhaseSoftFurnishing = "SoftFurnishing"
)

// PurchaseStatus 材料采购状态。
const (
	PurchaseStatusNotPurchased = "NotPurchased"
	PurchaseStatusOrdered      = "Ordered"
	PurchaseStatusDelivered    = "Delivered"
	PurchaseStatusInstalled    = "Installed"
)

// BudgetCategory 预算类别。
const (
	BudgetCategoryDesign    = "Design"
	BudgetCategoryMaterial  = "Material"
	BudgetCategoryLabor     = "Labor"
	BudgetCategoryFurniture = "Furniture"
	BudgetCategoryAppliance = "Appliance"
	BudgetCategoryOther     = "Other"
)

// MaterialCategory 材料品类。
const (
	MaterialCategoryTile     = "瓷砖"
	MaterialCategoryFloor    = "地板"
	MaterialCategoryPaint    = "油漆"
	MaterialCategoryLighting = "灯具"
	MaterialCategoryBathroom = "卫浴"
	MaterialCategoryHardware = "五金"
	MaterialCategoryBoard    = "板材"
	MaterialCategoryOther    = "其他"
)

// ProjectStatus 项目状态。
const (
	ProjectStatusDesigning  = "Designing"
	ProjectStatusQuoting    = "Quoting"
	ProjectStatusInProgress = "InProgress"
	ProjectStatusCompleted  = "Completed"
	ProjectStatusArchived   = "Archived"
)

// ConstructionStatus 施工节点状态。
const (
	ConstructionStatusPending    = "Pending"
	ConstructionStatusInProgress = "InProgress"
	ConstructionStatusCompleted  = "Completed"
	ConstructionStatusDelayed    = "Delayed"
)

// AcceptanceStatus 验收状态。
const (
	AcceptanceStatusPending = "Pending"
	AcceptanceStatusPassed  = "Passed"
	AcceptanceStatusFailed  = "Failed"
)

// Role 系统角色。
const (
	RoleAdmin          = "Admin"
	RoleDesigner       = "Designer"
	RoleContractor     = "Contractor"
	RoleOwner          = "Owner"
	RoleProjectManager = "ProjectManager"
)

var (
	DecorStyles = []string{
		DecorStyleModern, DecorStyleChinese, DecorStyleNordic, DecorStyleJapanese,
		DecorStyleIndustrial, DecorStyleMediterranean, DecorStyleMinimalist,
	}
	PhaseStatuses = []string{
		PhaseStatusNotStarted, PhaseStatusInProgress, PhaseStatusRevision, PhaseStatusApproved,
	}
	ConstructionPhases = []string{
		ConstructionPhaseDemolition, ConstructionPhasePlumbing, ConstructionPhaseCarpentry,
		ConstructionPhaseTiling, ConstructionPhasePainting, ConstructionPhaseInstallation,
		ConstructionPhaseSoftFurnishing,
	}
	PurchaseStatuses = []string{
		PurchaseStatusNotPurchased, PurchaseStatusOrdered, PurchaseStatusDelivered, PurchaseStatusInstalled,
	}
	BudgetCategories = []string{
		BudgetCategoryDesign, BudgetCategoryMaterial, BudgetCategoryLabor,
		BudgetCategoryFurniture, BudgetCategoryAppliance, BudgetCategoryOther,
	}
	ProjectStatuses = []string{
		ProjectStatusDesigning, ProjectStatusQuoting, ProjectStatusInProgress, ProjectStatusCompleted, ProjectStatusArchived,
	}
	ConstructionStatuses = []string{
		ConstructionStatusPending, ConstructionStatusInProgress, ConstructionStatusCompleted, ConstructionStatusDelayed,
	}
	AcceptanceStatuses = []string{AcceptanceStatusPending, AcceptanceStatusPassed, AcceptanceStatusFailed}
	Roles              = []string{RoleAdmin, RoleDesigner, RoleContractor, RoleOwner, RoleProjectManager}
)

// Contains 判断字符串是否在集合内。
func Contains(items []string, value string) bool {
	for _, item := range items {
		if item == value {
			return true
		}
	}
	return false
}

// MaterialCategoryBudgetMap 材料品类到预算类别的映射。
// 材料从已订货推进到已交付时，按项目 + 映射后的预算类别归集实际花费。
var MaterialCategoryBudgetMap = map[string]string{
	MaterialCategoryTile:     BudgetCategoryMaterial,
	MaterialCategoryFloor:    BudgetCategoryMaterial,
	MaterialCategoryPaint:    BudgetCategoryMaterial,
	MaterialCategoryLighting: BudgetCategoryMaterial,
	MaterialCategoryBathroom: BudgetCategoryMaterial,
	MaterialCategoryHardware: BudgetCategoryMaterial,
	MaterialCategoryBoard:    BudgetCategoryMaterial,
	MaterialCategoryOther:    BudgetCategoryOther,
}

// PurchaseStatusOrder 采购状态推进顺序，值越大越靠后。
var PurchaseStatusOrder = map[string]int{
	PurchaseStatusNotPurchased: 0,
	PurchaseStatusOrdered:      1,
	PurchaseStatusDelivered:    2,
	PurchaseStatusInstalled:    3,
}

// BudgetCategoryForMaterial 返回材料品类对应的预算类别，未知品类返回 false。
func BudgetCategoryForMaterial(materialCategory string) (string, bool) {
	category, ok := MaterialCategoryBudgetMap[materialCategory]
	return category, ok
}
