package notifier

import (
	"github.com/thanhnp/chain-apis/internal/models"
)

// BlockHandler is called when a new block is connected
type BlockHandler func(block *models.Block, txs []*models.Transaction, vins []*models.Vin, vouts []*models.Vout)

// DisconnectHandler is called when a block is disconnected (reorg)
type DisconnectHandler func(blockHash string, height int64)

// BlockNotifier defines the interface for blockchain notifiers
type BlockNotifier interface {
	// Start starts the notifier and begins listening for block notifications
	Start() error

	// Stop stops the notifier
	Stop() error

	// OnBlockConnected registers a handler for new blocks
	OnBlockConnected(handler BlockHandler)

	// OnBlockDisconnected registers a handler for disconnected blocks (reorgs)
	OnBlockDisconnected(handler DisconnectHandler)

	// GetBlock retrieves a block by hash
	GetBlock(hash string) (*models.Block, []*models.Transaction, []*models.Vin, []*models.Vout, error)

	// GetBlockByHeight retrieves a block by height
	GetBlockByHeight(height int64) (*models.Block, []*models.Transaction, []*models.Vin, []*models.Vout, error)

	// GetCurrentHeight returns the current blockchain height
	GetCurrentHeight() (int64, error)

	// Chain returns the chain identifier ("btc" or "ltc")
	Chain() string
}
