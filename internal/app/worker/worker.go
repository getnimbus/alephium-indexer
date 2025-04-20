package worker

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/alitto/pond"
	"github.com/avast/retry-go/v4"
	"github.com/getnimbus/ultrago/u_logger"
	"github.com/hashicorp/golang-lru/v2/expirable"
	"github.com/samber/lo"
	"golang.org/x/sync/errgroup"
	"gorm.io/gorm"

	"alephium-indexer/internal/conf"
	"alephium-indexer/internal/entity"
	"alephium-indexer/internal/entity_dto"
	"alephium-indexer/internal/infra"
	"alephium-indexer/internal/repo"
	"alephium-indexer/internal/service"
)

func NewWorker(
	kafkaProducer infra.KafkaSyncProducer,
	blockTimeRepo repo.BlockTimeRepo,
	baseSvc service.BaseService,
	s3Svc service.S3Service,
) (Worker, error) {
	return &worker{
		kafkaProducer:    kafkaProducer,
		blockTimeRepo:    blockTimeRepo,
		baseSvc:          baseSvc,
		s3Svc:            s3Svc,
		indexer:          service.NewAlephiumIndexer(),
		cache:            expirable.NewLRU[string, bool](1000, nil, 50*time.Second),
		limitCheckpoints: 20, // maximum is 20
		numWorkers:       20,
		cooldown:         2 * time.Second,
		blocksTopic:      conf.Config.AlphBlocksTopic,
		txsTopic:         conf.Config.AlphTxsTopic,
		eventsTopic:      conf.Config.AlphEventsTopic,
	}, nil
}

type worker struct {
	kafkaProducer    infra.KafkaSyncProducer
	blockTimeRepo    repo.BlockTimeRepo
	baseSvc          service.BaseService
	s3Svc            service.S3Service
	indexer          *service.AlephiumIndexer
	cache            *expirable.LRU[string, bool]
	limitCheckpoints int
	numWorkers       int
	cooldown         time.Duration
	blocksTopic      string
	txsTopic         string
	eventsTopic      string
}

type Worker interface {
	FetchTxs(ctx context.Context) error
}

func (w *worker) FetchTxs(ctx context.Context) error {
	ctx, logger := u_logger.GetLogger(ctx)
	logger.Info("start fetching txs...")

	var (
		blocksCh = make(chan []*entity_dto.Block, 300)
		txsCh    = make(chan []*entity_dto.Transaction, 300)
		eventsCh = make(chan []*entity_dto.Event, 300)
	)
	defer func() {
		close(blocksCh)
		close(txsCh)
		close(eventsCh)
	}()

	eg, childCtx := errgroup.WithContext(ctx)
	eg.Go(func() error {
		if err := w.StoreS3(childCtx, blocksCh, txsCh, eventsCh); err != nil {
			logger.Errorf("failed to store: %v", err)
		}
		return fmt.Errorf("stop stored goroutine") // force to stop goroutine
	})

	for i := 0; i < w.numWorkers; i++ {
		eg.Go(func() error {
			for {
				select {
				case <-childCtx.Done():
					logger.Infof("stop fetching txs!")
					return nil
				default:
					w.fetchData(childCtx, blocksCh, txsCh, eventsCh)
					time.Sleep(w.cooldown) // cooldown api
				}
			}
		})
	}

	return eg.Wait()
}

