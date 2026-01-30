package sync

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	btcrpcclient "github.com/btcsuite/btcd/rpcclient"
	ltcrpcclient "github.com/ltcsuite/ltcd/rpcclient"
	"github.com/thanhnp/chain-apis/internal/models"
	"github.com/thanhnp/chain-apis/internal/notifier"
	"github.com/thanhnp/chain-apis/internal/storage"
)

// pendingBlock holds block data waiting to be processed
type pendingBlock struct {
	block *models.Block
	txs   []*models.Transaction
	vins  []*models.Vin
	vouts []*models.Vout
}

// SyncCheckpointInterval is the number of blocks between sync checkpoints during historical sync
const SyncCheckpointInterval = 300

// Syncer handles historical block synchronization
type Syncer struct {
	notifier       notifier.BlockNotifier
	btcClient      *btcrpcclient.Client
	ltcClient      *ltcrpcclient.Client
	db             *storage.PebbleDB // direct db reference for sync control
	blockStore     *storage.BlockStore
	txStore        *storage.TxStore
	vinStore       *storage.VinStore
	voutStore      *storage.VoutStore
	addressStore   *storage.AddressStore
	syncStore      *storage.SyncStore
	startHeight    int64
	chain          string
	mu             sync.RWMutex
	syncing        bool
	historicalDone bool            // true when historical sync is complete
	pendingBlocks  []*pendingBlock // blocks received during historical sync
	cancel         context.CancelFunc
}

// NewSyncer creates a new Syncer
func NewSyncer(
	n notifier.BlockNotifier,
	db *storage.PebbleDB,
	blockStore *storage.BlockStore,
	txStore *storage.TxStore,
	vinStore *storage.VinStore,
	voutStore *storage.VoutStore,
	addressStore *storage.AddressStore,
	syncStore *storage.SyncStore,
	startHeight int64,
) *Syncer {
	return &Syncer{
		notifier:     n,
		db:           db,
		blockStore:   blockStore,
		txStore:      txStore,
		vinStore:     vinStore,
		voutStore:    voutStore,
		addressStore: addressStore,
		syncStore:    syncStore,
		startHeight:  startHeight,
		chain:        n.Chain(),
	}
}

// SetClient sets the BTC RPC client for direct RPC calls
func (s *Syncer) SetClient(client *btcrpcclient.Client) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.btcClient = client
}

// GetClient returns the BTC RPC client for direct RPC calls
func (s *Syncer) GetClient() *btcrpcclient.Client {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.btcClient
}

// SetLTCClient sets the LTC RPC client for direct RPC calls
func (s *Syncer) SetLTCClient(client *ltcrpcclient.Client) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ltcClient = client
}

// GetLTCClient returns the LTC RPC client for direct RPC calls
func (s *Syncer) GetLTCClient() *ltcrpcclient.Client {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.ltcClient
}

// Start begins the synchronization process
func (s *Syncer) Start(ctx context.Context) error {
	s.mu.Lock()
	if s.syncing {
		s.mu.Unlock()
		return nil
	}
	s.syncing = true
	ctx, s.cancel = context.WithCancel(ctx)
	s.mu.Unlock()

	// Register block handlers
	s.notifier.OnBlockConnected(s.handleBlockConnected)
	s.notifier.OnBlockDisconnected(s.handleBlockDisconnected)

	// Start the notifier for real-time updates
	if err := s.notifier.Start(); err != nil {
		return fmt.Errorf("failed to start notifier: %w", err)
	}

	// Run historical sync in background
	go s.syncHistorical(ctx)

	return nil
}

// Stop stops the synchronization
func (s *Syncer) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.syncing {
		return nil
	}

	if s.cancel != nil {
		s.cancel()
	}

	s.syncing = false
	return s.notifier.Stop()
}

