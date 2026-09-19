package service

import (
	"errors"
	"fmt"
	"log/slog"
	"sort"

	"github.com/home-renovation/platform/internal/constants"
	"github.com/home-renovation/platform/internal/dto"
	apperrors "github.com/home-renovation/platform/internal/errors"
	"github.com/home-renovation/platform/internal/model"
	"github.com/home-renovation/platform/internal/repository"
	"github.com/home-renovation/platform/internal/utils"
	"gorm.io/gorm"
)

// MaterialService 材料服务接口。
type MaterialService interface {
	Create(req *dto.CreateMaterialRequest) (*model.MaterialItem, error)
	GetByID(id uint) (*model.MaterialItem, error)
	List(projectID uint, category, space string, page, pageSize int) ([]model.MaterialItem, int64, error)
	ListByProjectID(projectID uint) ([]model.MaterialItem, error)
	Update(id uint, req *dto.UpdateMaterialRequest) (*model.MaterialItem, error)
	Delete(id uint) error
	UpdateStatus(id uint, status string) (*model.MaterialItem, error)
	// DeliverBatch 批量确认交付：材料推进为已交付并把总价计入对应预算实际金额，整体原子提交。
	DeliverBatch(ids []uint) (*MaterialDeliveryResult, error)
}

// MaterialDeliveryResult 批量交付入账结果。
type MaterialDeliveryResult struct {
	Materials   []model.MaterialItem
	Budgets     []model.BudgetItem
	BookedTotal float64
}

// budgetBookingKey 预算入账键：项目 + 预算类别。
type budgetBookingKey struct {
	projectID uint
	category  string
}

type materialService struct {
	repo       repository.MaterialRepository
	budgetRepo repository.BudgetRepository
	logger     *slog.Logger
}

// NewMaterialService 构造材料服务。
func NewMaterialService(repo repository.MaterialRepository, budgetRepo repository.BudgetRepository, logger *slog.Logger) MaterialService {
	return &materialService{repo: repo, budgetRepo: budgetRepo, logger: logger}
}

func (s *materialService) Create(req *dto.CreateMaterialRequest) (*model.MaterialItem, error) {
	item := &model.MaterialItem{
		ProjectID:      req.ProjectID,
		Name:           req.Name,
		Category:       req.Category,
		Spec:           req.Spec,
		Brand:          req.Brand,
		Quantity:       req.Quantity,
		Unit:           req.Unit,
		UnitPrice:      req.UnitPrice,
		TotalPrice:     utils.MaterialTotal(req.Quantity, req.UnitPrice),
		PurchaseStatus: constants.PurchaseStatusNotPurchased,
		Supplier:       req.Supplier,
		Space:          req.Space,
	}
	if err := s.repo.Create(item); err != nil {
		return nil, fmt.Errorf("create material item: %w", err)
	}
	return item, nil
}

func (s *materialService) GetByID(id uint) (*model.MaterialItem, error) {
	item, err := s.repo.GetByID(id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, apperrors.NewNotFound("material item not found")
		}
		return nil, fmt.Errorf("get material item: %w", err)
	}
	return item, nil
}

func (s *materialService) List(projectID uint, category, space string, page, pageSize int) ([]model.MaterialItem, int64, error) {
	items, total, err := s.repo.List(repository.MaterialFilter{ProjectID: projectID, Category: category, Space: space}, page, pageSize)
	if err != nil {
		return nil, 0, fmt.Errorf("list material items: %w", err)
	}
	return items, total, nil
}

func (s *materialService) ListByProjectID(projectID uint) ([]model.MaterialItem, error) {
	items, err := s.repo.ListByProjectID(projectID)
	if err != nil {
		return nil, fmt.Errorf("list material items by project: %w", err)
	}
	return items, nil
}

func (s *materialService) Update(id uint, req *dto.UpdateMaterialRequest) (*model.MaterialItem, error) {
	item, err := s.GetByID(id)
	if err != nil {
		return nil, err
	}
	if req.Name != nil {
		item.Name = *req.Name
	}
	if req.Category != nil {
		item.Category = *req.Category
	}
	if req.Spec != nil {
		item.Spec = *req.Spec
	}
	if req.Brand != nil {
		item.Brand = *req.Brand
	}
	if req.Quantity != nil {
		item.Quantity = *req.Quantity
	}
	if req.Unit != nil {
		item.Unit = *req.Unit
	}
	if req.UnitPrice != nil {
		item.UnitPrice = *req.UnitPrice
	}
	if req.Supplier != nil {
		item.Supplier = *req.Supplier
	}
	if req.Space != nil {
		item.Space = *req.Space
	}
	item.TotalPrice = utils.MaterialTotal(item.Quantity, item.UnitPrice)
	if err := s.repo.Update(item); err != nil {
		return nil, fmt.Errorf("update material item: %w", err)
	}
	return item, nil
}

func (s *materialService) Delete(id uint) error {
	if _, err := s.GetByID(id); err != nil {
		return err
	}
	if err := s.repo.Delete(id); err != nil {
		return fmt.Errorf("delete material item: %w", err)
	}
	return nil
}