func (w *worker) fetchData(ctx context.Context, blocksCh chan<- []*entity_dto.Block, txsCh chan<- []*entity_dto.Transaction, eventsCh chan<- []*entity_dto.Event) error {
	ctx, logger := u_logger.GetLogger(ctx)
	logger.Info("waiting query from db...")

	var blockStatuses []*entity.BlockTime

	// recover panic
	defer func() {
		if re := recover(); re != nil {
			logger.Infof("panic: %v", re)
			// update status FAIL
			w.updateBlockTimeStatus(ctx, entity.BlockTime_FAIL, blockStatuses...)
			return
		}
	}()

	var dbErr = w.baseSvc.ExecTx(ctx, func(txCtx context.Context) error {
		scopes := []func(db *gorm.DB) *gorm.DB{
			w.blockTimeRepo.S().Locking(),
			w.blockTimeRepo.S().FilterStatuses(
				entity.BlockTime_NOT_READY,
				entity.BlockTime_FAIL,
			),
			w.blockTimeRepo.S().FilterType(entity.BlockTimeType_REALTIME),
			w.blockTimeRepo.S().Limit(w.limitCheckpoints),
		}

		var err error
		blockStatuses, err = w.blockTimeRepo.GetList(txCtx, scopes...)
		if err != nil {
			return err
		}

		// update block status PROCESSING
		return w.updateBlockTimeStatus(txCtx, entity.BlockTime_PROCESSING, blockStatuses...)
	})
	if dbErr != nil {
		logger.Errorf("failed to fetch block status from db: %v", dbErr)
		// update status FAIL
		w.updateBlockTimeStatus(ctx, entity.BlockTime_FAIL, blockStatuses...)
		return fmt.Errorf("failed to fetch block status from db: %v", dbErr)
	} else if len(blockStatuses) == 0 {
		logger.Warnf("not found any new blocks in db")
		return nil
	}

	var (
		blocks = make([]*entity_dto.Block, 0)
		txs    = make([]*entity_dto.Transaction, 0)
		events = make([]*entity_dto.Event, 0)
		wg     sync.WaitGroup
	)
	for _, b := range blockStatuses {
		blockStatus := b
		wg.Add(1)

		go func() {
			defer wg.Done()

			data, err := retry.DoWithData(
				func() (*service.ResponseData, error) {
					return w.indexer.FetchData(ctx, blockStatus.FromTs, blockStatus.ToTs)
				},
				// retry configs
				[]retry.Option{
					retry.Attempts(uint(5)),
					retry.OnRetry(func(n uint, err error) {
						logger.Errorf("Retry invoke function FetchCheckpoint %d to and get error: %v", n+1, err)
					}),
					retry.Delay(3 * time.Second),
					retry.Context(ctx),
				}...,
			)
			if err != nil {
				logger.Errorf("failed to fetch block time from %v: %v", blockStatus.FromTs, blockStatus.ToTs, err)
				// if node rpc has some errors so that it cannot return checkpoints => update status to failed
				// update status FAIL
				w.updateBlockTimeStatus(ctx, entity.BlockTime_FAIL, blockStatus)
				return
			}

			blocks = append(blocks, data.Blocks...)
			txs = append(txs, data.Txs...)
			events = append(events, data.Events...)
		}()
	}

	// wait for all workers to finish
	wg.Wait()

	if len(blocks) == 0 {
		logger.Warnf("no data blocks to process")
		w.updateBlockTimeStatus(ctx, entity.BlockTime_FAIL, blockStatuses...)
		return nil
	}

	eg, childCtx := errgroup.WithContext(ctx)

	// send txs to kafka
	eg.Go(func() error {
		for _, tx := range txs {
			if err := w.kafkaProducer.SendJson(childCtx, w.txsTopic, tx); err != nil {
				logger.Errorf("failed to send payload to kafka alephium txs topic: %v", err)
				return err
			}
		}
		return nil
	})

	// send events to kafka
	eg.Go(func() error {
		for _, event := range events {
			if err := w.kafkaProducer.SendJson(childCtx, w.eventsTopic, event); err != nil {
				logger.Errorf("failed to send payload to kafka alephium events topic: %v", err)
				return err
			}
		}
		return nil
	})

	// send blocks to kafka
	eg.Go(func() error {
		for _, block := range blocks {
			if err := w.kafkaProducer.SendJson(childCtx, w.blocksTopic, block); err != nil {
				logger.Errorf("failed to send payload to kafka alephium blocks topic: %v", err)
				return err
			}
		}
		return nil
	})

	if err := eg.Wait(); err != nil {
		logger.Errorf("failed to fetch data: %v", err)
		// update status FAIL
		w.updateBlockTimeStatus(ctx, entity.BlockTime_FAIL, blockStatuses...)
		return fmt.Errorf("failed to fetch data: %v", err)
	}

	// update status DONE
	w.updateBlockTimeStatus(ctx, entity.BlockTime_DONE, blockStatuses...)

	return nil
}

func (w *worker) updateBlockTimeStatus(ctx context.Context, status int, blockTimes ...*entity.BlockTime) error {
	if len(blockTimes) == 0 {
		return nil
	}
	if err := w.blockTimeRepo.UpdateStatus(ctx, status, lo.Map(blockTimes, func(item *entity.BlockTime, _ int) string {
		return item.Id
	})...); err != nil {
		return err
	}
	return nil
}

