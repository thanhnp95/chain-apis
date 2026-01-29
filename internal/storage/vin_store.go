package storage

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/thanhnp/chain-apis/internal/models"
)

// VinStore handles vin storage operations
type VinStore struct {
	db *RocksDB
}

// NewVinStore creates a new VinStore
func NewVinStore(db *RocksDB) *VinStore {
	return &VinStore{db: db}
}

// vinKey creates a key for the vins column family
func vinKey(chain, txid string, index int) []byte {
	return []byte(fmt.Sprintf("%s:%s:%06d", chain, txid, index))
}

// vinTxPrefix creates a prefix for all vins in a transaction
func vinTxPrefix(chain, txid string) []byte {
	return []byte(fmt.Sprintf("%s:%s:", chain, txid))
}

// Save stores a vin in the database
func (s *VinStore) Save(vin *models.Vin) error {
	data, err := json.Marshal(vin)
	if err != nil {
		return fmt.Errorf("failed to marshal vin: %w", err)
	}

	return s.db.Put(CFVins, vinKey(vin.Chain, vin.TxID, vin.VinIndex), data)
}

// Get retrieves a specific vin by transaction ID and index
func (s *VinStore) Get(chain, txid string, index int) (*models.Vin, error) {
	data, err := s.db.Get(CFVins, vinKey(chain, txid, index))
	if err != nil {
		return nil, err
	}
	if data == nil {
		return nil, nil
	}

	var vin models.Vin
	if err := json.Unmarshal(data, &vin); err != nil {
		return nil, fmt.Errorf("failed to unmarshal vin: %w", err)
	}
	return &vin, nil
}

// GetByTx retrieves all vins for a transaction
func (s *VinStore) GetByTx(chain, txid string) ([]*models.Vin, error) {
	prefix := vinTxPrefix(chain, txid)
	iter, err := s.db.NewPrefixIterator(CFVins, prefix)
	if err != nil {
		return nil, err
	}
	defer iter.Close()

	var vins []*models.Vin
	for ; iter.Valid(); iter.Next() {
		key := iter.Key()
		if !bytes.HasPrefix(key, prefix) {
			break
		}

		value := iter.Value()
		var vin models.Vin
		if err := json.Unmarshal(value, &vin); err != nil {
			return nil, fmt.Errorf("failed to unmarshal vin: %w", err)
		}
		vins = append(vins, &vin)
	}

	return vins, nil
}

// Delete removes a vin from the database
func (s *VinStore) Delete(chain, txid string, index int) error {
	return s.db.Delete(CFVins, vinKey(chain, txid, index))
}

// SaveBatch saves multiple vins in a single batch operation
func (s *VinStore) SaveBatch(vins []*models.Vin) error {
	batch := s.db.NewBatch()
	defer batch.Destroy()

	for _, vin := range vins {
		data, err := json.Marshal(vin)
		if err != nil {
			return fmt.Errorf("failed to marshal vin: %w", err)
		}
		if err := s.db.PutBatch(batch, CFVins, vinKey(vin.Chain, vin.TxID, vin.VinIndex), data); err != nil {
			return err
		}
	}

	return s.db.WriteBatch(batch)
}

// DeleteByTx deletes all vins for a transaction
func (s *VinStore) DeleteByTx(chain, txid string) error {
	prefix := vinTxPrefix(chain, txid)
	iter, err := s.db.NewPrefixIterator(CFVins, prefix)
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

		if err := s.db.DeleteBatch(batch, CFVins, keyCopy); err != nil {
			return err
		}
	}

	return s.db.WriteBatch(batch)
}

// VinIterator wraps the database iterator for vin-specific iteration
type VinIterator struct {
	iter *Iterator
}

// NewPrefixIterator creates an iterator for vins with the given prefix
func (s *VinStore) NewPrefixIterator(prefix []byte) (*VinIterator, error) {
	iter, err := s.db.NewPrefixIterator(CFVins, prefix)
	if err != nil {
		return nil, err
	}
	return &VinIterator{iter: iter}, nil
}

// Valid returns true if the iterator is positioned at a valid key
func (vi *VinIterator) Valid() bool {
	return vi.iter.Valid()
}

// Next advances the iterator to the next key
func (vi *VinIterator) Next() bool {
	return vi.iter.Next()
}

// Unmarshal unmarshals the current value into a Vin
func (vi *VinIterator) Unmarshal(vin *models.Vin) error {
	value := vi.iter.Value()
	return json.Unmarshal(value, vin)
}

// Close closes the iterator
func (vi *VinIterator) Close() error {
	return vi.iter.Close()
}