// syncHistorical synchronizes historical blocks
func (s *Syncer) syncHistorical(ctx context.Context) {
	log.Printf("[%s] Starting historical sync", s.chain)

	// Get last synced height
	lastSynced, err := s.syncStore.GetSyncedHeight(s.chain)
	if err != nil {
		log.Printf("[%s] Failed to get last synced height: %v", s.chain, err)
		return
	}

	// Determine starting point
	startHeight := s.startHeight
	if lastSynced >= startHeight {
		startHeight = lastSynced + 1
	}

	// Get current blockchain height
	currentHeight, err := s.notifier.GetCurrentHeight()
	if err != nil {
		log.Printf("[%s] Failed to get current height: %v", s.chain, err)
		return
	}

	if startHeight > currentHeight {
		log.Printf("[%s] Already synced to height %d, current height %d", s.chain, lastSynced, currentHeight)
		return
	}

	log.Printf("[%s] Syncing from height %d to %d", s.chain, startHeight, currentHeight)

	// Enable historical sync mode for faster writes (NoSync)
	s.db.SetHistoricalSyncMode(true)
	log.Printf("[%s] Historical sync mode enabled (NoSync, checkpoint every %d blocks)", s.chain, SyncCheckpointInterval)

	// Sync blocks
	for height := startHeight; height <= currentHeight; height++ {
		select {
		case <-ctx.Done():
			log.Printf("[%s] Sync cancelled at height %d, flushing to disk...", s.chain, height)
			s.db.Sync()
			s.db.SetHistoricalSyncMode(false)
			return
		default:
		}
		// log.Printf("[%s] Start sync to height %d", s.chain, height)
		if err := s.syncBlock(height); err != nil {
			log.Printf("[%s] Failed to sync block %d: %v", s.chain, height, err)
			// Wait and retry
			time.Sleep(5 * time.Second)
			height-- // Retry the same block
			continue
		}

		// log.Printf("[%s] Finish sync to height %d", s.chain, height)

		// Log progress every 100 blocks
		if height%100 == 0 {
			log.Printf("[%s] Synced to height %d", s.chain, height)
		}

		// Checkpoint sync every N blocks to ensure data durability
		if height%SyncCheckpointInterval == 0 {
			if err := s.db.Sync(); err != nil {
				log.Printf("[%s] Warning: checkpoint sync failed at height %d: %v", s.chain, height, err)
			}
		}
	}

	// Final sync and disable historical sync mode before bulk operations
	log.Printf("[%s] Historical block sync completed, flushing to disk...", s.chain)
	if err := s.db.Sync(); err != nil {
		log.Printf("[%s] Warning: final sync failed: %v", s.chain, err)
	}
	s.db.SetHistoricalSyncMode(false)
	log.Printf("[%s] Historical sync mode disabled, switching to normal sync", s.chain)

	log.Printf("[%s] Now updating spent flags in bulk...", s.chain)

	// Bulk update spent flags for all vouts
	spentCount, err := s.voutStore.BulkMarkSpent(s.chain, s.vinStore)
	if err != nil {
		log.Printf("[%s] Warning: bulk mark spent failed: %v", s.chain, err)
	} else {
		log.Printf("[%s] Bulk marked %d vouts as spent", s.chain, spentCount)
	}

	// Update address balances for inputs (subtract spent amounts)
	if err := s.bulkUpdateInputBalances(); err != nil {
		log.Printf("[%s] Warning: bulk update input balances failed: %v", s.chain, err)
	}

	s.mu.Lock()
	s.historicalDone = true
	pending := s.pendingBlocks
	s.pendingBlocks = nil
	s.mu.Unlock()

	log.Printf("[%s] Historical sync fully completed at height %d", s.chain, currentHeight)

	// Process any blocks that were queued during historical sync
	if len(pending) > 0 {
		log.Printf("[%s] Processing %d queued blocks", s.chain, len(pending))
		for _, pb := range pending {
			if err := s.processBlock(pb.block, pb.txs, pb.vins, pb.vouts); err != nil {
				log.Printf("[%s] Failed to process queued block %d: %v", s.chain, pb.block.Height, err)
			}
		}
		log.Printf("[%s] Finished processing queued blocks", s.chain)
	}
}

// syncBlock synchronizes a single block during historical sync (skips spent flag updates)
func (s *Syncer) syncBlock(height int64) error {
	block, txs, vins, vouts, err := s.notifier.GetBlockByHeight(height)
	if err != nil {
		return err
	}

	return s.processBlockHistorical(block, txs, vins, vouts)
}

