package router

import (
	"log/slog"
	"testing"

	"github.com/home-renovation/platform/internal/config"
	"github.com/home-renovation/platform/internal/handler"
	"github.com/home-renovation/platform/internal/service"
)

// TestNewRouterRegistersRoutes 验证全部路由注册无冲突（Gin 在路由冲突时会 panic），
// 特别是 POST /materials/deliver 与 /materials/:id 系列路由共存。
func TestNewRouterRegistersRoutes(t *testing.T) {
	logger := slog.Default()
	materialSvc := service.NewMaterialService(nil, nil, logger)
	budgetSvc := service.NewBudgetService(nil, logger)
	deps := Deps{
		Config:   &config.Config{},
		Logger:   logger,
		MaterialH: handler.NewMaterialHandler(materialSvc),
		BudgetH:   handler.NewBudgetHandler(budgetSvc),
	}
	engine := New(deps)
	if engine == nil {
		t.Fatal("expected gin engine")
	}
	var found bool
	for _, route := range engine.Routes() {
		if route.Method == "POST" && route.Path == "/api/v1/materials/deliver" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected POST /api/v1/materials/deliver route")
	}
}
