package repository

import (
	"errors"
	"fmt"

	"github.com/home-renovation/platform/internal/model"
	"gorm.io/gorm"
)

// MaterialRepository 材料清单仓储接口。
type MaterialRepository interface {
	Create(item *model.MaterialItem) error
	GetByID(id uint) (*model.MaterialItem, error)
	List(filter MaterialFilter, page, pageSize int) ([]model.MaterialItem, int64, error)
	ListByProjectID(projectID uint) ([]model.MaterialItem, error)
	Update(item *model.MaterialItem) error
	Delete(id uint) error
	// WithTx 返回绑定到指定事务的仓储，用于跨表原子写入。
	WithTx(tx *gorm.DB) MaterialRepository
	// UpdateStatusIfCurrent 仅当当前状态为 from 时才推进到 to，
	// 返回是否真正生效；并发或重复提交时生效行数为 0，用于防重复入账。
	UpdateStatusIfCurrent(id uint, from, to string) (bool, error)
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

func (r *materialRepository) UpdateStatusIfCurrent(id uint, from, to string) (bool, error) {
	result := r.db.Model(&model.MaterialItem{}).
		Where("id = ? AND purchase_status = ?", id, from).
		Update("purchase_status", to)
	if result.Error != nil {
		return false, fmt.Errorf("advance material item %d status %s -> %s: %w", id, from, to, result.Error)
	}
	return result.RowsAffected == 1, nil
}
