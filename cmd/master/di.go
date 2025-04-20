//go:build wireinject
// +build wireinject

package main

import (
	"context"
	"github.com/google/wire"

	"alephium-indexer/internal/app/master"
)

func initMasterApp(ctx context.Context) (master.App, func(), error) {
	wire.Build(master.GraphSet)
	return nil, nil, nil
}
