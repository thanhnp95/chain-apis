package models

// Vin represents a transaction input
type Vin struct {
	TxID        string   `json:"-"` // excluded from API response
	VinIndex    int      `json:"vin_index"`
	PrevTxID    string   `json:"prev_txid"`
	PrevVoutIdx int      `json:"prev_vout_index"`
	ScriptSig   string   `json:"script_sig"`
	Sequence    uint32   `json:"sequence"`
	Witness     []string `json:"witness,omitempty"`
	Chain       string   `json:"chain"`
}
