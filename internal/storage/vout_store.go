package storage

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/thanhnp/chain-apis/internal/models"
)

// VoutStore handles vout storage operations
type VoutStore struct {
	db *RocksDB
}

// NewVoutStore creates a new VoutStore
func NewVoutStore(db *RocksDB) *VoutStore {
	return &VoutStore{db: db}
}

// voutKey creates a key for the vouts column family
func voutKey(chain, txid string, index int) []byte {
	return []byte(fmt.Sprintf("%s:%s:%06d", chain, txid, index))
}

// voutTxPrefix creates a prefix for all vouts in a transaction
func voutTxPrefix(chain, txid string) []byte {
	return []byte(fmt.Sprintf("%s:%s:", chain, txid))
}

// Save stores a vout in the database
func (s *VoutStore) Save(vout *models.Vout) error {
	data, err := json.Marshal(vout)
	if err != nil {
		return fmt.Errorf("failed to marshal vout: %w", err)
	}

	return s.db.Put(CFVouts, voutKey(vout.Chain, vout.TxID, vout.VoutIndex), data)
}

// Get retrieves a specific vout by transaction ID and index
func (s *VoutStore) Get(chain, txid string, index int) (*models.Vout, error) {
	data, err := s.db.Get(CFVouts, voutKey(chain, txid, index))
	if err != nil {
		return nil, err
	}
	if data == nil {
		return nil, nil
	}

	var vout models.Vout
	if err := json.Unmarshal(data, &vout); err != nil {
		return nil, fmt.Errorf("failed to unmarshal vout: %w", err)
	}
	return &vout, nil
}

// GetByTx retrieves all vouts for a transaction
func (s *VoutStore) GetByTx(chain, txid string) ([]*models.Vout, error) {
	prefix := voutTxPrefix(chain, txid)
	iter, err := s.db.NewPrefixIterator(CFVouts, prefix)
	if err != nil {
		return nil, err
	}
	defer iter.Close()

	var vouts []*models.Vout
	for ; iter.Valid(); iter.Next() {
		value := iter.Value()
		var vout models.Vout
		if err := json.Unmarshal(value, &vout); err != nil {
			return nil, fmt.Errorf("failed to unmarshal vout: %w", err)
		}
		vouts = append(vouts, &vout)
	}

	return vouts, nil
}

// Delete removes a vout from the database
func (s *VoutStore) Delete(chain, txid string, index int) error {
	return s.db.Delete(CFVouts, voutKey(chain, txid, index))
}

// MarkSpent marks a vout as spent
func (s *VoutStore) MarkSpent(chain, txid string, index int, spentByTxID string, spentByVin int) error {
	vout, err := s.Get(chain, txid, index)
	if err != nil {
		return err
	}
	if vout == nil {
		return fmt.Errorf("vout not found: %s:%s:%d", chain, txid, index)
	}

	vout.Spent = true
	vout.SpentByTxID = spentByTxID
	vout.SpentByVin = spentByVin
	vout.TxID = txid // restore TxID for Save

	return s.Save(vout)
}

// MarkUnspent marks a vout as unspent (for reorg handling)
func (s *VoutStore) MarkUnspent(chain, txid string, index int) error {
	vout, err := s.Get(chain, txid, index)
	if err != nil {
		return err
	}
	if vout == nil {
		return fmt.Errorf("vout not found: %s:%s:%d", chain, txid, index)
	}

	vout.Spent = false
	vout.SpentByTxID = ""
	vout.SpentByVin = 0
	vout.TxID = txid // restore TxID for Save

	return s.Save(vout)
}

// SaveBatch saves multiple vouts in a single batch operation
func (s *VoutStore) SaveBatch(vouts []*models.Vout) error {
	batch := s.db.NewBatch()
	defer batch.Destroy()

	for _, vout := range vouts {
		data, err := json.Marshal(vout)
		if err != nil {
			return fmt.Errorf("failed to marshal vout: %w", err)
		}
		if err := s.db.PutBatch(batch, CFVouts, voutKey(vout.Chain, vout.TxID, vout.VoutIndex), data); err != nil {
			return err
		}
	}

	return s.db.WriteBatch(batch)
}

