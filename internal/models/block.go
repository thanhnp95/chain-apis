package models

import (
	"time"
)

// Block represents a blockchain block
type Block struct {
	Hash         string    `json:"hash"`
	Height       int64     `json:"height"`
	Version      int32     `json:"version"`
	PreviousHash string    `json:"previous_hash"`
	MerkleRoot   string    `json:"merkle_root"`
	Timestamp    time.Time `json:"timestamp"`
	Bits         string    `json:"bits"`
	Nonce        uint32    `json:"nonce"`
	TxCount      int       `json:"tx_count"`
	Size         int       `json:"size"`
	Weight       int       `json:"weight"`
	Chain        string    `json:"chain"` // "btc" or "ltc"
}