func (w *worker) StoreS3(ctx context.Context, blocksCh <-chan []*entity_dto.Block, txsCh <-chan []*entity_dto.Transaction, eventsCh <-chan []*entity_dto.Event) error {
	ctx, logger := u_logger.GetLogger(ctx)
	logger.Info("start goroutine store s3...")

	pool := pond.New(100, 0, pond.MinWorkers(100), pond.Context(ctx))
	defer pool.StopAndWait()

	for {
		select {
		case <-ctx.Done():
			return nil
		case b, ok := <-blocksCh:
			if !ok {
				logger.Info("channel is closed")
				return nil
			}
			blocks := b
			if len(blocks) == 0 {
				continue
			}

			pool.Submit(func() {
				errCh := make(chan error, 1)
				defer close(errCh)

				pw := w.s3Svc.FileStreamWriter(ctx, conf.Config.AwsBucket, fmt.Sprintf("blocks/alephium-blocks/datekey=%v/%v.json.gz", blocks[0].DateKey, blocks[0].PartitionKey()), errCh)
				zw, err := gzip.NewWriterLevel(pw, gzip.BestSpeed)
				if err != nil {
					logger.Errorf("failed to create gzip writer: %v", err)
					return
				}

				// add data to gzip
				for _, block := range blocks {
					data, err := json.Marshal(block)
					if err != nil {
						logger.Errorf("failed to marshal tx: %v", err)
						return
					}
					_, err = zw.Write(data)
					if err != nil {
						logger.Errorf("failed to write tx to S3: %v", err)
						return
					}
					zw.Write([]byte("\n"))
				}

				// flush data to S3
				zw.Close()
				pw.Close()

				returnErr := <-errCh
				if returnErr != nil {
					logger.Errorf("failed to store tx to S3: %v", err)
					return
				}
				logger.Infof("[%v] submit blocks %v to S3 success", blocks[0].DateKey, blocks[0].PartitionKey())
			})
		case t, ok := <-txsCh:
			if !ok {
				logger.Info("channel is closed")
				return nil
			}
			txs := t
			if len(txs) == 0 {
				continue
			}

			pool.Submit(func() {
				errCh := make(chan error, 1)
				defer close(errCh)

				pw := w.s3Svc.FileStreamWriter(ctx, conf.Config.AwsBucket, fmt.Sprintf("txs/alephium-txs/datekey=%v/%v.json.gz", txs[0].DateKey, txs[0].PartitionKey()), errCh)
				zw, err := gzip.NewWriterLevel(pw, gzip.BestSpeed)
				if err != nil {
					logger.Errorf("failed to create gzip writer: %v", err)
					return
				}

				// add data to gzip
				for _, tx := range txs {
					data, err := json.Marshal(tx)
					if err != nil {
						logger.Errorf("failed to marshal tx: %v", err)
						return
					}
					_, err = zw.Write(data)
					if err != nil {
						logger.Errorf("failed to write tx to S3: %v", err)
						return
					}
					zw.Write([]byte("\n"))
				}

				// flush data to S3
				zw.Close()
				pw.Close()

				returnErr := <-errCh
				if returnErr != nil {
					logger.Errorf("failed to store tx to S3: %v", err)
					return
				}
				logger.Infof("[%v] submit txs in block %v to S3 success", txs[0].DateKey, txs[0].BlockHash)
			})
		case e, ok := <-eventsCh:
			if !ok {
				logger.Info("channel is closed")
				return nil
			}
			events := e
			if len(events) == 0 {
				continue
			}

			pool.Submit(func() {
				errCh := make(chan error, 1)
				defer close(errCh)

				pw := w.s3Svc.FileStreamWriter(ctx, conf.Config.AwsBucket, fmt.Sprintf("events/alephium-events/datekey=%v/%v.json.gz", events[0].DateKey, events[0].PartitionKey()), errCh)
				zw, err := gzip.NewWriterLevel(pw, gzip.BestSpeed)
				if err != nil {
					logger.Errorf("failed to create gzip writer: %v", err)
					return
				}

				// add data to gzip
				for _, event := range events {
					data, err := json.Marshal(event)
					if err != nil {
						logger.Errorf("failed to marshal tx: %v", err)
						return
					}
					_, err = zw.Write(data)
					if err != nil {
						logger.Errorf("failed to write tx to S3: %v", err)
						return
					}
					zw.Write([]byte("\n"))
				}

				// flush data to S3
				zw.Close()
				pw.Close()

				returnErr := <-errCh
				if returnErr != nil {
					logger.Errorf("failed to store tx to S3: %v", err)
					return
				}
				logger.Infof("[%v] submit events in tx id %v to S3 success", events[0].DateKey, events[0].TxId)
			})
		}
	}
}
