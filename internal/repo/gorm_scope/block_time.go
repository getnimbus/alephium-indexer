package gorm_scope

import (
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type BlockTimeScope struct {
	*base
}

func NewBlockTime(b *base) *BlockTimeScope {
	return &BlockTimeScope{base: b}
}

func (s *BlockTimeScope) Locking() func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		return db.Clauses(clause.Locking{
			Strength: "UPDATE",
			Options:  "SKIP LOCKED",
		})
	}
}

func (s *BlockTimeScope) FilterStatuses(statuses ...int) func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		return db.Where("status IN ?", statuses)
	}
}

func (s *BlockTimeScope) FilterType(queryType int) func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		return db.Where("type = ?", queryType)
	}
}
