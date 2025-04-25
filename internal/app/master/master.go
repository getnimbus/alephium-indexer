package master

import (
	"context"
	"fmt"
	"time"

	"github.com/getnimbus/ultrago/u_logger"
	"github.com/golang-module/carbon/v2"

	"alephium-indexer/internal/entity"
	"alephium-indexer/internal/repo"
	"alephium-indexer/pkg/alert"
)

func NewMaster(
	blockTimeRepo repo.BlockTimeRepo,
) (Master, error) {
	return &master{
		blockTimeRepo: blockTimeRepo,
		cooldown:      60 * time.Second,
	}, nil
}

type master struct {
	blockTimeRepo repo.BlockTimeRepo
	cooldown      time.Duration
	numWorkers    int
}

type Master interface {
	FetchTimeRange(ctx context.Context) error
	Snapshot(ctx context.Context, fromTs int64, toTs int64) error
	Monitor(ctx context.Context) error
}

func (m *master) FetchTimeRange(ctx context.Context) error {
	ctx, logger := u_logger.GetLogger(ctx)

	for {
		select {
		case <-ctx.Done():
			logger.Infof("stop fetching checkpoint!")
			return nil
		default:
			if err := m.fetchBlockTime(ctx); err != nil {
				logger.Errorf("failed to fetch checkpoint: %v", err)
			}
			time.Sleep(m.cooldown)
		}
	}
}

// TODO: implement backfill logic later
func (m *master) Snapshot(ctx context.Context, fromTs int64, toTs int64) error {
	panic("implement me")
}

func (m *master) fetchBlockTime(ctx context.Context) error {
	ctx, logger := u_logger.GetLogger(ctx)
	logger.Infof("start fetch checkpoint...")

	savedBlockTime, err := m.blockTimeRepo.GetList(ctx,
		m.blockTimeRepo.S().SortBy("created_at", "DESC"),
		m.blockTimeRepo.S().FilterType(entity.BlockTimeType_REALTIME),
		m.blockTimeRepo.S().LimitOffset(1, 0),
	)
	if err != nil {
		return err
	}

	// delay 1 mins to confirmed blocks
	now := carbon.Now(carbon.UTC).SubMinutes(1).StartOfMinute()
	toBlockTime := now.ToStdTime().UnixMilli()
	fromBlockTime := now.SubMinutes(1).StartOfMinute().ToStdTime().UnixMilli()
	if len(savedBlockTime) > 0 {
		fromBlockTime = carbon.CreateFromTimestampMilli(savedBlockTime[0].ToTs, carbon.UTC).ToStdTime().UnixMilli()
	}

	var blockTimes = make([]*entity.BlockTime, 0)
	if toBlockTime-fromBlockTime > 1800000 {
		i := fromBlockTime
		for i <= toBlockTime {
			blockTimes = append(blockTimes, &entity.BlockTime{
				FromTs: i + 1,
				ToTs:   i + 60000,
				Status: entity.BlockTime_NOT_READY,
				Type:   entity.BlockTimeType_REALTIME,
			})
			i += 60000 // 1 min
		}
	} else {
		blockTimes = append(blockTimes, &entity.BlockTime{
			FromTs: fromBlockTime + 1,
			ToTs:   toBlockTime,
			Status: entity.BlockTime_NOT_READY,
			Type:   entity.BlockTimeType_REALTIME,
		})
	}
	if len(blockTimes) == 0 {
		return nil
	}

	if err := m.blockTimeRepo.CreateMany(ctx, blockTimes...); err != nil {
		logger.Errorf("failed to save block times: %v", err)
		return fmt.Errorf("failed to save block times: %v", err)
	}
	return nil
}

func (m *master) Monitor(ctx context.Context) error {
	ctx, logger := u_logger.GetLogger(ctx)

	latestProcessed, err := m.blockTimeRepo.GetList(ctx,
		m.blockTimeRepo.S().SortBy("created_at", "DESC"),
		m.blockTimeRepo.S().FilterType(entity.BlockTimeType_REALTIME),
		m.blockTimeRepo.S().FilterStatuses(entity.BlockTime_DONE),
		m.blockTimeRepo.S().LimitOffset(1, 0),
	)
	if err != nil {
		logger.Errorf("failed to latest processed block time: %v", err)
		return err
	} else if len(latestProcessed) == 0 {
		return nil
	}

	var message string
	now := time.Now().UnixMilli()
	delayBlockTime := latestProcessed[0].ToTs - now
	message = fmt.Sprintf("ALPH - %v/%v. Delay %v ms", latestProcessed[0].ToTs, now, delayBlockTime)
	logger.Info(message)
	if delayBlockTime > 43200000 { // delay 12 hours
		message = "@here " + message
	}
	alert.AlertDiscord(ctx, message)

	return nil
}
