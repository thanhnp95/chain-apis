package notifier

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/ltcsuite/ltcd/btcjson"
	"github.com/ltcsuite/ltcd/chaincfg/chainhash"
	"github.com/ltcsuite/ltcd/ltcutil"
	"github.com/ltcsuite/ltcd/rpcclient"

	"github.com/thanhnp/chain-apis/internal/models"
)

// LtcBlockHeader is used for block notifications
type LtcBlockHeader struct {
	Hash   chainhash.Hash
	Height int32
	Time   time.Time
}

// LTCDNode is an interface to wrap a ltcd rpcclient.Client
type LTCDNode interface {
	NotifyBlocks() error
	NotifyNewTransactions(bool) error
}

// LTCNotifier handles block, tx, and reorg notifications from a ltcd node
type LTCNotifier struct {
	client            *rpcclient.Client
	node              LTCDNode
	anyQ              chan interface{}
	blockHandler      BlockHandler
	disconnectHandler DisconnectHandler
	mu                sync.RWMutex
	running           bool
	cancel            context.CancelFunc
	httpMode          bool          // true for litecoind (HTTP polling), false for ltcd (WebSocket)
	pollInterval      time.Duration // polling interval for HTTP mode
	lastKnownHeight   int64         // last known block height for polling
	previous          struct {
		hash   chainhash.Hash
		height uint32
	}
}

// NewLTCNotifier creates a new Litecoin notifier
func NewLTCNotifier() *LTCNotifier {
	return &LTCNotifier{
		// anyQ can cause deadlocks if it gets full. All mempool transactions pass
		// through here, so the size should stay pretty big.
		anyQ:         make(chan interface{}, 1024),
		pollInterval: 10 * time.Second, // default polling interval
	}
}

// SetHTTPMode enables HTTP polling mode for litecoind
func (n *LTCNotifier) SetHTTPMode(enabled bool, pollIntervalSecs int) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.httpMode = enabled
	if pollIntervalSecs > 0 {
		n.pollInterval = time.Duration(pollIntervalSecs) * time.Second
	}
}

// LtcdHandlers creates a set of handlers to be passed to the ltcd rpcclient.Client
func (n *LTCNotifier) LtcdHandlers() *rpcclient.NotificationHandlers {
	return &rpcclient.NotificationHandlers{
		OnBlockConnected:    n.onBlockConnected,
		OnBlockDisconnected: n.onBlockDisconnected,
		OnTxAcceptedVerbose: n.onTxAcceptedVerbose,
	}
}

// SetClient sets the RPC client
func (n *LTCNotifier) SetClient(client *rpcclient.Client) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.client = client
}

// GetClient returns the RPC client for direct RPC calls
func (n *LTCNotifier) GetClient() *rpcclient.Client {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.client
}

// SetPreviousBlock sets the previous block info
func (n *LTCNotifier) SetPreviousBlock(prevHash chainhash.Hash, prevHeight uint32) {
	n.previous.hash = prevHash
	n.previous.height = prevHeight
}

// Listen starts listening for notifications. Must be called after SetClient.
func (n *LTCNotifier) Listen(ctx context.Context) error {
	n.mu.Lock()
	if n.client == nil {
		n.mu.Unlock()
		return fmt.Errorf("client not set, call SetClient first")
	}
	n.node = n.client
	n.mu.Unlock()

	// Register for block connection notifications
	if err := n.client.NotifyBlocks(); err != nil {
		return fmt.Errorf("block notification registration failed: %w", err)
	}

	// Register for tx accepted into mempool notifications
	if err := n.client.NotifyNewTransactions(true); err != nil {
		return fmt.Errorf("new transaction verbose notification registration failed: %w", err)
	}

	go n.superQueue(ctx)
	return nil
}

// Start starts the notifier (implements BlockNotifier interface)
func (n *LTCNotifier) Start() error {
	n.mu.Lock()
	if n.running {
		n.mu.Unlock()
		return nil
	}

	if n.client == nil {
		n.mu.Unlock()
		return fmt.Errorf("client not set, call SetClient first")
	}

	ctx, cancel := context.WithCancel(context.Background())
	n.cancel = cancel
	n.running = true
	httpMode := n.httpMode
	n.mu.Unlock()

	if httpMode {
		// HTTP mode: use polling
		return n.ListenHTTP(ctx)
	}
	// WebSocket mode: use notifications
	return n.Listen(ctx)
}

// ListenHTTP starts polling for new blocks (for litecoind HTTP mode)
func (n *LTCNotifier) ListenHTTP(ctx context.Context) error {
	n.mu.RLock()
	if n.client == nil {
		n.mu.RUnlock()
		return fmt.Errorf("client not set, call SetClient first")
	}
	pollInterval := n.pollInterval
	n.mu.RUnlock()

	// Get initial height
	height, err := n.client.GetBlockCount()
	if err != nil {
		return fmt.Errorf("failed to get initial block count: %w", err)
	}
	n.mu.Lock()
	n.lastKnownHeight = height
	n.mu.Unlock()

	log.Printf("[LTC] Starting HTTP polling mode with interval %v, current height: %d", pollInterval, height)

	go n.superQueue(ctx)
	go n.pollBlocks(ctx, pollInterval)

	return nil
}

