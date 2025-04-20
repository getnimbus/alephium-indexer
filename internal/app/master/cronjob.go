package master

import (
	"context"
	"fmt"
	"time"

	"github.com/getnimbus/ultrago/u_logger"
	"github.com/go-co-op/gocron/v2"
	"github.com/google/uuid"

	"alephium-indexer/internal/repo"
	"alephium-indexer/pkg/alert"
)

func NewCronjob(
	blockTimeRepo repo.BlockTimeRepo,
	master Master,
) Cronjob {
	return &cronjob{
		blockTimeRepo: blockTimeRepo,
		master:        master,
	}
}

type cronjob struct {
	blockTimeRepo repo.BlockTimeRepo
	master        Master
}

type Cronjob interface {
	Start(ctx context.Context) error
}

func (c *cronjob) Start(ctx context.Context) error {
	ctx, logger := u_logger.GetLogger(ctx)

	// create new scheduler
	s, err := gocron.NewScheduler(gocron.WithLocation(time.UTC))
	if err != nil {
		logger.Errorf("failed to create new scheduler: %v", err)
		return fmt.Errorf("failed to create new scheduler: %v", err)
	}
	defer func() { _ = s.Shutdown() }()

	// update checkpoint status PENDING to FAIL if exceed 15 mins
	j1, err := s.NewJob(
		gocron.CronJob(
			"*/15 * * * *",
			false,
		),
		gocron.NewTask(
			func() {
				logger.Info("start update failed block status...")
				if err := c.blockTimeRepo.UpdateFailedStatus(ctx); err != nil {
					logger.Errorf("failed to update block status: %v", err)
					return
				}
				logger.Info("end update failed block status!")
			},
		),
		gocron.WithName("update_failed_status"),
		gocron.WithSingletonMode(gocron.LimitModeReschedule),
		gocron.WithEventListeners(
			gocron.AfterJobRuns(
				func(jobID uuid.UUID, jobName string) {
					logger.Infof("job %s with id %s finished", jobName, jobID)
				},
			),
			gocron.AfterJobRunsWithError(
				func(jobID uuid.UUID, jobName string, err error) {
					errMes := fmt.Sprintf("[alephium-indexer] job %s with id %s failed: %v", jobName, jobID, err)
					logger.Errorf(errMes)
					alert.AlertDiscord(ctx, errMes)
				},
			),
		),
	)
	if err != nil {
		logger.Errorf("failed to registered job %s: %v", j1.Name(), err)
		return fmt.Errorf("failed to registered job %s: %v", j1.Name(), err)
	}

	// monitor status of block process
	j2, err := s.NewJob(
		gocron.CronJob(
			"0 * * * *",
			false,
		),
		gocron.NewTask(
			func() {
				logger.Info("start monitor alephium checkpoint...")
				if err := c.master.Monitor(ctx); err != nil {
					logger.Errorf("failed to monitor alephium checkpoint: %v", err)
					return
				}
				logger.Info("end monitor alephium checkpoint!")
			},
		),
		gocron.WithName("monitor_alephium_checkpoint"),
		gocron.WithSingletonMode(gocron.LimitModeReschedule),
		gocron.WithEventListeners(
			gocron.AfterJobRuns(
				func(jobID uuid.UUID, jobName string) {
					logger.Infof("job %s with id %s finished", jobName, jobID)
				},
			),
			gocron.AfterJobRunsWithError(
				func(jobID uuid.UUID, jobName string, err error) {
					errMes := fmt.Sprintf("[alephium-indexer] job %s with id %s failed: %v", jobName, jobID, err)
					logger.Errorf(errMes)
					alert.AlertDiscord(ctx, errMes)
				},
			),
		),
	)
	if err != nil {
		logger.Errorf("failed to registered job %s: %v", j2.Name(), err)
		return fmt.Errorf("failed to registered job %s: %v", j2.Name(), err)
	}

	s.Start() // non-blocking
	logger.Infof("start cronjob scheduler...")

	timer := time.After(5 * time.Minute)
	for {
		select {
		case <-ctx.Done():
			logger.Infof("stopped cronjob scheduler!")
			return nil
		case <-timer:
			j1LastRun, _ := j1.LastRun()
			j1NextRun, _ := j1.NextRun()
			logger.Infof("job %s last run: %s, next run: %s", j1.Name(), j1LastRun, j1NextRun)

			j2LastRun, _ := j2.LastRun()
			j2NextRun, _ := j2.NextRun()
			logger.Infof("job %s last run: %s, next run: %s", j2.Name(), j2LastRun, j2NextRun)
		}
	}
}
