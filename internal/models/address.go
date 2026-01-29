package models

// Address represents an address with balance information
type Address struct {
	Address       string `json:"address"`
	Balance       int64  `json:"balance"` // in satoshis
	TotalReceived int64  `json:"total_received"`
	TotalSent     int64  `json:"total_sent"`
	TxCount       int    `json:"tx_count"`
	Chain         string `json:"chain"`
}
