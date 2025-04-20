package master

import (
	"context"
	"fmt"
	"time"

	"github.com/getnimbus/ultrago/u_logger"
	"github.com/getnimbus/ultrago/u_monitor"
	"golang.org/x/sync/errgroup"
)

func NewApp(
	cronjob Cronjob,
	master Master,
) App {
	return &app{
		cronjob: cronjob,
		master:  master,
	}
}

type App interface {
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
}

type app struct {
	cronjob Cronjob
	master  Master
}

func (a *app) Start(ctx context.Context) error {
	ctx, logger := u_logger.GetLogger(ctx)

	// for monitoring memory
	go u_monitor.Monitor(ctx, 30*time.Second)

	eg, childCtx := errgroup.WithContext(ctx)
	// start cronjob for update failed block status
	eg.Go(func() error {
		if err := a.cronjob.Start(childCtx); err != nil {
			logger.Errorf("failed to start cronjob: %v", err)
			return fmt.Errorf("failed to start cronjob: %v", err)
		}
		return nil
	})

	// fetch latest checkpoint periodically
	eg.Go(func() error {
		if err := a.master.FetchTimeRange(childCtx); err != nil {
			logger.Errorf("failed to fetch checkpoint: %v", err)
			return fmt.Errorf("failed to fetch checkpoint: %v", err)
		}
		return nil
	})

	logger.Info("master started!")
	return eg.Wait()
}

func (a *app) Stop(ctx context.Context) error {
	ctx, logger := u_logger.GetLogger(ctx)
	logger.Infof("master stopped!")
	return nil
}
