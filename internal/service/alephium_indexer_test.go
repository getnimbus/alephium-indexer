package service

import (
	"context"
	"fmt"
	"testing"

	"github.com/smartystreets/goconvey/convey"

	"alephium-indexer/internal/conf"
)

func TestBlockSubscribeService(t *testing.T) {
	ctx := context.Background()

	_ = conf.LoadConfig(".")

	convey.FocusConvey("TestAlephiumIndexerService", t, func() {
		indexer := NewAlephiumIndexer()

		convey.FocusConvey("Fetch onchain data", func() {
			data, err := indexer.FetchData(ctx, 1742205745000, 1742207545000)
			convey.So(err, convey.ShouldBeNil)

			fmt.Println(data.Blocks)
			fmt.Println(data.Txs)
			fmt.Println(data.Events)
		})
	})
}
