package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/thanhnp/chain-apis/internal/storage"
)

const (
	defaultLimit = 50
	maxLimit     = 1000
)

// TxHandler handles transaction-related API requests
type TxHandler struct {
	txStore   *storage.MultiChainTxStore
	vinStore  *storage.MultiChainVinStore
	voutStore *storage.MultiChainVoutStore
	syncStore *storage.MultiChainSyncStore
}

// NewTxHandler creates a new TxHandler
func NewTxHandler(txStore *storage.MultiChainTxStore, vinStore *storage.MultiChainVinStore, voutStore *storage.MultiChainVoutStore, syncStore *storage.MultiChainSyncStore) *TxHandler {
	return &TxHandler{
		txStore:   txStore,
		vinStore:  vinStore,
		voutStore: voutStore,
		syncStore: syncStore,
	}
}

// Get returns a transaction by its ID with vins, vouts, and confirmations
// GET /api/v1/:chain/transactions/:txid
func (h *TxHandler) Get(c *gin.Context) {
	chain := c.Param("chain")
	txid := c.Param("txid")

	tx, err := h.txStore.Get(chain, txid)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if tx == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Transaction not found"})
		return
	}

	// Get vins for this transaction
	vins, err := h.vinStore.GetByTx(chain, txid)
	if err != nil {
		vins = nil // Non-fatal, continue without vins
	}

	// Get vouts for this transaction
	vouts, err := h.voutStore.GetByTx(chain, txid)
	if err != nil {
		vouts = nil // Non-fatal, continue without vouts
	}

	// Calculate confirmations
	var confirmations int64 = 0
	currentHeight, err := h.syncStore.GetSyncedHeight(chain)
	if err == nil && currentHeight >= 0 && tx.BlockHeight >= 0 {
		confirmations = currentHeight - tx.BlockHeight + 1
	}

	c.JSON(http.StatusOK, gin.H{
		"txid":          tx.TxID,
		"block_hash":    tx.BlockHash,
		"block_height":  tx.BlockHeight,
		"version":       tx.Version,
		"lock_time":     tx.LockTime,
		"size":          tx.Size,
		"vsize":         tx.VSize,
		"weight":        tx.Weight,
		"value":         tx.Sent,
		"fee":           tx.Fee,
		"is_coinbase":   tx.IsCoinbase,
		"timestamp":     tx.Timestamp,
		"chain":         tx.Chain,
		"confirmations": confirmations,
		"vins":          vins,
		"vouts":         vouts,
	})
}

// GetByBlock returns transactions in a block with pagination
// GET /api/v1/:chain/blocks/:hash/transactions?offset=0&limit=50
func (h *TxHandler) GetByBlock(c *gin.Context) {
	chain := c.Param("chain")
	blockHash := c.Param("hash")

	// Parse pagination parameters
	offset, limit := parsePaginationParams(c)

	txs, total, err := h.txStore.GetByBlockPaginated(chain, blockHash, offset, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if total == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "No transactions found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"block_hash":   blockHash,
		"total":        total,
		"offset":       offset,
		"limit":        limit,
		"count":        len(txs),
		"transactions": txs,
	})
}

// parsePaginationParams extracts and validates offset and limit from query parameters
func parsePaginationParams(c *gin.Context) (offset, limit int) {
	offset = 0
	limit = defaultLimit

	if offsetStr := c.Query("offset"); offsetStr != "" {
		if v, err := strconv.Atoi(offsetStr); err == nil && v >= 0 {
			offset = v
		}
	}

	if limitStr := c.Query("limit"); limitStr != "" {
		if v, err := strconv.Atoi(limitStr); err == nil && v > 0 {
			limit = v
			if limit > maxLimit {
				limit = maxLimit
			}
		}
	}

	return offset, limit
}
