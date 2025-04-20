package service

import (
	"context"

	alephium "github.com/alephium/go-sdk"
	"github.com/samber/lo"

	"alephium-indexer/internal/entity_dto"
)

func NewAlephiumIndexer() *AlephiumIndexer {
	configuration := alephium.NewConfiguration()
	configuration.Host = "node.mainnet.alephium.org"
	configuration.Scheme = "https"
	client := alephium.NewAPIClient(configuration)
	return &AlephiumIndexer{
		client: client,
	}
}

type AlephiumIndexer struct {
	client *alephium.APIClient
}

type ResponseData struct {
	Blocks []*entity_dto.Block
	Txs    []*entity_dto.Transaction
	Events []*entity_dto.Event
}

func (svc *AlephiumIndexer) FetchData(ctx context.Context, fromTs int64, toTs int64) (*ResponseData, error) {
	resp, _, err := svc.client.BlockflowApi.GetBlockflowBlocksWithEvents(ctx).FromTs(fromTs).ToTs(toTs).Execute()
	if err != nil {
		return nil, err
	}

	var blocks = make([]*entity_dto.Block, 0)
	var transactions = make([]*entity_dto.Transaction, 0)
	var events = make([]*entity_dto.Event, 0)
	for _, data := range resp.BlocksAndEvents {
		for _, elem := range data {
			// add block
			block := &entity_dto.Block{
				BlockEntry: elem.Block,
			}
			block.WithDateKey()
			blocks = append(blocks, block)

			// add txs
			if block.Transactions != nil && len(block.Transactions) > 0 {
				transactions = append(transactions, lo.Map(block.Transactions, func(tx alephium.Transaction, _ int) *entity_dto.Transaction {
					return &entity_dto.Transaction{
						Transaction: tx,
						BlockHash:   block.Hash,
						DateKey:     block.DateKey,
					}
				})...)
			}

			// add events
			if elem.Events != nil && len(elem.Events) > 0 {
				events = append(events, lo.Map(elem.Events, func(event alephium.ContractEventByBlockHash, _ int) *entity_dto.Event {
					return &entity_dto.Event{
						ContractEventByBlockHash: event,
						DateKey:                  block.DateKey,
					}
				})...)
			}
		}
	}
	return &ResponseData{
		Blocks: blocks,
		Txs:    transactions,
		Events: events,
	}, nil
}
