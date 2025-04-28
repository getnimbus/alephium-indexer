package entity_dto

import (
	"encoding/json"
	"fmt"
	alephium "github.com/alephium/go-sdk"
	"github.com/getnimbus/ultrago/u_validator"
)

type Transaction struct {
	alephium.Transaction
	BlockHash string `json:"blockHash" validate:"required"`
	DateKey   string `json:"dateKey" validate:"required"`
	Timestamp int64  `json:"timestamp" validate:"required"`
}

func (tx *Transaction) Validate() error {
	return u_validator.Struct(tx)
}

func (tx *Transaction) PartitionKey() string {
	return fmt.Sprintf("%v", tx.BlockHash)
}

func (tx *Transaction) WithBlockHash(blockHash string) *Transaction {
	tx.BlockHash = blockHash
	return tx
}

func (tx *Transaction) WithDateKey(date string) *Transaction {
	if tx.DateKey != "" {
		return tx
	}
	tx.DateKey = date
	return tx
}

func (tx *Transaction) WithTimestamp(timestamp int64) *Transaction {
	if tx.Timestamp != 0 {
		return tx
	}
	tx.Timestamp = timestamp
	return tx
}

func (tx *Transaction) MarshalJSON() ([]byte, error) {
	toSerialize, err := tx.ToMap()
	if err != nil {
		return []byte{}, err
	}
	return json.Marshal(toSerialize)
}

func (tx *Transaction) ToMap() (map[string]interface{}, error) {
	toSerialize := map[string]interface{}{}
	toSerialize["unsigned"] = tx.Unsigned
	toSerialize["scriptExecutionOk"] = tx.ScriptExecutionOk
	toSerialize["contractInputs"] = tx.ContractInputs
	toSerialize["generatedOutputs"] = tx.GeneratedOutputs
	toSerialize["inputSignatures"] = tx.InputSignatures
	toSerialize["scriptSignatures"] = tx.ScriptSignatures
	toSerialize["blockHash"] = tx.BlockHash
	toSerialize["dateKey"] = tx.DateKey
	toSerialize["timestamp"] = tx.Timestamp
	return toSerialize, nil
}
