package entity_dto

import (
	"encoding/json"
	"fmt"
	alephium "github.com/alephium/go-sdk"
)

type Event struct {
	alephium.ContractEventByBlockHash
	BlockHash string `json:"blockHash" validate:"required"`
	DateKey   string `json:"dateKey" validate:"required"`
	Timestamp int64  `json:"timestamp" validate:"required"`
}

func (e *Event) PartitionKey() string {
	return fmt.Sprintf("%v-%v", e.TxId, e.EventIndex)
}

func (e *Event) WithBlockHash(blockHash string) *Event {
	e.BlockHash = blockHash
	return e
}

func (e *Event) WithDateKey(date string) *Event {
	if e.DateKey != "" {
		return e
	}
	e.DateKey = date
	return e
}

func (e *Event) WithTimestamp(timestamp int64) *Event {
	if e.Timestamp != 0 {
		return e
	}
	e.Timestamp = timestamp
	return e
}

func (e *Event) MarshalJSON() ([]byte, error) {
	toSerialize, err := e.ToMap()
	if err != nil {
		return []byte{}, err
	}
	return json.Marshal(toSerialize)
}

func (e *Event) ToMap() (map[string]interface{}, error) {
	toSerialize := map[string]interface{}{}
	toSerialize["txId"] = e.TxId
	toSerialize["contractAddress"] = e.ContractAddress
	toSerialize["eventIndex"] = e.EventIndex
	toSerialize["fields"] = e.Fields
	toSerialize["blockHash"] = e.BlockHash
	toSerialize["dateKey"] = e.DateKey
	toSerialize["timestamp"] = e.Timestamp
	return toSerialize, nil
}
