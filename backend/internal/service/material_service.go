package service

import (
	"errors"
	"fmt"
	"log/slog"

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
}

type materialService struct {
	repo       repository.MaterialRepository
	budgetRepo repository.BudgetRepository
	tx         repository.Transactor
	logger     *slog.Logger
}

// NewMaterialService 构造材料服务。
func NewMaterialService(repo repository.MaterialRepository, budgetRepo repository.BudgetRepository, tx repository.Transactor, logger *slog.Logger) MaterialService {
	return &materialService{repo: repo, budgetRepo: budgetRepo, tx: tx, logger: logger}
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
	// 推进到已交付会触发预算入账，走独立的事务路径。
	if status == constants.PurchaseStatusDelivered {
		return s.deliver(id)
	}
	item, err := s.GetByID(id)
	if err != nil {
		return nil, err
	}
	if constants.PurchaseStatusOrder[status] < constants.PurchaseStatusOrder[item.PurchaseStatus] {
		return nil, apperrors.NewConflict("purchase status cannot move backwards")
	}
	if constants.PurchaseStatusOrder[status] == constants.PurchaseStatusOrder[item.PurchaseStatus] {
		return nil, apperrors.NewConflict("material already in status " + status)
	}
	item.PurchaseStatus = status
	if err := s.repo.Update(item); err != nil {
		return nil, fmt.Errorf("update material status: %w", err)
	}
	s.logger.Info("material purchase status changed", "material_id", id, "status", status)
	return item, nil
}

// deliver 将材料从已订货推进到已交付，并在同一事务内把材料总价计入
// 对应项目 + 预算类别的实际花费并重算差异。没有匹配预算项时整批回滚，
// 材料与预算都不得部分更新；重复交付或并发推进由条件更新拦截，返回冲突。
func (s *materialService) deliver(id uint) (*model.MaterialItem, error) {
	item, err := s.GetByID(id)
	if err != nil {
		return nil, err
	}
	switch item.PurchaseStatus {
	case constants.PurchaseStatusOrdered:
		// 唯一允许的交付前置状态。
	case constants.PurchaseStatusDelivered, constants.PurchaseStatusInstalled:
		return nil, apperrors.NewConflict("material already delivered, expense already posted to budget")
	default:
		return nil, apperrors.NewConflict("material must be ordered before delivery")
	}
	budgetCategory, ok := constants.BudgetCategoryForMaterial(item.Category)
	if !ok {
		return nil, apperrors.NewBadRequest("no budget category mapped for material category " + item.Category)
	}
	err = s.tx.WithinTransaction(func(tx *gorm.DB) error {
		materialRepo := s.repo.WithTx(tx)
		budgetRepo := s.budgetRepo.WithTx(tx)
		// 原子占位：仅当仍处于已订货才推进；并发或重复提交在此被拦截。
		advanced, err := materialRepo.UpdateStatusIfCurrent(id, constants.PurchaseStatusOrdered, constants.PurchaseStatusDelivered)
		if err != nil {
			return err
		}
		if !advanced {
			return apperrors.NewConflict("material delivery already processed by another request")
		}
		budget, err := budgetRepo.GetByProjectAndCategory(item.ProjectID, budgetCategory)
		if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				return apperrors.NewBadRequest(fmt.Sprintf(
					"no budget item for project %d category %s, delivery rejected", item.ProjectID, budgetCategory))
			}
			return err
		}
		if err := budgetRepo.ApplyActualDelta(budget.ID, item.TotalPrice); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	item.PurchaseStatus = constants.PurchaseStatusDelivered
	s.logger.Info("material delivered and posted to budget",
		"material_id", id, "project_id", item.ProjectID,
		"budget_category", budgetCategory, "posted_amount", item.TotalPrice)
	return item, nil
}
