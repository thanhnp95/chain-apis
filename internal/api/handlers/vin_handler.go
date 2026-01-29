package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/thanhnp/chain-apis/internal/storage"
)

// VinHandler handles vin-related API requests
type VinHandler struct {
	vinStore *storage.VinStore
}

// NewVinHandler creates a new VinHandler
func NewVinHandler(vinStore *storage.VinStore) *VinHandler {
	return &VinHandler{
		vinStore: vinStore,
	}
}

// GetByTx returns all vins for a transaction
// GET /api/v1/:chain/transactions/:txid/vins
func (h *VinHandler) GetByTx(c *gin.Context) {
	chain := c.Param("chain")
	txid := c.Param("txid")

	vins, err := h.vinStore.GetByTx(chain, txid)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if len(vins) == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "No vins found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"txid":  txid,
		"count": len(vins),
		"vins":  vins,
	})
}

// Get returns a specific vin by transaction ID and index
// GET /api/v1/:chain/transactions/:txid/vins/:index
func (h *VinHandler) Get(c *gin.Context) {
	chain := c.Param("chain")
	txid := c.Param("txid")
	indexStr := c.Param("index")

	index, err := strconv.Atoi(indexStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid index"})
		return
	}

	vin, err := h.vinStore.Get(chain, txid, index)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if vin == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Vin not found"})
		return
	}

	c.JSON(http.StatusOK, vin)
}
