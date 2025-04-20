package repo

import (
	"context"

	"gorm.io/gorm"

	"alephium-indexer/internal/entity"
	"alephium-indexer/internal/repo/gorm_scope"
)

type BlockTimeRepo interface {
	S() *gorm_scope.BlockTimeScope
	GetOne(ctx context.Context, scopes ...func(db *gorm.DB) *gorm.DB) (*entity.BlockTime, error)
	GetList(ctx context.Context, scopes ...func(db *gorm.DB) *gorm.DB) ([]*entity.BlockTime, error)
	CreateMany(ctx context.Context, entities ...*entity.BlockTime) error
	Save(ctx context.Context, entity *entity.BlockTime) error
	UpdateOne(ctx context.Context, entity *entity.BlockTime) error
	UpdateStatus(ctx context.Context, status int, ids ...string) error
	UpdateFailedStatus(ctx context.Context) error
}
