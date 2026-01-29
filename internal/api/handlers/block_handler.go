package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/thanhnp/chain-apis/internal/storage"
)

// BlockHandler handles block-related API requests
type BlockHandler struct {
	blockStore *storage.MultiChainBlockStore
}

// NewBlockHandler creates a new BlockHandler
func NewBlockHandler(blockStore *storage.MultiChainBlockStore) *BlockHandler {
	return &BlockHandler{
		blockStore: blockStore,
	}
}

// GetByHash returns a block by its hash
// GET /api/v1/:chain/blocks/:hash
func (h *BlockHandler) GetByHash(c *gin.Context) {
	chain := c.Param("chain")
	hash := c.Param("hash")

	block, err := h.blockStore.GetByHash(chain, hash)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if block == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Block not found"})
		return
	}

	c.JSON(http.StatusOK, block)
}

// GetByHeight returns a block by its height
// GET /api/v1/:chain/blocks/height/:height
func (h *BlockHandler) GetByHeight(c *gin.Context) {
	chain := c.Param("chain")
	heightStr := c.Param("height")

	height, err := strconv.ParseInt(heightStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid height"})
		return
	}

	block, err := h.blockStore.GetByHeight(chain, height)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if block == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Block not found"})
		return
	}

	c.JSON(http.StatusOK, block)
}

// GetLatest returns the latest block
// GET /api/v1/:chain/blocks/latest
func (h *BlockHandler) GetLatest(c *gin.Context) {
	chain := c.Param("chain")

	block, err := h.blockStore.GetLatest(chain)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if block == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "No blocks found"})
		return
	}

	c.JSON(http.StatusOK, block)
}
