package repository

import (
	"errors"
	"fmt"

	"github.com/home-renovation/platform/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// MaterialRepository 材料清单仓储接口。
type MaterialRepository interface {
	Create(item *model.MaterialItem) error
	GetByID(id uint) (*model.MaterialItem, error)
	List(filter MaterialFilter, page, pageSize int) ([]model.MaterialItem, int64, error)
	ListByProjectID(projectID uint) ([]model.MaterialItem, error)
	Update(item *model.MaterialItem) error
	Delete(id uint) error
	// WithTx 返回绑定到给定事务的仓储实例。
	WithTx(tx *gorm.DB) MaterialRepository
	// Transaction 在数据库事务中执行 fn，fn 返回错误时整体回滚。
	Transaction(fn func(tx *gorm.DB) error) error
	// GetByIDsForUpdate 按 ID 批量查询并加行级排他锁（须在事务内使用）。
	GetByIDsForUpdate(ids []uint) ([]model.MaterialItem, error)
	// UpdateStatus 仅更新采购状态字段（须在事务内使用）。
	UpdateStatus(id uint, status string) error
}

// MaterialFilter 材料查询过滤条件。
type MaterialFilter struct {
	ProjectID uint
	Category  string
	Space     string
}

type materialRepository struct {
	db *gorm.DB
}

// NewMaterialRepository 构造材料仓储。
func NewMaterialRepository(db *gorm.DB) MaterialRepository {
	return &materialRepository{db: db}
}

func (r *materialRepository) Create(item *model.MaterialItem) error {
	if err := r.db.Create(item).Error; err != nil {
		return fmt.Errorf("create material item: %w", err)
	}
	return nil
}

func (r *materialRepository) GetByID(id uint) (*model.MaterialItem, error) {
	var item model.MaterialItem
	if err := r.db.First(&item, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get material item %d: %w", id, err)
	}
	return &item, nil
}

func (r *materialRepository) List(filter MaterialFilter, page, pageSize int) ([]model.MaterialItem, int64, error) {
	var items []model.MaterialItem
	var total int64
	query := r.db.Model(&model.MaterialItem{})
	if filter.ProjectID != 0 {
		query = query.Where("project_id = ?", filter.ProjectID)
	}
	if filter.Category != "" {
		query = query.Where("category = ?", filter.Category)
	}
	if filter.Space != "" {
		query = query.Where("space = ?", filter.Space)
	}
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count material items: %w", err)
	}
	offset := (page - 1) * pageSize
	if err := query.Order("id DESC").Offset(offset).Limit(pageSize).Find(&items).Error; err != nil {
		return nil, 0, fmt.Errorf("list material items: %w", err)
	}
	return items, total, nil
}

func (r *materialRepository) ListByProjectID(projectID uint) ([]model.MaterialItem, error) {
	var items []model.MaterialItem
	if err := r.db.Where("project_id = ?", projectID).Order("id ASC").Find(&items).Error; err != nil {
		return nil, fmt.Errorf("list material items by project %d: %w", projectID, err)
	}
	return items, nil
}

func (r *materialRepository) Update(item *model.MaterialItem) error {
	if err := r.db.Save(item).Error; err != nil {
		return fmt.Errorf("update material item %d: %w", item.ID, err)
	}
	return nil
}

func (r *materialRepository) Delete(id uint) error {
	if err := r.db.Delete(&model.MaterialItem{}, id).Error; err != nil {
		return fmt.Errorf("delete material item %d: %w", id, err)
	}
	return nil
}

func (r *materialRepository) WithTx(tx *gorm.DB) MaterialRepository {
	return &materialRepository{db: tx}
}

func (r *materialRepository) Transaction(fn func(tx *gorm.DB) error) error {
	return r.db.Transaction(fn)
}

func (r *materialRepository) GetByIDsForUpdate(ids []uint) ([]model.MaterialItem, error) {
	var items []model.MaterialItem
	if err := r.db.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id IN ?", ids).
		Order("id ASC").
		Find(&items).Error; err != nil {
		return nil, fmt.Errorf("lock material items for update: %w", err)
	}
	return items, nil
}

func (r *materialRepository) UpdateStatus(id uint, status string) error {
	result := r.db.Model(&model.MaterialItem{}).
		Where("id = ?", id).
		Update("purchase_status", status)
	if result.Error != nil {
		return fmt.Errorf("update material status %d: %w", id, result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}