// pollBlocks polls for new blocks periodically
func (n *LTCNotifier) pollBlocks(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("[LTC] Block polling stopped")
			return
		case <-ticker.C:
			n.checkForNewBlocks()
		}
	}
}

// checkForNewBlocks checks if there are new blocks and queues them
func (n *LTCNotifier) checkForNewBlocks() {
	n.mu.RLock()
	client := n.client
	lastHeight := n.lastKnownHeight
	n.mu.RUnlock()

	if client == nil {
		return
	}

	currentHeight, err := client.GetBlockCount()
	if err != nil {
		log.Printf("[LTC] Failed to get block count: %v", err)
		return
	}

	// Process any new blocks
	for height := lastHeight + 1; height <= currentHeight; height++ {
		hash, err := client.GetBlockHash(height)
		if err != nil {
			log.Printf("[LTC] Failed to get block hash for height %d: %v", height, err)
			return
		}

		blockHeader := &LtcBlockHeader{
			Hash:   *hash,
			Height: int32(height),
			Time:   time.Now(),
		}

		log.Printf("[LTC] Poll: New block detected at height %d: %v", height, hash)
		n.anyQ <- blockHeader
	}

	if currentHeight > lastHeight {
		n.mu.Lock()
		n.lastKnownHeight = currentHeight
		n.mu.Unlock()
	}
}

// Stop stops the notifier
func (n *LTCNotifier) Stop() error {
	n.mu.Lock()
	defer n.mu.Unlock()

	if !n.running {
		return nil
	}

	if n.cancel != nil {
		n.cancel()
	}

	if n.client != nil {
		n.client.Shutdown()
	}

	n.running = false
	log.Println("[LTC] Block notifier stopped")
	return nil
}

// superQueue processes notifications from the queue
func (n *LTCNotifier) superQueue(ctx context.Context) {
out:
	for {
		select {
		case rawMsg := <-n.anyQ:
			switch msg := rawMsg.(type) {
			case *LtcBlockHeader:
				log.Printf("[LTC] SuperQueue: Processing new block %v. Height: %d", msg.Hash, msg.Height)
				n.processBlock(msg)
			case *btcjson.TxRawResult:
				n.processTx(msg)
			default:
				log.Printf("[LTC] Warning: unknown message type in superQueue: %T", rawMsg)
			}
		case <-ctx.Done():
			break out
		}
	}
}

// onBlockConnected is called by rpcclient when a new block is connected
func (n *LTCNotifier) onBlockConnected(hash *chainhash.Hash, height int32, t time.Time) {
	blockHeader := &LtcBlockHeader{
		Hash:   *hash,
		Height: height,
		Time:   t,
	}

	log.Printf("[LTC] OnBlockConnected: %d / %v", height, hash)

	n.anyQ <- blockHeader
}

// onBlockDisconnected is called by rpcclient when a block is disconnected
func (n *LTCNotifier) onBlockDisconnected(hash *chainhash.Hash, height int32, t time.Time) {
	log.Printf("[LTC] OnBlockDisconnected: %d / %v", height, hash)

	n.mu.RLock()
	handler := n.disconnectHandler
	n.mu.RUnlock()

	if handler != nil {
		handler(hash.String(), int64(height))
	}
}

// onTxAcceptedVerbose is called when a tx is accepted into mempool
func (n *LTCNotifier) onTxAcceptedVerbose(tx *btcjson.TxRawResult) {
	tx.Time = time.Now().Unix()
	n.anyQ <- tx
}

// processBlock processes a block from the queue
func (n *LTCNotifier) processBlock(bh *LtcBlockHeader) {
	n.mu.RLock()
	handler := n.blockHandler
	client := n.client
	n.mu.RUnlock()

	if handler == nil || client == nil {
		return
	}

	// Get full block data
	block, txs, vins, vouts, err := n.GetBlock(bh.Hash.String())
	if err != nil {
		log.Printf("[LTC] Failed to get block %s: %v", bh.Hash.String(), err)
		return
	}

	handler(block, txs, vins, vouts)
}

// processTx processes a transaction from the queue
func (n *LTCNotifier) processTx(tx *btcjson.TxRawResult) {
	// Currently we don't have tx handlers, but the queue is ready for them
	_ = tx
}

// OnBlockConnected registers a handler for new blocks
func (n *LTCNotifier) OnBlockConnected(handler BlockHandler) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.blockHandler = handler
}

// OnBlockDisconnected registers a handler for disconnected blocks
func (n *LTCNotifier) OnBlockDisconnected(handler DisconnectHandler) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.disconnectHandler = handler
}

