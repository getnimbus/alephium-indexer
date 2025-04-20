package entity_dto

import (
	"encoding/json"
	"fmt"
	alephium "github.com/alephium/go-sdk"
	"github.com/getnimbus/ultrago/u_validator"
	"github.com/golang-module/carbon/v2"
)

type Block struct {
	alephium.BlockEntry
	DateKey string `json:"dateKey" validate:"-"`
}

func (b *Block) Validate() error {
	return u_validator.Struct(b)
}

func (b *Block) PartitionKey() string {
	return fmt.Sprintf("%v-%v_%v", b.ChainFrom, b.ChainTo, b.Height)
}

func (b *Block) WithDateKey() *Block {
	if b.DateKey != "" {
		return b
	}
	b.DateKey = carbon.CreateFromTimestampMilli(b.Timestamp, "UTC").ToDateString()
	return b
}

func (b *Block) MarshalJSON() ([]byte, error) {
	type Alias struct {
		Hash         string   `json:"hash"`
		Timestamp    int64    `json:"timestamp"`
		ChainFrom    int32    `json:"chainFrom"`
		ChainTo      int32    `json:"chainTo"`
		Height       int32    `json:"height"`
		Deps         []string `json:"deps"`
		Nonce        string   `json:"nonce"`
		Version      int32    `json:"version"`
		DepStateHash string   `json:"depStateHash"`
		TxsHash      string   `json:"txsHash"`
		Target       string   `json:"target"`
		DateKey      string   `json:"dateKey"`
	}
	return json.Marshal(Alias{
		Hash:         b.Hash,
		Timestamp:    b.Timestamp,
		ChainFrom:    b.ChainFrom,
		ChainTo:      b.ChainTo,
		Height:       b.Height,
		Deps:         b.Deps,
		Nonce:        b.Nonce,
		Version:      b.Version,
		DepStateHash: b.DepStateHash,
		TxsHash:      b.TxsHash,
		Target:       b.Target,
		DateKey:      b.DateKey,
	})
}
