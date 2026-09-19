package repository

import (
	"errors"
	"fmt"

	"github.com/home-renovation/platform/internal/model"
	"gorm.io/gorm"
)

// BudgetRepository 预算项仓储接口。
type BudgetRepository interface {
	Create(item *model.BudgetItem) error
	GetByID(id uint) (*model.BudgetItem, error)
	List(filter BudgetFilter, page, pageSize int) ([]model.BudgetItem, int64, error)
	ListByProjectID(projectID uint) ([]model.BudgetItem, error)
	Update(item *model.BudgetItem) error
	Delete(id uint) error
	// WithTx 返回绑定到指定事务的仓储，用于跨表原子写入。
	WithTx(tx *gorm.DB) BudgetRepository
	// GetByProjectAndCategory 按项目 + 预算类别查找唯一预算项。
	GetByProjectAndCategory(projectID uint, category string) (*model.BudgetItem, error)
	// ApplyActualDelta 原子地累加实际花费并按同额重算差异（variance 随 actual 同步增减，
	// 因为 budget_amount 不变），避免并发交付时的丢失更新。
	ApplyActualDelta(id uint, delta float64) error
}

// BudgetFilter 预算查询过滤条件。
type BudgetFilter struct {
	ProjectID uint
	Category  string
}

type budgetRepository struct {
	db *gorm.DB
}

// NewBudgetRepository 构造预算仓储。
func NewBudgetRepository(db *gorm.DB) BudgetRepository {
	return &budgetRepository{db: db}
}

func (r *budgetRepository) Create(item *model.BudgetItem) error {
	if err := r.db.Create(item).Error; err != nil {
		return fmt.Errorf("create budget item: %w", err)
	}
	return nil
}

func (r *budgetRepository) GetByID(id uint) (*model.BudgetItem, error) {
	var item model.BudgetItem
	if err := r.db.First(&item, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get budget item %d: %w", id, err)
	}
	return &item, nil
}

func (r *budgetRepository) List(filter BudgetFilter, page, pageSize int) ([]model.BudgetItem, int64, error) {
	var items []model.BudgetItem
	var total int64
	query := r.db.Model(&model.BudgetItem{})
	if filter.ProjectID != 0 {
		query = query.Where("project_id = ?", filter.ProjectID)
	}
	if filter.Category != "" {
		query = query.Where("category = ?", filter.Category)
	}
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count budget items: %w", err)
	}
	offset := (page - 1) * pageSize
	if err := query.Order("id DESC").Offset(offset).Limit(pageSize).Find(&items).Error; err != nil {
		return nil, 0, fmt.Errorf("list budget items: %w", err)
	}
	return items, total, nil
}

func (r *budgetRepository) ListByProjectID(projectID uint) ([]model.BudgetItem, error) {
	var items []model.BudgetItem
	if err := r.db.Where("project_id = ?", projectID).Order("id ASC").Find(&items).Error; err != nil {
		return nil, fmt.Errorf("list budget items by project %d: %w", projectID, err)
	}
	return items, nil
}

func (r *budgetRepository) Update(item *model.BudgetItem) error {
	if err := r.db.Save(item).Error; err != nil {
		return fmt.Errorf("update budget item %d: %w", item.ID, err)
	}
	return nil
}

func (r *budgetRepository) Delete(id uint) error {
	if err := r.db.Delete(&model.BudgetItem{}, id).Error; err != nil {
		return fmt.Errorf("delete budget item %d: %w", id, err)
	}
	return nil
}

func (r *budgetRepository) WithTx(tx *gorm.DB) BudgetRepository {
	return &budgetRepository{db: tx}
}

func (r *budgetRepository) GetByProjectAndCategory(projectID uint, category string) (*model.BudgetItem, error) {
	var item model.BudgetItem
	if err := r.db.Where("project_id = ? AND category = ?", projectID, category).First(&item).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get budget item by project %d category %s: %w", projectID, category, err)
	}
	return &item, nil
}

func (r *budgetRepository) ApplyActualDelta(id uint, delta float64) error {
	result := r.db.Model(&model.BudgetItem{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"actual_amount": gorm.Expr("ROUND(actual_amount + ?, 2)", delta),
			"variance":      gorm.Expr("ROUND(variance + ?, 2)", delta),
		})
	if result.Error != nil {
		return fmt.Errorf("apply actual delta %f to budget item %d: %w", delta, id, result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}
