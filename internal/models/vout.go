package models

// Vout represents a transaction output
type Vout struct {
	TxID         string   `json:"-"` // excluded from API response
	VoutIndex    int      `json:"vout_index"`
	Value        int64    `json:"value"` // in satoshis
	ScriptPubKey string   `json:"script_pubkey"`
	Type         string   `json:"type"` // pubkeyhash, scripthash, etc.
	Addresses    []string `json:"addresses"`
	Spent        bool     `json:"spent"`
	SpentByTxID  string   `json:"spent_by_txid,omitempty"`
	SpentByVin   int      `json:"spent_by_vin,omitempty"`
	Chain        string   `json:"chain"`
}
