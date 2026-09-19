package service

import (
	"errors"
	"log/slog"
	"testing"

	"github.com/home-renovation/platform/internal/constants"
	"github.com/home-renovation/platform/internal/dto"
	apperrors "github.com/home-renovation/platform/internal/errors"
	"github.com/home-renovation/platform/internal/model"
	"github.com/home-renovation/platform/internal/repository"
	"gorm.io/gorm"
)

// fakeMaterialRepo 测试用内存材料仓储。
type fakeMaterialRepo struct {
	nextID uint
	items  map[uint]*model.MaterialItem
}

func newFakeMaterialRepo() *fakeMaterialRepo {
	return &fakeMaterialRepo{nextID: 1, items: map[uint]*model.MaterialItem{}}
}

func (f *fakeMaterialRepo) Create(item *model.MaterialItem) error {
	item.ID = f.nextID
	f.nextID++
	f.items[item.ID] = item
	return nil
}

func (f *fakeMaterialRepo) GetByID(id uint) (*model.MaterialItem, error) {
	item, ok := f.items[id]
	if !ok {
		return nil, repository.ErrNotFound
	}
	return item, nil
}

func (f *fakeMaterialRepo) List(_ repository.MaterialFilter, _, _ int) ([]model.MaterialItem, int64, error) {
	out := make([]model.MaterialItem, 0, len(f.items))
	for _, item := range f.items {
		out = append(out, *item)
	}
	return out, int64(len(out)), nil
}

func (f *fakeMaterialRepo) ListByProjectID(projectID uint) ([]model.MaterialItem, error) {
	var out []model.MaterialItem
	for _, item := range f.items {
		if item.ProjectID == projectID {
			out = append(out, *item)
		}
	}
	return out, nil
}

func (f *fakeMaterialRepo) Update(item *model.MaterialItem) error {
	if _, ok := f.items[item.ID]; !ok {
		return repository.ErrNotFound
	}
	f.items[item.ID] = item
	return nil
}

func (f *fakeMaterialRepo) Delete(id uint) error {
	delete(f.items, id)
	return nil
}

func (f *fakeMaterialRepo) WithTx(_ *gorm.DB) repository.MaterialRepository { return f }

func (f *fakeMaterialRepo) Transaction(fn func(tx *gorm.DB) error) error { return fn(nil) }

func (f *fakeMaterialRepo) GetByIDsForUpdate(ids []uint) ([]model.MaterialItem, error) {
	out := make([]model.MaterialItem, 0, len(ids))
	for _, id := range ids {
		if item, ok := f.items[id]; ok {
			out = append(out, *item)
		}
	}
	return out, nil
}

func (f *fakeMaterialRepo) UpdateStatus(id uint, status string) error {
	item, ok := f.items[id]
	if !ok {
		return repository.ErrNotFound
	}
	item.PurchaseStatus = status
	return nil
}

// fakeBudgetRepo 测试用内存预算仓储。
type fakeBudgetRepo struct {
	nextID uint
	items  map[uint]*model.BudgetItem
}

func newFakeBudgetRepo() *fakeBudgetRepo {
	return &fakeBudgetRepo{nextID: 1, items: map[uint]*model.BudgetItem{}}
}

func (f *fakeBudgetRepo) Create(item *model.BudgetItem) error {
	item.ID = f.nextID
	f.nextID++
	f.items[item.ID] = item
	return nil
}

func (f *fakeBudgetRepo) GetByID(id uint) (*model.BudgetItem, error) {
	item, ok := f.items[id]
	if !ok {
		return nil, repository.ErrNotFound
	}
	return item, nil
}

func (f *fakeBudgetRepo) List(_ repository.BudgetFilter, _, _ int) ([]model.BudgetItem, int64, error) {
	out := make([]model.BudgetItem, 0, len(f.items))
	for _, item := range f.items {
		out = append(out, *item)
	}
	return out, int64(len(out)), nil
}

func (f *fakeBudgetRepo) ListByProjectID(projectID uint) ([]model.BudgetItem, error) {
	var out []model.BudgetItem
	for _, item := range f.items {
		if item.ProjectID == projectID {
			out = append(out, *item)
		}
	}
	return out, nil
}

func (f *fakeBudgetRepo) Update(item *model.BudgetItem) error {
	if _, ok := f.items[item.ID]; !ok {
		return repository.ErrNotFound
	}
	f.items[item.ID] = item
	return nil
}

func (f *fakeBudgetRepo) Delete(id uint) error {
	delete(f.items, id)
	return nil
}

func (f *fakeBudgetRepo) WithTx(_ *gorm.DB) repository.BudgetRepository { return f }

func (f *fakeBudgetRepo) GetByProjectCategoryForUpdate(projectID uint, category string) (*model.BudgetItem, error) {
	for _, item := range f.items {
		if item.ProjectID == projectID && item.Category == category {
			cp := *item
			return &cp, nil
		}
	}
	return nil, repository.ErrNotFound
}

func newTestMaterialService() (MaterialService, *fakeMaterialRepo, *fakeBudgetRepo) {
	materialRepo := newFakeMaterialRepo()
	budgetRepo := newFakeBudgetRepo()
	return NewMaterialService(materialRepo, budgetRepo, slog.Default()), materialRepo, budgetRepo
}

func seedOrderedMaterial(t *testing.T, svc MaterialService, projectID uint, category string, total float64) *model.MaterialItem {
	t.Helper()
	item, err := svc.Create(&dto.CreateMaterialRequest{
		ProjectID: projectID, Name: "材料", Category: category, Quantity: 1, Unit: "件", UnitPrice: total,
	})
	if err != nil {
		t.Fatalf("create material: %v", err)
	}
	if _, err := svc.UpdateStatus(item.ID, constants.PurchaseStatusOrdered); err != nil {
		t.Fatalf("advance to ordered: %v", err)
	}
	return item
}

