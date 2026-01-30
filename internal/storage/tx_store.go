package storage

import (
	"encoding/json"
	"fmt"

	"github.com/thanhnp/chain-apis/internal/models"
)

// TxStore handles transaction storage operations
type TxStore struct {
	db *RocksDB
}

// NewTxStore creates a new TxStore
func NewTxStore(db *RocksDB) *TxStore {
	return &TxStore{db: db}
}

// txKey creates a key for the transactions column family
func txKey(chain, txid string) []byte {
	return []byte(fmt.Sprintf("%s:%s", chain, txid))
}

// blockTxPrefix creates a prefix for transactions in a block
func blockTxPrefix(chain, blockHash string) []byte {
	return []byte(fmt.Sprintf("%s:%s:", chain, blockHash))
}

// Save stores a transaction in the database
func (s *TxStore) Save(tx *models.Transaction) error {
	data, err := json.Marshal(tx)
	if err != nil {
		return fmt.Errorf("failed to marshal transaction: %w", err)
	}

	return s.db.Put(CFTransactions, txKey(tx.Chain, tx.TxID), data)
}

// Get retrieves a transaction by its ID
func (s *TxStore) Get(chain, txid string) (*models.Transaction, error) {
	data, err := s.db.Get(CFTransactions, txKey(chain, txid))
	if err != nil {
		return nil, err
	}
	if data == nil {
		return nil, nil
	}

	var tx models.Transaction
	if err := json.Unmarshal(data, &tx); err != nil {
		return nil, fmt.Errorf("failed to unmarshal transaction: %w", err)
	}
	return &tx, nil
}

// GetByBlock retrieves all transactions in a block
func (s *TxStore) GetByBlock(chain, blockHash string) ([]*models.Transaction, error) {
	prefix := []byte(fmt.Sprintf("%s:", chain))
	iter, err := s.db.NewPrefixIterator(CFTransactions, prefix)
	if err != nil {
		return nil, err
	}
	defer iter.Close()

	var txs []*models.Transaction
	for ; iter.Valid(); iter.Next() {
		value := iter.Value()
		var tx models.Transaction
		if err := json.Unmarshal(value, &tx); err != nil {
			return nil, fmt.Errorf("failed to unmarshal transaction: %w", err)
		}

		if tx.BlockHash == blockHash {
			txs = append(txs, &tx)
		}
	}

	return txs, nil
}

// GetByBlockPaginated retrieves transactions in a block with pagination
// Returns transactions, total count, and error
func (s *TxStore) GetByBlockPaginated(chain, blockHash string, offset, limit int) ([]*models.Transaction, int, error) {
	prefix := []byte(fmt.Sprintf("%s:", chain))
	iter, err := s.db.NewPrefixIterator(CFTransactions, prefix)
	if err != nil {
		return nil, 0, err
	}
	defer iter.Close()

	var allTxs []*models.Transaction
	for ; iter.Valid(); iter.Next() {
		value := iter.Value()
		var tx models.Transaction
		if err := json.Unmarshal(value, &tx); err != nil {
			return nil, 0, fmt.Errorf("failed to unmarshal transaction: %w", err)
		}

		if tx.BlockHash == blockHash {
			allTxs = append(allTxs, &tx)
		}
	}

	total := len(allTxs)

	// Apply pagination
	if offset >= total {
		return []*models.Transaction{}, total, nil
	}

	end := offset + limit
	if end > total {
		end = total
	}

	return allTxs[offset:end], total, nil
}

// Delete removes a transaction from the database
func (s *TxStore) Delete(chain, txid string) error {
	return s.db.Delete(CFTransactions, txKey(chain, txid))
}

// SaveBatch saves multiple transactions in a single batch operation
func (s *TxStore) SaveBatch(txs []*models.Transaction) error {
	batch := s.db.NewBatch()
	defer batch.Destroy()

	for _, tx := range txs {
		data, err := json.Marshal(tx)
		if err != nil {
			return fmt.Errorf("failed to marshal transaction: %w", err)
		}
		if err := s.db.PutBatch(batch, CFTransactions, txKey(tx.Chain, tx.TxID), data); err != nil {
			return err
		}
	}

	return s.db.WriteBatch(batch)
}

// DeleteBatch deletes multiple transactions in a single batch operation
func (s *TxStore) DeleteBatch(chain string, txids []string) error {
	batch := s.db.NewBatch()
	defer batch.Destroy()

	for _, txid := range txids {
		if err := s.db.DeleteBatch(batch, CFTransactions, txKey(chain, txid)); err != nil {
			return err
		}
	}

	return s.db.WriteBatch(batch)
}
