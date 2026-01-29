package storage

import (
	"fmt"
	"strconv"
)

// SyncStore handles sync state storage operations
type SyncStore struct {
	db *RocksDB
}

// NewSyncStore creates a new SyncStore
func NewSyncStore(db *RocksDB) *SyncStore {
	return &SyncStore{db: db}
}

// GetSyncedHeight retrieves the last synced block height for a chain
func (s *SyncStore) GetSyncedHeight(chain string) (int64, error) {
	data, err := s.db.Get(CFSyncState, []byte(chain))
	if err != nil {
		return 0, err
	}
	if data == nil {
		return -1, nil // -1 indicates no sync state exists
	}

	height, err := strconv.ParseInt(string(data), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse sync height: %w", err)
	}

	return height, nil
}

// SetSyncedHeight sets the last synced block height for a chain
func (s *SyncStore) SetSyncedHeight(chain string, height int64) error {
	return s.db.Put(CFSyncState, []byte(chain), []byte(strconv.FormatInt(height, 10)))
}
