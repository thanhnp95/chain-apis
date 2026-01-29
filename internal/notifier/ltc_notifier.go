package notifier

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/ltcsuite/ltcd/blockchain"
	"github.com/ltcsuite/ltcd/btcjson"
	"github.com/ltcsuite/ltcd/chaincfg/chainhash"
	"github.com/ltcsuite/ltcd/ltcutil"
	"github.com/ltcsuite/ltcd/rpcclient"
	"github.com/ltcsuite/ltcd/wire"

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
		anyQ: make(chan interface{}, 1024),
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
	n.mu.Unlock()

	return n.Listen(ctx)
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

// GetBlock retrieves a block by hash
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

	blockVerbose, err := client.GetBlockVerbose(hash)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("failed to get block header verbose: %w", err)
	}

	msgBlock, err := client.GetBlock(hash)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	return n.ParseLTCBlockWithTx(blockVerbose, msgBlock)
}

// ParseLTCBlockWithTx is a standalone function to parse ltcd block data with transactions
func (n *LTCNotifier) ParseLTCBlockWithTx(blockVerbose *btcjson.GetBlockVerboseResult, msgBlock *wire.MsgBlock) (*models.Block, []*models.Transaction, []*models.Vin, []*models.Vout, error) {
	block := &models.Block{
		Hash:         blockVerbose.Hash,
		Height:       blockVerbose.Height,
		Version:      blockVerbose.Version,
		PreviousHash: blockVerbose.PreviousHash,
		MerkleRoot:   blockVerbose.MerkleRoot,
		Timestamp:    time.Unix(blockVerbose.Time, 0),
		Bits:         blockVerbose.Bits,
		Nonce:        blockVerbose.Nonce,
		TxCount:      len(msgBlock.Transactions),
		Size:         int(blockVerbose.Size),
		Weight:       int(blockVerbose.Weight),
		Chain:        "ltc",
	}

	var txs []*models.Transaction
	var vins []*models.Vin
	var vouts []*models.Vout

	for idx, tx := range msgBlock.Transactions {
		txhash, err := chainhash.NewHashFromStr(tx.TxHash().String())
		if err != nil {
			return nil, nil, nil, nil, fmt.Errorf("invalid tx hash: %w", err)
		}
		rawTx, err := n.client.GetRawTransactionVerbose(txhash)
		if err != nil {
			return nil, nil, nil, nil, err
		}
		var sent int64
		var inputTotal int64
		for _, txout := range rawTx.Vout {
			txAmount, err := ltcutil.NewAmount(txout.Value)
			if err != nil {
				return nil, nil, nil, nil, err
			}
			sent += int64(txAmount)
		}

		// Check if this is a coinbase transaction
		isCoinbase := blockchain.IsCoinBaseTx(msgBlock.Transactions[idx])

		txDb := &models.Transaction{
			TxID:        rawTx.Txid,
			BlockHash:   blockVerbose.Hash,
			BlockHeight: blockVerbose.Height,
			Version:     int32(rawTx.Version),
			LockTime:    rawTx.LockTime,
			Size:        int(rawTx.Size),
			VSize:       int(rawTx.Vsize),
			Weight:      int(rawTx.Weight),
			IsCoinbase:  isCoinbase,
			Sent:        sent,
			NumVin:      len(tx.TxIn),
			NumVout:     len(tx.TxOut),
			Timestamp:   time.Unix(blockVerbose.Time, 0),
			Chain:       "ltc",
		}

		// Calculate inputTotal and parse vins
		for vinIdx, txin := range tx.TxIn {
			if !isCoinbase {
				// Get transaction by txin to calculate input total
				txInDetail, err := n.client.GetRawTransactionVerbose(&txin.PreviousOutPoint.Hash)
				if err != nil {
					return nil, nil, nil, nil, err
				}
				inAmountCoin := txInDetail.Vout[txin.PreviousOutPoint.Index].Value
				amount, err := ltcutil.NewAmount(inAmountCoin)
				if err != nil {
					return nil, nil, nil, nil, err
				}
				inputTotal += int64(amount)
			}

			rawVin := rawTx.Vin[vinIdx]
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
				VoutIndex:    int(rawVout.N),             // Use actual vout index from RPC, not loop index
				Value:        int64(rawVout.Value * 1e8), // Convert LTC to satoshis
				ScriptPubKey: rawVout.ScriptPubKey.Hex,
				Type:         rawVout.ScriptPubKey.Type,
				Addresses:    rawVout.ScriptPubKey.Addresses,
				Spent:        false,
				Chain:        "ltc",
			}
			vouts = append(vouts, vout)
		}

		if !isCoinbase {
			txDb.Fee = inputTotal - sent
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