// processBlockHistorical processes and stores a block without updating spent flags.
// This is optimized for bulk historical sync - spent flags are updated in bulk after all blocks are synced.
func (s *Syncer) processBlockHistorical(block *models.Block, txs []*models.Transaction, vins []*models.Vin, vouts []*models.Vout) error {
	// Save block
	if err := s.blockStore.Save(block); err != nil {
		return fmt.Errorf("failed to save block: %w", err)
	}

	// Save transactions
	if err := s.txStore.SaveBatch(txs); err != nil {
		return fmt.Errorf("failed to save transactions: %w", err)
	}

	// Save vins
	if err := s.vinStore.SaveBatch(vins); err != nil {
		return fmt.Errorf("failed to save vins: %w", err)
	}

	// Save vouts
	if err := s.voutStore.SaveBatch(vouts); err != nil {
		return fmt.Errorf("failed to save vouts: %w", err)
	}

	// NOTE: We skip marking spent vouts here - it will be done in bulk after historical sync
	// NOTE: We skip updating address balances for inputs here - it will be done in bulk after historical sync

	// Update address balances for outputs only
	txAddresses := make(map[string]map[string]bool) // txid -> addresses

	for _, vout := range vouts {
		if txAddresses[vout.TxID] == nil {
			txAddresses[vout.TxID] = make(map[string]bool)
		}
		for _, addr := range vout.Addresses {
			if err := s.addressStore.UpdateBalance(s.chain, addr, vout.Value, 0, !txAddresses[vout.TxID][addr]); err != nil {
				log.Printf("[%s] Warning: could not update address balance: %v", s.chain, err)
			}
			txAddresses[vout.TxID][addr] = true
			if err := s.addressStore.AddVoutReference(s.chain, addr, vout.TxID, vout.VoutIndex); err != nil {
				log.Printf("[%s] Warning: could not add vout reference: %v", s.chain, err)
			}
		}
	}

	// Update sync state
	if err := s.syncStore.SetSyncedHeight(s.chain, block.Height); err != nil {
		return fmt.Errorf("failed to update sync state: %w", err)
	}

	return nil
}

// handleBlockConnected processes a newly connected block
func (s *Syncer) handleBlockConnected(block *models.Block, txs []*models.Transaction, vins []*models.Vin, vouts []*models.Vout) {
	log.Printf("[%s] New block connected: %d %s", s.chain, block.Height, block.Hash)

	s.mu.Lock()
	done := s.historicalDone
	if !done {
		// Queue block for later processing - historical sync not complete yet
		s.pendingBlocks = append(s.pendingBlocks, &pendingBlock{
			block: block,
			txs:   txs,
			vins:  vins,
			vouts: vouts,
		})
		log.Printf("[%s] Queued block %d for later (historical sync in progress, %d blocks queued)",
			s.chain, block.Height, len(s.pendingBlocks))
		s.mu.Unlock()
		return
	}
	s.mu.Unlock()

	if err := s.processBlock(block, txs, vins, vouts); err != nil {
		log.Printf("[%s] Failed to process block %d: %v", s.chain, block.Height, err)
	}
}

// handleBlockDisconnected handles a disconnected block (reorg)
func (s *Syncer) handleBlockDisconnected(blockHash string, height int64) {
	log.Printf("[%s] Block disconnected: %d %s", s.chain, height, blockHash)

	if err := s.revertBlock(blockHash, height); err != nil {
		log.Printf("[%s] Failed to revert block %d: %v", s.chain, height, err)
	}
}