func (s *materialService) UpdateStatus(id uint, status string) (*model.MaterialItem, error) {
	if !constants.Contains(constants.PurchaseStatuses, status) {
		return nil, apperrors.NewBadRequest("invalid purchase status")
	}
	if status == constants.PurchaseStatusDelivered {
		// 推进到已交付必须走交付入账流程：校验当前为已订货，材料与预算在同一事务内更新。
		result, err := s.DeliverBatch([]uint{id})
		if err != nil {
			return nil, err
		}
		return &result.Materials[0], nil
	}
	item, err := s.GetByID(id)
	if err != nil {
		return nil, err
	}
	order := map[string]int{
		constants.PurchaseStatusNotPurchased: 0,
		constants.PurchaseStatusOrdered:      1,
		constants.PurchaseStatusDelivered:    2,
		constants.PurchaseStatusInstalled:    3,
	}
	if order[status] < order[item.PurchaseStatus] {
		return nil, apperrors.NewConflict("purchase status cannot move backwards")
	}
	item.PurchaseStatus = status
	if err := s.repo.Update(item); err != nil {
		return nil, fmt.Errorf("update material status: %w", err)
	}
	s.logger.Info("material purchase status changed", "material_id", id, "status", status)
	return item, nil
}

func (s *materialService) DeliverBatch(ids []uint) (*MaterialDeliveryResult, error) {
	uniqueIDs := dedupeUints(ids)
	if len(uniqueIDs) == 0 {
		return nil, apperrors.NewBadRequest("material ids required")
	}
	result := &MaterialDeliveryResult{}
	err := s.repo.Transaction(func(tx *gorm.DB) error {
		materialRepo := s.repo.WithTx(tx)
		budgetRepo := s.budgetRepo.WithTx(tx)

		// 行级排他锁锁定本批材料，并发推进在此串行化。
		items, err := materialRepo.GetByIDsForUpdate(uniqueIDs)
		if err != nil {
			return fmt.Errorf("lock materials for delivery: %w", err)
		}
		if len(items) != len(uniqueIDs) {
			return apperrors.NewNotFound("some materials not found, delivery batch rejected")
		}

		// 校验整批材料并汇总每个预算项的入账金额；任一不满足则整批拒绝。
		deltas := make(map[budgetBookingKey]float64)
		for i := range items {
			item := &items[i]
			if item.PurchaseStatus != constants.PurchaseStatusOrdered {
				return apperrors.NewConflict(fmt.Sprintf("material %d is not in Ordered status, delivery rejected", item.ID))
			}
			budgetCategory, ok := constants.BudgetCategoryForMaterial(item.Category)
			if !ok {
				return apperrors.NewBadRequest(fmt.Sprintf("material category %s has no budget category mapping", item.Category))
			}
			key := budgetBookingKey{projectID: item.ProjectID, category: budgetCategory}
			deltas[key] = utils.Round2(deltas[key] + item.TotalPrice)
			item.PurchaseStatus = constants.PurchaseStatusDelivered
		}

		// 按固定顺序锁定并更新预算项，避免并发批次互相死锁。
		keys := make([]budgetBookingKey, 0, len(deltas))
		for key := range deltas {
			keys = append(keys, key)
		}
		sort.Slice(keys, func(i, j int) bool {
			if keys[i].projectID != keys[j].projectID {
				return keys[i].projectID < keys[j].projectID
			}
			return keys[i].category < keys[j].category
		})
		booked := 0.0
		updatedBudgets := make([]model.BudgetItem, 0, len(keys))
		for _, key := range keys {
			budget, err := budgetRepo.GetByProjectCategoryForUpdate(key.projectID, key.category)
			if err != nil {
				if errors.Is(err, repository.ErrNotFound) {
					return apperrors.NewBadRequest(fmt.Sprintf("no matching budget item for project %d category %s, delivery batch rejected", key.projectID, key.category))
				}
				return fmt.Errorf("lock budget item for booking: %w", err)
			}
			budget.ActualAmount = utils.Round2(budget.ActualAmount + deltas[key])
			budget.Variance = utils.BudgetVariance(budget.BudgetAmount, budget.ActualAmount)
			if err := budgetRepo.Update(budget); err != nil {
				return fmt.Errorf("update budget actual amount: %w", err)
			}
			booked = utils.Round2(booked + deltas[key])
			updatedBudgets = append(updatedBudgets, *budget)
		}

		// 预算入账成功后才推进材料状态，同事务提交或整体回滚。
		for i := range items {
			if err := materialRepo.UpdateStatus(items[i].ID, constants.PurchaseStatusDelivered); err != nil {
				return fmt.Errorf("mark material delivered: %w", err)
			}
		}
		result.Materials = items
		result.Budgets = updatedBudgets
		result.BookedTotal = booked
		return nil
	})
	if err != nil {
		return nil, err
	}
	s.logger.Info("materials delivered and booked", "material_ids", uniqueIDs, "booked_total", result.BookedTotal)
	return result, nil
}