func TestMaterialService_DeliverBatchBooksBudget(t *testing.T) {
	svc, _, budgetRepo := newTestMaterialService()
	if err := budgetRepo.Create(&model.BudgetItem{ProjectID: 1, Category: constants.BudgetCategoryMaterial, BudgetAmount: 10000}); err != nil {
		t.Fatalf("create budget: %v", err)
	}
	m1 := seedOrderedMaterial(t, svc, 1, constants.MaterialCategoryTile, 1200)
	m2 := seedOrderedMaterial(t, svc, 1, constants.MaterialCategoryLighting, 800)

	result, err := svc.DeliverBatch([]uint{m1.ID, m2.ID, m1.ID})
	if err != nil {
		t.Fatalf("deliver batch: %v", err)
	}
	if result.BookedTotal != 2000 {
		t.Fatalf("expected booked total 2000, got %v", result.BookedTotal)
	}
	for _, item := range result.Materials {
		if item.PurchaseStatus != constants.PurchaseStatusDelivered {
			t.Fatalf("expected Delivered, got %s", item.PurchaseStatus)
		}
	}
	budget, err := budgetRepo.GetByProjectCategoryForUpdate(1, constants.BudgetCategoryMaterial)
	if err != nil {
		t.Fatalf("get budget: %v", err)
	}
	if budget.ActualAmount != 2000 {
		t.Fatalf("expected actual 2000, got %v", budget.ActualAmount)
	}
	if budget.Variance != -8000 {
		t.Fatalf("expected variance -8000, got %v", budget.Variance)
	}
}

func TestMaterialService_DeliverBatchConflictOnRedeliver(t *testing.T) {
	svc, _, budgetRepo := newTestMaterialService()
	if err := budgetRepo.Create(&model.BudgetItem{ProjectID: 1, Category: constants.BudgetCategoryMaterial, BudgetAmount: 10000}); err != nil {
		t.Fatalf("create budget: %v", err)
	}
	m := seedOrderedMaterial(t, svc, 1, constants.MaterialCategoryFlooring, 1500)
	if _, err := svc.DeliverBatch([]uint{m.ID}); err != nil {
		t.Fatalf("first deliver: %v", err)
	}

	_, err := svc.DeliverBatch([]uint{m.ID})
	var businessErr *apperrors.BusinessError
	if !errors.As(err, &businessErr) || businessErr.HTTP != 409 {
		t.Fatalf("expected 409 conflict on redeliver, got %v", err)
	}
	budget, err := budgetRepo.GetByProjectCategoryForUpdate(1, constants.BudgetCategoryMaterial)
	if err != nil {
		t.Fatalf("get budget: %v", err)
	}
	if budget.ActualAmount != 1500 {
		t.Fatalf("expected actual 1500 (no double booking), got %v", budget.ActualAmount)
	}
}

func TestMaterialService_DeliverBatchRejectsWithoutMatchingBudget(t *testing.T) {
	svc, materialRepo, budgetRepo := newTestMaterialService()
	if err := budgetRepo.Create(&model.BudgetItem{ProjectID: 1, Category: constants.BudgetCategoryMaterial, BudgetAmount: 10000}); err != nil {
		t.Fatalf("create budget: %v", err)
	}
	// “其他”品类映射到 Other 预算类别，项目下不存在该预算项，整批拒绝。
	m := seedOrderedMaterial(t, svc, 1, constants.MaterialCategoryOther, 600)

	if _, err := svc.DeliverBatch([]uint{m.ID}); err == nil {
		t.Fatal("expected rejection when no matching budget item")
	}
	got, err := materialRepo.GetByID(m.ID)
	if err != nil {
		t.Fatalf("get material: %v", err)
	}
	if got.PurchaseStatus != constants.PurchaseStatusOrdered {
		t.Fatalf("material must stay Ordered after rejection, got %s", got.PurchaseStatus)
	}
	budget, err := budgetRepo.GetByProjectCategoryForUpdate(1, constants.BudgetCategoryMaterial)
	if err != nil {
		t.Fatalf("get budget: %v", err)
	}
	if budget.ActualAmount != 0 {
		t.Fatalf("budget must not be partially updated, got actual %v", budget.ActualAmount)
	}
}

func TestMaterialService_UpdateStatusDeliveredBooksOnce(t *testing.T) {
	svc, _, budgetRepo := newTestMaterialService()
	if err := budgetRepo.Create(&model.BudgetItem{ProjectID: 1, Category: constants.BudgetCategoryMaterial, BudgetAmount: 10000}); err != nil {
		t.Fatalf("create budget: %v", err)
	}
	m := seedOrderedMaterial(t, svc, 1, constants.MaterialCategoryPaint, 500)

	if _, err := svc.UpdateStatus(m.ID, constants.PurchaseStatusDelivered); err != nil {
		t.Fatalf("deliver via status: %v", err)
	}
	if _, err := svc.UpdateStatus(m.ID, constants.PurchaseStatusDelivered); err == nil {
		t.Fatal("expected conflict when resubmitting delivered status")
	}
	budget, err := budgetRepo.GetByProjectCategoryForUpdate(1, constants.BudgetCategoryMaterial)
	if err != nil {
		t.Fatalf("get budget: %v", err)
	}
	if budget.ActualAmount != 500 {
		t.Fatalf("expected actual 500 (booked once), got %v", budget.ActualAmount)
	}
}
