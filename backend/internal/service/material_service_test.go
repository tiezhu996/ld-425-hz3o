package service

import (
	"errors"
	"log/slog"
	"sync"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/home-renovation/platform/internal/constants"
	"github.com/home-renovation/platform/internal/dto"
	apperrors "github.com/home-renovation/platform/internal/errors"
	"github.com/home-renovation/platform/internal/model"
	"github.com/home-renovation/platform/internal/repository"
	"gorm.io/gorm"
)

// newMaterialTestEnv 使用内存 SQLite 装配真实的仓储与事务，验证交付入账的端到端行为。
func newMaterialTestEnv(t *testing.T) (MaterialService, repository.BudgetRepository) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.MaterialItem{}, &model.BudgetItem{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	budgetRepo := repository.NewBudgetRepository(db)
	svc := NewMaterialService(repository.NewMaterialRepository(db), budgetRepo, repository.NewTransactor(db), slog.Default())
	return svc, budgetRepo
}

func seedOrderedMaterial(t *testing.T, svc MaterialService, projectID uint, category string, qty, price float64) *model.MaterialItem {
	t.Helper()
	item, err := svc.Create(&dto.CreateMaterialRequest{
		ProjectID: projectID, Name: "测试材料", Category: category,
		Quantity: qty, Unit: "件", UnitPrice: price,
	})
	if err != nil {
		t.Fatalf("create material: %v", err)
	}
	if _, err := svc.UpdateStatus(item.ID, constants.PurchaseStatusOrdered); err != nil {
		t.Fatalf("order material: %v", err)
	}
	return item
}

func assertBusinessHTTP(t *testing.T, err error, wantHTTP int) {
	t.Helper()
	var berr *apperrors.BusinessError
	if !errors.As(err, &berr) {
		t.Fatalf("expected BusinessError, got %v", err)
	}
	if berr.HTTP != wantHTTP {
		t.Fatalf("expected http %d, got %d (%s)", wantHTTP, berr.HTTP, berr.Message)
	}
}

func TestMaterialService_DeliverPostsToBudget(t *testing.T) {
	svc, budgetRepo := newMaterialTestEnv(t)
	budget := &model.BudgetItem{ProjectID: 1, Category: constants.BudgetCategoryMaterial, BudgetAmount: 10000, ActualAmount: 500, Variance: -9500}
	if err := budgetRepo.Create(budget); err != nil {
		t.Fatalf("create budget: %v", err)
	}
	item := seedOrderedMaterial(t, svc, 1, constants.MaterialCategoryTile, 10, 100) // 总价 1000

	got, err := svc.UpdateStatus(item.ID, constants.PurchaseStatusDelivered)
	if err != nil {
		t.Fatalf("deliver: %v", err)
	}
	if got.PurchaseStatus != constants.PurchaseStatusDelivered {
		t.Fatalf("expected Delivered, got %s", got.PurchaseStatus)
	}
	updated, err := budgetRepo.GetByID(budget.ID)
	if err != nil {
		t.Fatalf("get budget: %v", err)
	}
	if updated.ActualAmount != 1500 {
		t.Fatalf("expected actual 1500, got %v", updated.ActualAmount)
	}
	if updated.Variance != -8500 {
		t.Fatalf("expected variance -8500, got %v", updated.Variance)
	}
}

func TestMaterialService_DeliverWithoutBudgetRejected(t *testing.T) {
	svc, budgetRepo := newMaterialTestEnv(t)
	item := seedOrderedMaterial(t, svc, 1, constants.MaterialCategoryTile, 10, 100)

	_, err := svc.UpdateStatus(item.ID, constants.PurchaseStatusDelivered)
	if err == nil {
		t.Fatal("expected error when no matching budget item")
	}
	assertBusinessHTTP(t, err, 400)

	// 整批拒绝：材料状态必须保持已订货，预算表不得出现部分写入。
	got, err := svc.GetByID(item.ID)
	if err != nil {
		t.Fatalf("get material: %v", err)
	}
	if got.PurchaseStatus != constants.PurchaseStatusOrdered {
		t.Fatalf("expected status rolled back to Ordered, got %s", got.PurchaseStatus)
	}
	budgets, err := budgetRepo.ListByProjectID(1)
	if err != nil {
		t.Fatalf("list budgets: %v", err)
	}
	if len(budgets) != 0 {
		t.Fatalf("expected no budget rows, got %d", len(budgets))
	}
}

func TestMaterialService_DeliverDuplicateConflict(t *testing.T) {
	svc, budgetRepo := newMaterialTestEnv(t)
	budget := &model.BudgetItem{ProjectID: 1, Category: constants.BudgetCategoryMaterial, BudgetAmount: 10000}
	if err := budgetRepo.Create(budget); err != nil {
		t.Fatalf("create budget: %v", err)
	}
	item := seedOrderedMaterial(t, svc, 1, constants.MaterialCategoryTile, 10, 100)

	if _, err := svc.UpdateStatus(item.ID, constants.PurchaseStatusDelivered); err != nil {
		t.Fatalf("first deliver: %v", err)
	}
	// 重复提交已交付状态：必须冲突且不得重复入账。
	_, err := svc.UpdateStatus(item.ID, constants.PurchaseStatusDelivered)
	if err == nil {
		t.Fatal("expected conflict on duplicate delivery")
	}
	assertBusinessHTTP(t, err, 409)

	updated, err := budgetRepo.GetByID(budget.ID)
	if err != nil {
		t.Fatalf("get budget: %v", err)
	}
	if updated.ActualAmount != 1000 {
		t.Fatalf("expected single posting of 1000, got %v", updated.ActualAmount)
	}
}

