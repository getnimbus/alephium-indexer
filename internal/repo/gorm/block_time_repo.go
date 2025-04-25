package gorm

import (
	"context"
	"github.com/google/uuid"
	"time"

	"github.com/golang-module/carbon/v2"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"alephium-indexer/internal/entity"
	"alephium-indexer/internal/repo"
	"alephium-indexer/internal/repo/gorm_scope"
	"alephium-indexer/internal/setting"
)

func NewBlockTimeRepo(
	baseRepo *baseRepo,
	s *gorm_scope.BlockTimeScope,
) repo.BlockTimeRepo {
	return &blockTimeRepo{
		baseRepo: baseRepo,
		s:        s,
	}
}

type blockTimeRepo struct {
	*baseRepo
	s *gorm_scope.BlockTimeScope
}

func (repo *blockTimeRepo) S() *gorm_scope.BlockTimeScope {
	return repo.s
}

func (repo *blockTimeRepo) GetOne(ctx context.Context, scopes ...func(db *gorm.DB) *gorm.DB) (*entity.BlockTime, error) {
	if len(scopes) == 0 {
		return nil, setting.MissingConditionErr
	}

	var row BlockTimeDao
	q := repo.getDB(ctx).Model(&BlockTimeDao{}).
		Scopes(scopes...).
		First(&row)
	if err := q.Error; err != nil {
		return nil, err
	}
	return row.toStruct()
}

func (repo *blockTimeRepo) GetList(ctx context.Context, scopes ...func(db *gorm.DB) *gorm.DB) ([]*entity.BlockTime, error) {
	if len(scopes) == 0 {
		return nil, setting.MissingConditionErr
	}

	var rows []*BlockTimeDao
	q := repo.getDB(ctx).Model(&BlockTimeDao{}).
		Scopes(scopes...).
		Find(&rows)
	if err := q.Error; err != nil {
		return nil, err
	}

	res := make([]*entity.BlockTime, 0, q.RowsAffected)
	for _, row := range rows {
		item, err := row.toStruct()
		if err != nil {
			return nil, err
		}
		res = append(res, item)
	}
	return res, nil
}

func (repo *blockTimeRepo) CreateMany(ctx context.Context, entities ...*entity.BlockTime) error {
	if len(entities) == 0 {
		return nil
	}

	var rows = make([]*BlockTimeDao, 0, len(entities))
	for _, item := range entities {
		row, err := new(BlockTimeDao).fromStruct(item)
		if err != nil {
			return err
		}
		rows = append(rows, row)
	}

	q := repo.getDB(ctx).CreateInBatches(rows, 200)
	return q.Error
}

func (repo *blockTimeRepo) Save(ctx context.Context, entity *entity.BlockTime) error {
	row, err := new(BlockTimeDao).fromStruct(entity)
	if err != nil {
		return err
	}

	q := repo.getDB(ctx).
		Clauses(clause.OnConflict{DoNothing: true}).
		Create(&row)
	return q.Error
}

func (repo *blockTimeRepo) UpdateOne(ctx context.Context, entity *entity.BlockTime) error {
	row, err := new(BlockTimeDao).fromStruct(entity)
	if err != nil {
		return err
	}

	q := repo.getDB(ctx).Updates(&row)
	return q.Error
}

func (repo *blockTimeRepo) UpdateStatus(ctx context.Context, status int, ids ...string) error {
	q := repo.getDB(ctx).
		Model(&BlockTimeDao{}).
		Where("id IN ?", ids).
		Updates(map[string]interface{}{
			"status":     status,
			"updated_at": time.Now(),
		})
	return q.Error
}

func (repo *blockTimeRepo) UpdateFailedStatus(ctx context.Context) error {
	q := repo.getDB(ctx).
		Model(&BlockTimeDao{}).
		Where("status = ? AND updated_at < ?", entity.BlockTime_PROCESSING, carbon.Now().AddHours(-1).ToDateTimeString(carbon.UTC)).
		Update("status", entity.BlockTime_FAIL)
	return q.Error
}

type BlockTimeDao struct {
	Id        string    `gorm:"column:id;type:text;primaryKey"`
	FromTs    int64     `gorm:"column:from_ts;type:bigint;uniqueIndex:blocktime_timestamp_idx"`
	ToTs      int64     `gorm:"column:to_ts;type:bigint;uniqueIndex:blocktime_timestamp_idx"`
	Status    int       `gorm:"column:status;type:int;not null;default:0"`
	Type      int       `gorm:"column:type;type:int;not null;default:0;<-create"`
	CreatedAt time.Time `gorm:"column:created_at;autoCreateTime;<-:create"`
	UpdatedAt time.Time `gorm:"column:updated_at;autoUpdateTime"`
}

func (dao *BlockTimeDao) TableName() string {
	return "alephium_block_time"
}

func (dao *BlockTimeDao) fromStruct(item *entity.BlockTime) (*BlockTimeDao, error) {
	if item.Id != "" {
		dao.Id = item.Id
	} else {
		dao.Id = uuid.NewString()
	}
	dao.FromTs = item.FromTs
	dao.ToTs = item.ToTs
	dao.Status = item.Status
	dao.Type = item.Type
	dao.CreatedAt = item.CreatedAt
	dao.UpdatedAt = item.UpdatedAt
	return dao, nil
}

func (dao *BlockTimeDao) toStruct() (*entity.BlockTime, error) {
	return &entity.BlockTime{
		Id:        dao.Id,
		FromTs:    dao.FromTs,
		ToTs:      dao.ToTs,
		Status:    dao.Status,
		Type:      dao.Type,
		CreatedAt: dao.CreatedAt,
		UpdatedAt: dao.UpdatedAt,
	}, nil
}