// DeleteByTx deletes all vouts for a transaction
func (s *VoutStore) DeleteByTx(chain, txid string) error {
	prefix := voutTxPrefix(chain, txid)
	iter, err := s.db.NewPrefixIterator(CFVouts, prefix)
	if err != nil {
		return err
	}
	defer iter.Close()

	batch := s.db.NewBatch()
	defer batch.Destroy()

	for ; iter.Valid(); iter.Next() {
		key := iter.Key()
		if !bytes.HasPrefix(key, prefix) {
			break
		}

		// Copy key since we need it after advancing iterator
		keyCopy := make([]byte, len(key))
		copy(keyCopy, key)

		if err := s.db.DeleteBatch(batch, CFVouts, keyCopy); err != nil {
			return err
		}
	}

	return s.db.WriteBatch(batch)
}

// BulkMarkSpent iterates all vins for a chain and marks corresponding vouts as spent.
// This is more efficient than individual MarkSpent calls during bulk historical sync.
// Returns the number of vouts marked as spent and any error encountered.
func (s *VoutStore) BulkMarkSpent(chain string, vinStore *VinStore) (int, error) {
	// Create iterator over all vins for this chain
	prefix := []byte(chain + ":")
	iter, err := s.db.NewPrefixIterator(CFVins, prefix)
	if err != nil {
		return 0, fmt.Errorf("failed to create vin iterator: %w", err)
	}
	defer iter.Close()

	batch := s.db.NewBatch()
	defer batch.Destroy()

	count := 0
	batchSize := 0
	const maxBatchSize = 10000 // Commit batch every 10000 items to avoid memory issues

	for ; iter.Valid(); iter.Next() {
		value := iter.Value()
		var vin models.Vin
		if err := json.Unmarshal(value, &vin); err != nil {
			return count, fmt.Errorf("failed to unmarshal vin: %w", err)
		}

		// Skip coinbase inputs
		if vin.PrevTxID == "" ||
			vin.PrevTxID == "0000000000000000000000000000000000000000000000000000000000000000" ||
			vin.PrevVoutIdx == 4294967295 {
			continue
		}

		// Get the vout to mark as spent
		vout, err := s.Get(chain, vin.PrevTxID, vin.PrevVoutIdx)
		if err != nil {
			return count, fmt.Errorf("failed to get vout %s:%d: %w", vin.PrevTxID, vin.PrevVoutIdx, err)
		}
		if vout == nil {
			// Vout might not exist if it's from before start_height, skip it
			continue
		}

		// Update vout spent fields
		vout.Spent = true
		vout.SpentByTxID = vin.TxID
		vout.SpentByVin = vin.VinIndex

		// Marshal and add to batch
		data, err := json.Marshal(vout)
		if err != nil {
			return count, fmt.Errorf("failed to marshal vout: %w", err)
		}
		if err := s.db.PutBatch(batch, CFVouts, voutKey(chain, vin.PrevTxID, vin.PrevVoutIdx), data); err != nil {
			return count, fmt.Errorf("failed to add to batch: %w", err)
		}

		count++
		batchSize++

		// Commit batch periodically to avoid memory issues
		if batchSize >= maxBatchSize {
			if err := s.db.WriteBatch(batch); err != nil {
				return count, fmt.Errorf("failed to write batch: %w", err)
			}
			batch.Destroy()
			batch = s.db.NewBatch()
			batchSize = 0
		}
	}

	// Write remaining batch
	if batchSize > 0 {
		if err := s.db.WriteBatch(batch); err != nil {
			return count, fmt.Errorf("failed to write final batch: %w", err)
		}
	}

	return count, nil
}
