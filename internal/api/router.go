package api

import (
	"github.com/gin-gonic/gin"

	"github.com/thanhnp/chain-apis/internal/api/handlers"
	"github.com/thanhnp/chain-apis/internal/api/middleware"
	"github.com/thanhnp/chain-apis/internal/storage"
)

// Router wraps the Gin router with handlers
type Router struct {
	engine         *gin.Engine
	blockHandler   *handlers.BlockHandler
	txHandler      *handlers.TxHandler
	vinHandler     *handlers.VinHandler
	voutHandler    *handlers.VoutHandler
	addressHandler *handlers.AddressHandler
}

// NewRouter creates a new Router with all handlers
func NewRouter(
	blockStore *storage.MultiChainBlockStore,
	txStore *storage.MultiChainTxStore,
	vinStore *storage.MultiChainVinStore,
	voutStore *storage.MultiChainVoutStore,
	addressStore *storage.MultiChainAddressStore,
	syncStore *storage.MultiChainSyncStore,
) *Router {
	gin.SetMode(gin.ReleaseMode)

	r := &Router{
		engine:         gin.New(),
		blockHandler:   handlers.NewBlockHandler(blockStore),
		txHandler:      handlers.NewTxHandler(txStore, vinStore, voutStore, syncStore),
		vinHandler:     handlers.NewVinHandler(vinStore),
		voutHandler:    handlers.NewVoutHandler(voutStore),
		addressHandler: handlers.NewAddressHandler(addressStore, vinStore, voutStore, txStore, syncStore),
	}

	r.setupMiddleware()
	r.setupRoutes()

	return r
}

// setupMiddleware configures middleware
func (r *Router) setupMiddleware() {
	r.engine.Use(middleware.Recovery())
	r.engine.Use(middleware.Logger())
	r.engine.Use(middleware.CORS())
}

// setupRoutes configures API routes
func (r *Router) setupRoutes() {
	// Health check
	r.engine.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	// API v1 routes
	v1 := r.engine.Group("/api/v1/:chain")
	v1.Use(middleware.ValidateChain())
	{
		// Block routes
		blocks := v1.Group("/blocks")
		{
			blocks.GET("/latest", r.blockHandler.GetLatest)
			blocks.GET("/height/:height", r.blockHandler.GetByHeight)
			blocks.GET("/:hash", r.blockHandler.GetByHash)
			blocks.GET("/:hash/transactions", r.txHandler.GetByBlock)
		}

		// Transaction routes
		txs := v1.Group("/transactions")
		{
			txs.GET("/:txid", r.txHandler.Get)
			txs.GET("/:txid/vins", r.vinHandler.GetByTx)
			txs.GET("/:txid/vins/:index", r.vinHandler.Get)
			txs.GET("/:txid/vouts", r.voutHandler.GetByTx)
			txs.GET("/:txid/vouts/:index", r.voutHandler.Get)
		}

		// Address routes
		addresses := v1.Group("/addresses")
		{
			addresses.GET("/:address", r.addressHandler.Get)
			addresses.GET("/:address/vins", r.addressHandler.GetVins)
			addresses.GET("/:address/vouts", r.addressHandler.GetVouts)
			addresses.GET("/:address/transactions", r.addressHandler.GetTransactions)
		}
	}
}

// Engine returns the underlying Gin engine
func (r *Router) Engine() *gin.Engine {
	return r.engine
}

// Run starts the HTTP server
func (r *Router) Run(addr string) error {
	return r.engine.Run(addr)
}
