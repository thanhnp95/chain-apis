package models

import (
	"time"
)

// Transaction represents a blockchain transaction
type Transaction struct {
	TxID        string    `json:"txid"`
	BlockHash   string    `json:"block_hash"`
	BlockHeight int64     `json:"block_height"`
	Version     int32     `json:"version"`
	LockTime    uint32    `json:"lock_time"`
	Size        int       `json:"size"`
	VSize       int       `json:"vsize"`
	Weight      int       `json:"weight"`
	Fee         int64     `json:"fee"`
	Sent        int64     `json:"sent"`
	IsCoinbase  bool      `json:"is_coinbase"`
	Timestamp   time.Time `json:"timestamp"`
	Chain       string    `json:"chain"`
	NumVin      int       `json:"num_vin"`
	NumVout     int       `json:"num_vout"`
}