// processBlock processes and stores a block with all its data
func (s *Syncer) processBlock(block *models.Block, txs []*models.Transaction, vins []*models.Vin, vouts []*models.Vout) error {
	// Save block
	if err := s.blockStore.Save(block); err != nil {
		return fmt.Errorf("failed to save block: %w", err)
	}

	// Save transactions
	if err := s.txStore.SaveBatch(txs); err != nil {
		return fmt.Errorf("failed to save transactions: %w", err)
	}

	// Save vins
	if err := s.vinStore.SaveBatch(vins); err != nil {
		return fmt.Errorf("failed to save vins: %w", err)
	}

	// Save vouts first - needed before marking spent for intra-block spending
	// (when a later transaction in the same block spends an output from an earlier transaction)
	if err := s.voutStore.SaveBatch(vouts); err != nil {
		return fmt.Errorf("failed to save vouts: %w", err)
	}

	// Mark spent vouts and update address balances for inputs
	for _, vin := range vins {
		// Skip coinbase inputs - they don't reference any previous output
		// Coinbase has PrevTxID = "000...000" (64 zeros) or empty, and PrevVoutIdx = 4294967295 (max uint32)
		if vin.PrevTxID == "" || vin.PrevTxID == "0000000000000000000000000000000000000000000000000000000000000000" || vin.PrevVoutIdx == 4294967295 {
			continue // Coinbase transaction
		}

		// Mark the referenced vout as spent
		if err := s.voutStore.MarkSpent(s.chain, vin.PrevTxID, vin.PrevVoutIdx, vin.TxID, vin.VinIndex); err != nil {
			// Vout might not exist if it's from before start_height
			if block.Height > s.startHeight {
				log.Printf("[%s] Warning: MarkSpent failed at height %d: prev_tx=%s prev_vout=%d spent_by_tx=%s - %v",
					s.chain, block.Height, vin.PrevTxID, vin.PrevVoutIdx, vin.TxID, err)
			}
		}

		// Get the spent vout to update address balance
		spentVout, err := s.voutStore.Get(s.chain, vin.PrevTxID, vin.PrevVoutIdx)
		if err == nil && spentVout != nil {
			for _, addr := range spentVout.Addresses {
				if err := s.addressStore.UpdateBalance(s.chain, addr, 0, spentVout.Value, false); err != nil {
					log.Printf("[%s] Warning: could not update address balance: %v", s.chain, err)
				}
				if err := s.addressStore.AddVinReference(s.chain, addr, vin.TxID, vin.VinIndex); err != nil {
					log.Printf("[%s] Warning: could not add vin reference: %v", s.chain, err)
				}
			}
		}
	}

	// Update address balances for outputs
	// Track unique addresses per transaction for tx count
	txAddresses := make(map[string]map[string]bool) // txid -> addresses

	for _, vout := range vouts {
		if txAddresses[vout.TxID] == nil {
			txAddresses[vout.TxID] = make(map[string]bool)
		}
		for _, addr := range vout.Addresses {
			if err := s.addressStore.UpdateBalance(s.chain, addr, vout.Value, 0, !txAddresses[vout.TxID][addr]); err != nil {
				log.Printf("[%s] Warning: could not update address balance: %v", s.chain, err)
			}
			txAddresses[vout.TxID][addr] = true
			if err := s.addressStore.AddVoutReference(s.chain, addr, vout.TxID, vout.VoutIndex); err != nil {
				log.Printf("[%s] Warning: could not add vout reference: %v", s.chain, err)
			}
		}
	}

	// Update sync state
	if err := s.syncStore.SetSyncedHeight(s.chain, block.Height); err != nil {
		return fmt.Errorf("failed to update sync state: %w", err)
	}

	return nil
}

