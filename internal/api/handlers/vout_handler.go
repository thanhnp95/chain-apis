package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/thanhnp/chain-apis/internal/storage"
)

// VoutHandler handles vout-related API requests
type VoutHandler struct {
	voutStore *storage.MultiChainVoutStore
}

// NewVoutHandler creates a new VoutHandler
func NewVoutHandler(voutStore *storage.MultiChainVoutStore) *VoutHandler {
	return &VoutHandler{
		voutStore: voutStore,
	}
}

// GetByTx returns all vouts for a transaction
// GET /api/v1/:chain/transactions/:txid/vouts
func (h *VoutHandler) GetByTx(c *gin.Context) {
	chain := c.Param("chain")
	txid := c.Param("txid")

	vouts, err := h.voutStore.GetByTx(chain, txid)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if len(vouts) == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "No vouts found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"txid":  txid,
		"count": len(vouts),
		"vouts": vouts,
	})
}

// Get returns a specific vout by transaction ID and index
// GET /api/v1/:chain/transactions/:txid/vouts/:index
func (h *VoutHandler) Get(c *gin.Context) {
	chain := c.Param("chain")
	txid := c.Param("txid")
	indexStr := c.Param("index")

	index, err := strconv.Atoi(indexStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid index"})
		return
	}

	vout, err := h.voutStore.Get(chain, txid, index)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if vout == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Vout not found"})
		return
	}

	c.JSON(http.StatusOK, vout)
}
