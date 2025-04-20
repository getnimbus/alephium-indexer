//go:build wireinject
// +build wireinject

package main

import (
	"context"

	"github.com/google/wire"

	"alephium-indexer/internal/app/worker"
)

func initWorkerApp(ctx context.Context) (worker.App, func(), error) {
	wire.Build(worker.GraphSet)
	return nil, nil, nil
}
