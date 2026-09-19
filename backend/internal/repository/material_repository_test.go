package repository

import (
	"errors"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/home-renovation/platform/internal/model"
	"gorm.io/gorm"
)

func newMaterialBudgetTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.MaterialItem{}, &model.BudgetItem{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func TestMaterialRepository_UpdateStatusIfCurrent(t *testing.T) {
	db := newMaterialBudgetTestDB(t)
	repo := NewMaterialRepository(db)
	item := &model.MaterialItem{ProjectID: 1, Name: "瓷砖", Category: "瓷砖", Unit: "片", PurchaseStatus: "Ordered"}
	if err := repo.Create(item); err != nil {
		t.Fatalf("create: %v", err)
	}

	ok, err := repo.UpdateStatusIfCurrent(item.ID, "Ordered", "Delivered")
	if err != nil {
		t.Fatalf("first advance: %v", err)
	}
	if !ok {
		t.Fatal("expected first advance to take effect")
	}

	// 模拟并发/重复推进：状态已变化，条件更新不得再次生效。
	ok, err = repo.UpdateStatusIfCurrent(item.ID, "Ordered", "Delivered")
	if err != nil {
		t.Fatalf("second advance: %v", err)
	}
	if ok {
		t.Fatal("expected second advance to be rejected by guard")
	}

	got, err := repo.GetByID(item.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.PurchaseStatus != "Delivered" {
		t.Fatalf("expected Delivered, got %s", got.PurchaseStatus)
	}
}

func TestBudgetRepository_GetByProjectAndCategory(t *testing.T) {
	db := newMaterialBudgetTestDB(t)
	repo := NewBudgetRepository(db)
	if err := repo.Create(&model.BudgetItem{ProjectID: 7, Category: "Material", BudgetAmount: 1000}); err != nil {
		t.Fatalf("create: %v", err)
	}

	got, err := repo.GetByProjectAndCategory(7, "Material")
	if err != nil {
		t.Fatalf("get by project and category: %v", err)
	}
	if got.BudgetAmount != 1000 {
		t.Fatalf("expected budget 1000, got %v", got.BudgetAmount)
	}
	if _, err := repo.GetByProjectAndCategory(7, "Labor"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound for missing category, got %v", err)
	}
	if _, err := repo.GetByProjectAndCategory(8, "Material"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound for missing project, got %v", err)
	}
}

func TestBudgetRepository_ApplyActualDelta(t *testing.T) {
	db := newMaterialBudgetTestDB(t)
	repo := NewBudgetRepository(db)
	budget := &model.BudgetItem{ProjectID: 1, Category: "Material", BudgetAmount: 1000, ActualAmount: 100, Variance: -900}
	if err := repo.Create(budget); err != nil {
		t.Fatalf("create: %v", err)
	}

	// 连续两笔交付入账，实际花费与差异同步累加。
	for i := 0; i < 2; i++ {
		if err := repo.ApplyActualDelta(budget.ID, 250); err != nil {
			t.Fatalf("apply delta: %v", err)
		}
	}
	got, err := repo.GetByID(budget.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.ActualAmount != 600 {
		t.Fatalf("expected actual 600, got %v", got.ActualAmount)
	}
	if got.Variance != -400 {
		t.Fatalf("expected variance -400, got %v", got.Variance)
	}

	if err := repo.ApplyActualDelta(9999, 1); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound for missing budget, got %v", err)
	}
}