func TestMaterialService_DeliverRequiresOrdered(t *testing.T) {
	svc, _ := newMaterialTestEnv(t)
	item, err := svc.Create(&dto.CreateMaterialRequest{
		ProjectID: 1, Name: "未订货材料", Category: constants.MaterialCategoryPaint,
		Quantity: 2, Unit: "桶", UnitPrice: 300,
	})
	if err != nil {
		t.Fatalf("create material: %v", err)
	}
	// 未订货直接交付：违反采购顺序，拒绝且不入账。
	_, err = svc.UpdateStatus(item.ID, constants.PurchaseStatusDelivered)
	if err == nil {
		t.Fatal("expected conflict when skipping Ordered")
	}
	assertBusinessHTTP(t, err, 409)
}

func TestMaterialService_SameAndBackwardsStatusConflict(t *testing.T) {
	svc, _ := newMaterialTestEnv(t)
	item := seedOrderedMaterial(t, svc, 1, constants.MaterialCategoryBoard, 5, 80)

	if _, err := svc.UpdateStatus(item.ID, constants.PurchaseStatusOrdered); err == nil {
		t.Fatal("expected conflict on same-status resubmission")
	} else {
		assertBusinessHTTP(t, err, 409)
	}
	if _, err := svc.UpdateStatus(item.ID, constants.PurchaseStatusNotPurchased); err == nil {
		t.Fatal("expected conflict on backwards transition")
	} else {
		assertBusinessHTTP(t, err, 409)
	}
}

func TestMaterialService_NormalFlowStillWorks(t *testing.T) {
	svc, budgetRepo := newMaterialTestEnv(t)
	budget := &model.BudgetItem{ProjectID: 1, Category: constants.BudgetCategoryOther, BudgetAmount: 5000}
	if err := budgetRepo.Create(budget); err != nil {
		t.Fatalf("create budget: %v", err)
	}
	item, err := svc.Create(&dto.CreateMaterialRequest{
		ProjectID: 1, Name: "其他材料", Category: constants.MaterialCategoryOther,
		Quantity: 1, Unit: "批", UnitPrice: 800,
	})
	if err != nil {
		t.Fatalf("create material: %v", err)
	}
	// 原有采购顺序：未采购 -> 已订货 -> 已交付 -> 已安装。
	if _, err := svc.UpdateStatus(item.ID, constants.PurchaseStatusOrdered); err != nil {
		t.Fatalf("order: %v", err)
	}
	if _, err := svc.UpdateStatus(item.ID, constants.PurchaseStatusDelivered); err != nil {
		t.Fatalf("deliver: %v", err)
	}
	got, err := svc.UpdateStatus(item.ID, constants.PurchaseStatusInstalled)
	if err != nil {
		t.Fatalf("install: %v", err)
	}
	if got.PurchaseStatus != constants.PurchaseStatusInstalled {
		t.Fatalf("expected Installed, got %s", got.PurchaseStatus)
	}
	updated, err := budgetRepo.GetByID(budget.ID)
	if err != nil {
		t.Fatalf("get budget: %v", err)
	}
	if updated.ActualAmount != 800 {
		t.Fatalf("expected actual 800 posted to Other category, got %v", updated.ActualAmount)
	}
}

func TestMaterialService_DeliverConcurrentSinglePosting(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	// 单连接强制并发事务串行化，确定性地模拟两个请求同时推进交付。
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sql db: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	if err := db.AutoMigrate(&model.MaterialItem{}, &model.BudgetItem{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	budgetRepo := repository.NewBudgetRepository(db)
	svc := NewMaterialService(repository.NewMaterialRepository(db), budgetRepo, repository.NewTransactor(db), slog.Default())

	budget := &model.BudgetItem{ProjectID: 1, Category: constants.BudgetCategoryMaterial, BudgetAmount: 10000}
	if err := budgetRepo.Create(budget); err != nil {
		t.Fatalf("create budget: %v", err)
	}
	item := seedOrderedMaterial(t, svc, 1, constants.MaterialCategoryTile, 10, 100)

	const workers = 4
	errs := make([]error, workers)
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_, errs[idx] = svc.UpdateStatus(item.ID, constants.PurchaseStatusDelivered)
		}(i)
	}
	wg.Wait()

	succeeded, conflicts := 0, 0
	for _, err := range errs {
		if err == nil {
			succeeded++
			continue
		}
		var berr *apperrors.BusinessError
		if !errors.As(err, &berr) || berr.HTTP != 409 {
			t.Fatalf("expected 409 conflict for loser, got %v", err)
		}
		conflicts++
	}
	if succeeded != 1 || conflicts != workers-1 {
		t.Fatalf("expected exactly 1 success and %d conflicts, got %d/%d", workers-1, succeeded, conflicts)
	}

	// 并发推进不得重复入账：预算实际花费只累加一次。
	updated, err := budgetRepo.GetByID(budget.ID)
	if err != nil {
		t.Fatalf("get budget: %v", err)
	}
	if updated.ActualAmount != 1000 {
		t.Fatalf("expected single posting of 1000, got %v", updated.ActualAmount)
	}
	got, err := svc.GetByID(item.ID)
	if err != nil {
		t.Fatalf("get material: %v", err)
	}
	if got.PurchaseStatus != constants.PurchaseStatusDelivered {
		t.Fatalf("expected Delivered, got %s", got.PurchaseStatus)
	}
}
