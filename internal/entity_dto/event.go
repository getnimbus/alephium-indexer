package entity_dto

import (
	"fmt"
	alephium "github.com/alephium/go-sdk"
)

type Event struct {
	alephium.ContractEventByBlockHash
	DateKey string `json:"dateKey" validate:"-"`
}

func (e *Event) PartitionKey() string {
	return fmt.Sprintf("%v-%v", e.TxId, e.EventIndex)
}

func (e *Event) WithDateKey(date string) *Event {
	if e.DateKey != "" {
		return e
	}
	e.DateKey = date
	return e
}
