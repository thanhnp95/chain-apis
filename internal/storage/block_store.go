package storage

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/thanhnp/chain-apis/internal/models"
)

// BlockStore handles block storage operations
type BlockStore struct {
	db *RocksDB
}

// NewBlockStore creates a new BlockStore
func NewBlockStore(db *RocksDB) *BlockStore {
	return &BlockStore{db: db}
}

// blockKey creates a key for the blocks column family
func blockKey(chain, hash string) []byte {
	return []byte(fmt.Sprintf("%s:%s", chain, hash))
}

// blockHeightKey creates a key for the blocks_by_height column family
func blockHeightKey(chain string, height int64) []byte {
	return []byte(fmt.Sprintf("%s:%012d", chain, height))
}

// Save stores a block in the database
func (s *BlockStore) Save(block *models.Block) error {
	data, err := json.Marshal(block)
	if err != nil {
		return fmt.Errorf("failed to marshal block: %w", err)
	}

	batch := s.db.NewBatch()
	defer batch.Destroy()

	// Store block by hash
	if err := s.db.PutBatch(batch, CFBlocks, blockKey(block.Chain, block.Hash), data); err != nil {
		return err
	}

	// Store hash by height for lookup
	if err := s.db.PutBatch(batch, CFBlocksByHeight, blockHeightKey(block.Chain, block.Height), []byte(block.Hash)); err != nil {
		return err
	}

	return s.db.WriteBatch(batch)
}

// GetByHash retrieves a block by its hash
func (s *BlockStore) GetByHash(chain, hash string) (*models.Block, error) {
	data, err := s.db.Get(CFBlocks, blockKey(chain, hash))
	if err != nil {
		return nil, err
	}
	if data == nil {
		return nil, nil
	}

	var block models.Block
	if err := json.Unmarshal(data, &block); err != nil {
		return nil, fmt.Errorf("failed to unmarshal block: %w", err)
	}
	return &block, nil
}

// GetByHeight retrieves a block by its height
func (s *BlockStore) GetByHeight(chain string, height int64) (*models.Block, error) {
	// Get hash from height index
	hashData, err := s.db.Get(CFBlocksByHeight, blockHeightKey(chain, height))
	if err != nil {
		return nil, err
	}
	if hashData == nil {
		return nil, nil
	}

	return s.GetByHash(chain, string(hashData))
}

// GetLatest retrieves the latest block for a chain
func (s *BlockStore) GetLatest(chain string) (*models.Block, error) {
	// Get sync state to find the latest height
	heightData, err := s.db.Get(CFSyncState, []byte(chain))
	if err != nil {
		return nil, err
	}
	if heightData == nil {
		return nil, nil
	}

	height, err := strconv.ParseInt(string(heightData), 10, 64)
	if err != nil {
		return nil, fmt.Errorf("failed to parse sync height: %w", err)
	}

	return s.GetByHeight(chain, height)
}

// Delete removes a block from the database
func (s *BlockStore) Delete(chain, hash string, height int64) error {
	batch := s.db.NewBatch()
	defer batch.Destroy()

	if err := s.db.DeleteBatch(batch, CFBlocks, blockKey(chain, hash)); err != nil {
		return err
	}
	if err := s.db.DeleteBatch(batch, CFBlocksByHeight, blockHeightKey(chain, height)); err != nil {
		return err
	}

	return s.db.WriteBatch(batch)
}

// SaveBatch saves multiple blocks in a single batch operation
func (s *BlockStore) SaveBatch(blocks []*models.Block) error {
	batch := s.db.NewBatch()
	defer batch.Destroy()

	for _, block := range blocks {
		data, err := json.Marshal(block)
		if err != nil {
			return fmt.Errorf("failed to marshal block: %w", err)
		}

		if err := s.db.PutBatch(batch, CFBlocks, blockKey(block.Chain, block.Hash), data); err != nil {
			return err
		}
		if err := s.db.PutBatch(batch, CFBlocksByHeight, blockHeightKey(block.Chain, block.Height), []byte(block.Hash)); err != nil {
			return err
		}
	}

	return s.db.WriteBatch(batch)
}
