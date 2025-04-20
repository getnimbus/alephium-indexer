package entity_dto

import (
	"fmt"
	alephium "github.com/alephium/go-sdk"
	"github.com/getnimbus/ultrago/u_validator"
)

type Transaction struct {
	alephium.Transaction
	BlockHash string `json:"blockHash" validate:"required"`
	DateKey   string `json:"dateKey" validate:"-"`
}

func (tx *Transaction) Validate() error {
	return u_validator.Struct(tx)
}

func (tx *Transaction) PartitionKey() string {
	return fmt.Sprintf("%v", tx.BlockHash)
}

func (tx *Transaction) WithDateKey(dateKey string) *Transaction {
	if tx.DateKey != "" {
		return tx
	}
	tx.DateKey = dateKey
	return tx
}
