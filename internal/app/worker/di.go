package worker

import (
	"github.com/getnimbus/ultrago/u_http_client"
	"github.com/google/wire"

	"alephium-indexer/internal/infra"
	"alephium-indexer/internal/repo/gorm"
	"alephium-indexer/internal/repo/gorm_scope"
	"alephium-indexer/internal/service"
)

var deps = wire.NewSet(
	u_http_client.NewHttpExecutor,
	infra.GraphSet,
	infra.NewKafkaSyncProducer,
	infra.NewAwsSession,
	gorm_scope.GraphSet,
	gorm.GraphSet,
	service.GraphSet,
	service.NewS3Service,
)

var GraphSet = wire.NewSet(
	deps,
	NewWorker,
	NewApp,
)
