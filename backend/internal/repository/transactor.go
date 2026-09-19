package repository

import "gorm.io/gorm"

// Transactor 在单个数据库事务中执行回调；回调返回错误时整体回滚，
// 保证跨表写入（如材料状态 + 预算实际金额）要么全部提交、要么全部放弃。
type Transactor interface {
	WithinTransaction(fn func(tx *gorm.DB) error) error
}

type transactor struct {
	db *gorm.DB
}

// NewTransactor 构造事务执行器。
func NewTransactor(db *gorm.DB) Transactor {
	return &transactor{db: db}
}

func (t *transactor) WithinTransaction(fn func(tx *gorm.DB) error) error {
	return t.db.Transaction(fn)
}