// GetBlock retrieves a block by hash using a single GetBlockVerboseTx RPC call
func (n *LTCNotifier) GetBlock(hashStr string) (*models.Block, []*models.Transaction, []*models.Vin, []*models.Vout, error) {
	n.mu.RLock()
	client := n.client
	n.mu.RUnlock()

	if client == nil {
		return nil, nil, nil, nil, fmt.Errorf("client not initialized")
	}

	hash, err := chainhash.NewHashFromStr(hashStr)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("invalid block hash: %w", err)
	}

	// Use GetBlockVerboseTx to get block with all transaction details in one call
	blockVerboseTx, err := client.GetBlockVerboseTx(hash)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("failed to get block verbose tx: %w", err)
	}

	return n.ParseLTCBlockVerboseTx(blockVerboseTx)
}

// ParseLTCBlockVerboseTx parses block data from GetBlockVerboseTx result
func (n *LTCNotifier) ParseLTCBlockVerboseTx(blockVerboseTx *btcjson.GetBlockVerboseTxResult) (*models.Block, []*models.Transaction, []*models.Vin, []*models.Vout, error) {
	block := &models.Block{
		Hash:         blockVerboseTx.Hash,
		Height:       blockVerboseTx.Height,
		Version:      blockVerboseTx.Version,
		PreviousHash: blockVerboseTx.PreviousHash,
		MerkleRoot:   blockVerboseTx.MerkleRoot,
		Timestamp:    time.Unix(blockVerboseTx.Time, 0),
		Bits:         blockVerboseTx.Bits,
		Nonce:        blockVerboseTx.Nonce,
		TxCount:      len(blockVerboseTx.Tx),
		Size:         int(blockVerboseTx.Size),
		Weight:       int(blockVerboseTx.Weight),
		Chain:        "ltc",
	}

	var txs []*models.Transaction
	var vins []*models.Vin
	var vouts []*models.Vout

	for _, rawTx := range blockVerboseTx.Tx {
		// Calculate total sent (sum of all outputs)
		var sent int64
		for _, txout := range rawTx.Vout {
			txAmount, err := ltcutil.NewAmount(txout.Value)
			if err != nil {
				return nil, nil, nil, nil, err
			}
			sent += int64(txAmount)
		}

		// Check if this is a coinbase transaction (first vin has empty Txid)
		isCoinbase := len(rawTx.Vin) > 0 && rawTx.Vin[0].Txid == ""

		txDb := &models.Transaction{
			TxID:        rawTx.Txid,
			BlockHash:   blockVerboseTx.Hash,
			BlockHeight: blockVerboseTx.Height,
			Version:     int32(rawTx.Version),
			LockTime:    rawTx.LockTime,
			Size:        int(rawTx.Size),
			VSize:       int(rawTx.Vsize),
			Weight:      int(rawTx.Weight),
			IsCoinbase:  isCoinbase,
			Sent:        sent,
			Timestamp:   time.Unix(blockVerboseTx.Time, 0),
			NumVin:      len(rawTx.Vin),
			NumVout:     len(rawTx.Vout),
			Chain:       "ltc",
		}

		// Parse vins
		for vinIdx, rawVin := range rawTx.Vin {
			vin := &models.Vin{
				TxID:        rawTx.Txid,
				VinIndex:    vinIdx,
				PrevTxID:    rawVin.Txid,
				PrevVoutIdx: int(rawVin.Vout),
				Sequence:    rawVin.Sequence,
				Chain:       "ltc",
			}
			if rawVin.ScriptSig != nil {
				vin.ScriptSig = rawVin.ScriptSig.Hex
			}
			if len(rawVin.Witness) > 0 {
				vin.Witness = rawVin.Witness
			}
			vins = append(vins, vin)
		}

		// Parse vouts
		for _, rawVout := range rawTx.Vout {
			vout := &models.Vout{
				TxID:         rawTx.Txid,
				VoutIndex:    int(rawVout.N),
				Value:        int64(rawVout.Value * 1e8), // Convert LTC to satoshis
				ScriptPubKey: rawVout.ScriptPubKey.Hex,
				Type:         rawVout.ScriptPubKey.Type,
				Addresses:    rawVout.ScriptPubKey.Addresses,
				Spent:        false,
				Chain:        "ltc",
			}
			vouts = append(vouts, vout)
		}

		txs = append(txs, txDb)
	}

	return block, txs, vins, vouts, nil
}

// GetBlockByHeight retrieves a block by height
func (n *LTCNotifier) GetBlockByHeight(height int64) (*models.Block, []*models.Transaction, []*models.Vin, []*models.Vout, error) {
	n.mu.RLock()
	client := n.client
	n.mu.RUnlock()

	if client == nil {
		return nil, nil, nil, nil, fmt.Errorf("client not initialized")
	}

	hash, err := client.GetBlockHash(height)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("failed to get block hash: %w", err)
	}

	return n.GetBlock(hash.String())
}

// GetCurrentHeight returns the current blockchain height
func (n *LTCNotifier) GetCurrentHeight() (int64, error) {
	n.mu.RLock()
	client := n.client
	n.mu.RUnlock()

	if client == nil {
		return 0, fmt.Errorf("client not initialized")
	}

	return client.GetBlockCount()
}

// Chain returns the chain identifier
func (n *LTCNotifier) Chain() string {
	return "ltc"
}