// revertBlock reverts a block during a reorg
func (s *Syncer) revertBlock(blockHash string, height int64) error {
	// Get the block to revert
	block, err := s.blockStore.GetByHash(s.chain, blockHash)
	if err != nil {
		return fmt.Errorf("failed to get block: %w", err)
	}
	if block == nil {
		return fmt.Errorf("block not found: %s", blockHash)
	}

	// Get all transactions in the block
	txs, err := s.txStore.GetByBlock(s.chain, blockHash)
	if err != nil {
		return fmt.Errorf("failed to get block transactions: %w", err)
	}

	// Revert each transaction
	for _, tx := range txs {
		// Get and revert vouts
		vouts, err := s.voutStore.GetByTx(s.chain, tx.TxID)
		if err != nil {
			log.Printf("[%s] Warning: could not get vouts for tx %s: %v", s.chain, tx.TxID, err)
		} else {
			for _, vout := range vouts {
				for _, addr := range vout.Addresses {
					// Subtract received amount
					if err := s.addressStore.UpdateBalance(s.chain, addr, -vout.Value, 0, false); err != nil {
						log.Printf("[%s] Warning: could not revert address balance: %v", s.chain, err)
					}
					if err := s.addressStore.RemoveVoutReference(s.chain, addr, vout.TxID, vout.VoutIndex); err != nil {
						log.Printf("[%s] Warning: could not remove vout reference: %v", s.chain, err)
					}
				}
			}
		}

		// Get and revert vins
		vins, err := s.vinStore.GetByTx(s.chain, tx.TxID)
		if err != nil {
			log.Printf("[%s] Warning: could not get vins for tx %s: %v", s.chain, tx.TxID, err)
		} else {
			for _, vin := range vins {
				if vin.PrevTxID == "" {
					continue // Coinbase
				}

				// Mark the referenced vout as unspent
				if err := s.voutStore.MarkUnspent(s.chain, vin.PrevTxID, vin.PrevVoutIdx); err != nil {
					log.Printf("[%s] Warning: could not mark vout as unspent: %v", s.chain, err)
				}

				// Revert address balance
				spentVout, err := s.voutStore.Get(s.chain, vin.PrevTxID, vin.PrevVoutIdx)
				if err == nil && spentVout != nil {
					for _, addr := range spentVout.Addresses {
						// Add back the sent amount
						if err := s.addressStore.UpdateBalance(s.chain, addr, 0, -spentVout.Value, false); err != nil {
							log.Printf("[%s] Warning: could not revert address balance: %v", s.chain, err)
						}
						if err := s.addressStore.RemoveVinReference(s.chain, addr, vin.TxID, vin.VinIndex); err != nil {
							log.Printf("[%s] Warning: could not remove vin reference: %v", s.chain, err)
						}
					}
				}
			}
		}

		// Delete vins, vouts, and transaction
		if err := s.vinStore.DeleteByTx(s.chain, tx.TxID); err != nil {
			log.Printf("[%s] Warning: could not delete vins: %v", s.chain, err)
		}
		if err := s.voutStore.DeleteByTx(s.chain, tx.TxID); err != nil {
			log.Printf("[%s] Warning: could not delete vouts: %v", s.chain, err)
		}
		if err := s.txStore.Delete(s.chain, tx.TxID); err != nil {
			log.Printf("[%s] Warning: could not delete transaction: %v", s.chain, err)
		}
	}

	// Delete the block
	if err := s.blockStore.Delete(s.chain, blockHash, height); err != nil {
		return fmt.Errorf("failed to delete block: %w", err)
	}

	// Update sync state to previous block
	if height > 0 {
		if err := s.syncStore.SetSyncedHeight(s.chain, height-1); err != nil {
			return fmt.Errorf("failed to update sync state: %w", err)
		}
	}

	return nil
}

// bulkUpdateInputBalances updates address balances for all spent vouts after historical sync.
// This iterates all vins and subtracts the spent amount from address balances.
func (s *Syncer) bulkUpdateInputBalances() error {
	log.Printf("[%s] Starting bulk update of input balances...", s.chain)

	// Get all vins for this chain
	prefix := []byte(s.chain + ":")
	iter, err := s.vinStore.NewPrefixIterator(prefix)
	if err != nil {
		return fmt.Errorf("failed to create vin iterator: %w", err)
	}
	defer iter.Close()

	count := 0
	for ; iter.Valid(); iter.Next() {
		var vin models.Vin
		if err := iter.Unmarshal(&vin); err != nil {
			log.Printf("[%s] Warning: failed to unmarshal vin: %v", s.chain, err)
			continue
		}

		// Skip coinbase inputs
		if vin.PrevTxID == "" ||
			vin.PrevTxID == "0000000000000000000000000000000000000000000000000000000000000000" ||
			vin.PrevVoutIdx == 4294967295 {
			continue
		}

		// Get the spent vout to update address balance
		spentVout, err := s.voutStore.Get(s.chain, vin.PrevTxID, vin.PrevVoutIdx)
		if err != nil || spentVout == nil {
			continue // Vout might not exist if it's from before start_height
		}

		for _, addr := range spentVout.Addresses {
			if err := s.addressStore.UpdateBalance(s.chain, addr, 0, spentVout.Value, false); err != nil {
				log.Printf("[%s] Warning: could not update address balance: %v", s.chain, err)
			}
			if err := s.addressStore.AddVinReference(s.chain, addr, vin.TxID, vin.VinIndex); err != nil {
				log.Printf("[%s] Warning: could not add vin reference: %v", s.chain, err)
			}
		}

		count++
		if count%100000 == 0 {
			log.Printf("[%s] Processed %d input balance updates...", s.chain, count)
		}
	}

	log.Printf("[%s] Completed bulk update of %d input balances", s.chain, count)
	return nil
}

// IsSyncing returns true if the syncer is currently running
func (s *Syncer) IsSyncing() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.syncing
}

// GetSyncedHeight returns the last synced block height
func (s *Syncer) GetSyncedHeight() (int64, error) {
	return s.syncStore.GetSyncedHeight(s.chain)
}
