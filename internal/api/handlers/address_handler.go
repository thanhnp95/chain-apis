package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/thanhnp/chain-apis/internal/models"
	"github.com/thanhnp/chain-apis/internal/storage"
)

// AddressHandler handles address-related API requests
type AddressHandler struct {
	addressStore *storage.AddressStore
	vinStore     *storage.VinStore
	voutStore    *storage.VoutStore
	txStore      *storage.TxStore
	syncStore    *storage.SyncStore
}

// NewAddressHandler creates a new AddressHandler
func NewAddressHandler(
	addressStore *storage.AddressStore,
	vinStore *storage.VinStore,
	voutStore *storage.VoutStore,
	txStore *storage.TxStore,
	syncStore *storage.SyncStore,
) *AddressHandler {
	return &AddressHandler{
		addressStore: addressStore,
		vinStore:     vinStore,
		voutStore:    voutStore,
		txStore:      txStore,
		syncStore:    syncStore,
	}
}

// Get returns address details
// GET /api/v1/:chain/addresses/:address
func (h *AddressHandler) Get(c *gin.Context) {
	chain := c.Param("chain")
	address := c.Param("address")

	addr, err := h.addressStore.Get(chain, address)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if addr == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Address not found"})
		return
	}

	c.JSON(http.StatusOK, addr)
}

// GetVins returns all vin history for an address
// GET /api/v1/:chain/addresses/:address/vins
func (h *AddressHandler) GetVins(c *gin.Context) {
	chain := c.Param("chain")
	address := c.Param("address")

	refs, err := h.addressStore.GetVinReferences(chain, address)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var vins []*models.Vin
	for _, ref := range refs {
		parts := strings.Split(ref, ":")
		if len(parts) != 2 {
			continue
		}
		txid := parts[0]
		index, err := strconv.Atoi(parts[1])
		if err != nil {
			continue
		}

		vin, err := h.vinStore.Get(chain, txid, index)
		if err != nil || vin == nil {
			continue
		}
		vins = append(vins, vin)
	}

	c.JSON(http.StatusOK, gin.H{
		"address": address,
		"count":   len(vins),
		"vins":    vins,
	})
}

// GetVouts returns all vout history for an address
// GET /api/v1/:chain/addresses/:address/vouts
func (h *AddressHandler) GetVouts(c *gin.Context) {
	chain := c.Param("chain")
	address := c.Param("address")

	refs, err := h.addressStore.GetVoutReferences(chain, address)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var vouts []*models.Vout
	for _, ref := range refs {
		parts := strings.Split(ref, ":")
		if len(parts) != 2 {
			continue
		}
		txid := parts[0]
		index, err := strconv.Atoi(parts[1])
		if err != nil {
			continue
		}

		vout, err := h.voutStore.Get(chain, txid, index)
		if err != nil || vout == nil {
			continue
		}
		vouts = append(vouts, vout)
	}

	c.JSON(http.StatusOK, gin.H{
		"address": address,
		"count":   len(vouts),
		"vouts":   vouts,
	})
}

// TransactionWithConfirmations represents a transaction with confirmations count
type TransactionWithConfirmations struct {
	TxID          string `json:"txid"`
	BlockHash     string `json:"block_hash"`
	BlockHeight   int64  `json:"block_height"`
	Version       int32  `json:"version"`
	LockTime      uint32 `json:"lock_time"`
	Size          int    `json:"size"`
	VSize         int    `json:"vsize"`
	Weight        int    `json:"weight"`
	Fee           int64  `json:"fee"`
	Value         int64  `json:"value"`
	IsCoinbase    bool   `json:"is_coinbase"`
	Timestamp     string `json:"timestamp"`
	Chain         string `json:"chain"`
	Confirmations int64  `json:"confirmations"`
}

// GetTransactions returns all transactions for an address with confirmations
// GET /api/v1/:chain/addresses/:address/transactions
func (h *AddressHandler) GetTransactions(c *gin.Context) {
	chain := c.Param("chain")
	address := c.Param("address")

	// Get current height for confirmations calculation
	currentHeight, _ := h.syncStore.GetSyncedHeight(chain)

	// Get unique transaction IDs from both vins and vouts
	txids := make(map[string]bool)

	vinRefs, err := h.addressStore.GetVinReferences(chain, address)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	for _, ref := range vinRefs {
		parts := strings.Split(ref, ":")
		if len(parts) >= 1 {
			txids[parts[0]] = true
		}
	}

	voutRefs, err := h.addressStore.GetVoutReferences(chain, address)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	for _, ref := range voutRefs {
		parts := strings.Split(ref, ":")
		if len(parts) >= 1 {
			txids[parts[0]] = true
		}
	}

	// Fetch transactions with confirmations
	var txs []TransactionWithConfirmations
	for txid := range txids {
		tx, err := h.txStore.Get(chain, txid)
		if err != nil || tx == nil {
			continue
		}

		var confirmations int64 = 0
		if currentHeight >= 0 && tx.BlockHeight >= 0 {
			confirmations = currentHeight - tx.BlockHeight + 1
		}

		txs = append(txs, TransactionWithConfirmations{
			TxID:          tx.TxID,
			BlockHash:     tx.BlockHash,
			BlockHeight:   tx.BlockHeight,
			Version:       tx.Version,
			LockTime:      tx.LockTime,
			Size:          tx.Size,
			VSize:         tx.VSize,
			Weight:        tx.Weight,
			Fee:           tx.Fee,
			Value:         tx.Sent,
			IsCoinbase:    tx.IsCoinbase,
			Timestamp:     tx.Timestamp.Format("2006-01-02T15:04:05Z"),
			Chain:         tx.Chain,
			Confirmations: confirmations,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"address":      address,
		"count":        len(txs),
		"transactions": txs,
	})
}
