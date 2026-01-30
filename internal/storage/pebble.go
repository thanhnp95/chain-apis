package storage

import (
	"bytes"
	"fmt"
	"os"

	"github.com/cockroachdb/pebble"
)

// Key prefixes (simulating column families)
const (
	PrefixBlocks         = "blk:"
	PrefixBlocksByHeight = "bht:"
	PrefixTransactions   = "txn:"
	PrefixVins           = "vin:"
	PrefixVouts          = "vot:"
	PrefixAddresses      = "adr:"
	PrefixAddressVins    = "avi:"
	PrefixAddressVouts   = "avo:"
	PrefixSyncState      = "syn:"
)

// Column family name to prefix mapping
var cfPrefixes = map[string]string{
	CFBlocks:         PrefixBlocks,
	CFBlocksByHeight: PrefixBlocksByHeight,
	CFTransactions:   PrefixTransactions,
	CFVins:           PrefixVins,
	CFVouts:          PrefixVouts,
	CFAddresses:      PrefixAddresses,
	CFAddressVins:    PrefixAddressVins,
	CFAddressVouts:   PrefixAddressVouts,
	CFSyncState:      PrefixSyncState,
}

// Column family names (kept for compatibility)
const (
	CFBlocks         = "blocks"
	CFBlocksByHeight = "blocks_by_height"
	CFTransactions   = "transactions"
	CFVins           = "vins"
	CFVouts          = "vouts"
	CFAddresses      = "addresses"
	CFAddressVins    = "address_vins"
	CFAddressVouts   = "address_vouts"
	CFSyncState      = "sync_state"
)

// PebbleDB wraps the Pebble database
type PebbleDB struct {
	db                 *pebble.DB
	historicalSyncMode bool // When true, uses NoSync for faster writes
}

// WriteBatch wraps Pebble's batch for atomic writes
type WriteBatch struct {
	batch *pebble.Batch
	db    *PebbleDB
}

// Iterator wraps Pebble's iterator
type Iterator struct {
	iter     *pebble.Iterator
	prefix   []byte // full prefix (cf + user prefix) for bounds checking
	cfPrefix []byte // just the column family prefix (to strip from keys)
}

// NewPebbleDB creates a new PebbleDB instance
func NewPebbleDB(path string) (*PebbleDB, error) {
	// Ensure directory exists
	if err := os.MkdirAll(path, 0755); err != nil {
		return nil, fmt.Errorf("failed to create database directory: %w", err)
	}

	opts := &pebble.Options{
		Cache:        pebble.NewCache(512 << 20), // 512MB cache for faster sync
		MaxOpenFiles: 500,
	}

	db, err := pebble.Open(path, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	return &PebbleDB{db: db}, nil
}

// Close closes the database
func (p *PebbleDB) Close() error {
	return p.db.Close()
}

// SetHistoricalSyncMode enables/disables historical sync mode.
// When enabled, writes use NoSync for faster performance.
// Call Sync() at checkpoints to ensure data durability.
func (p *PebbleDB) SetHistoricalSyncMode(enabled bool) {
	p.historicalSyncMode = enabled
}

// IsHistoricalSyncMode returns true if historical sync mode is enabled
func (p *PebbleDB) IsHistoricalSyncMode() bool {
	return p.historicalSyncMode
}

// Sync forces a sync to disk. Use this at checkpoints during historical sync.
func (p *PebbleDB) Sync() error {
	return p.db.Flush()
}

// writeOptions returns the appropriate write options based on sync mode
func (p *PebbleDB) writeOptions() *pebble.WriteOptions {
	if p.historicalSyncMode {
		return pebble.NoSync
	}
	return pebble.Sync
}

// prefixKey creates a prefixed key for the given column family
func (p *PebbleDB) prefixKey(cf string, key []byte) ([]byte, error) {
	prefix, ok := cfPrefixes[cf]
	if !ok {
		return nil, fmt.Errorf("column family not found: %s", cf)
	}
	return append([]byte(prefix), key...), nil
}

// Put stores a key-value pair in the specified column family
func (p *PebbleDB) Put(cf string, key, value []byte) error {
	prefixedKey, err := p.prefixKey(cf, key)
	if err != nil {
		return err
	}
	return p.db.Set(prefixedKey, value, p.writeOptions())
}

// Get retrieves a value from the specified column family
func (p *PebbleDB) Get(cf string, key []byte) ([]byte, error) {
	prefixedKey, err := p.prefixKey(cf, key)
	if err != nil {
		return nil, err
	}

	value, closer, err := p.db.Get(prefixedKey)
	if err != nil {
		if err == pebble.ErrNotFound {
			return nil, nil
		}
		return nil, err
	}
	defer closer.Close()

	// Copy the value since it's only valid until closer.Close()
	result := make([]byte, len(value))
	copy(result, value)
	return result, nil
}

// Delete removes a key from the specified column family
func (p *PebbleDB) Delete(cf string, key []byte) error {
	prefixedKey, err := p.prefixKey(cf, key)
	if err != nil {
		return err
	}
	return p.db.Delete(prefixedKey, p.writeOptions())
}

// NewBatch creates a new write batch
func (p *PebbleDB) NewBatch() *WriteBatch {
	return &WriteBatch{
		batch: p.db.NewBatch(),
		db:    p,
	}
}

// WriteBatch writes a batch to the database
func (p *PebbleDB) WriteBatch(batch *WriteBatch) error {
	return batch.batch.Commit(p.writeOptions())
}

// PutBatch adds a put operation to the batch
func (p *PebbleDB) PutBatch(batch *WriteBatch, cf string, key, value []byte) error {
	prefixedKey, err := p.prefixKey(cf, key)
	if err != nil {
		return err
	}
	return batch.batch.Set(prefixedKey, value, nil)
}

// DeleteBatch adds a delete operation to the batch
func (p *PebbleDB) DeleteBatch(batch *WriteBatch, cf string, key []byte) error {
	prefixedKey, err := p.prefixKey(cf, key)
	if err != nil {
		return err
	}
	return batch.batch.Delete(prefixedKey, nil)
}

// Destroy closes the batch and releases resources
func (b *WriteBatch) Destroy() {
	b.batch.Close()
}

// NewIterator creates an iterator for the specified column family
func (p *PebbleDB) NewIterator(cf string) (*Iterator, error) {
	prefix, ok := cfPrefixes[cf]
	if !ok {
		return nil, fmt.Errorf("column family not found: %s", cf)
	}

	prefixBytes := []byte(prefix)
	iter, err := p.db.NewIter(&pebble.IterOptions{
		LowerBound: prefixBytes,
		UpperBound: prefixUpperBound(prefixBytes),
	})
	if err != nil {
		return nil, err
	}

	iter.First()
	return &Iterator{iter: iter, prefix: prefixBytes, cfPrefix: prefixBytes}, nil
}

// NewPrefixIterator creates an iterator that seeks to the given prefix within a column family
func (p *PebbleDB) NewPrefixIterator(cf string, prefix []byte) (*Iterator, error) {
	cfPrefix, ok := cfPrefixes[cf]
	if !ok {
		return nil, fmt.Errorf("column family not found: %s", cf)
	}

	cfPrefixBytes := []byte(cfPrefix)
	fullPrefix := append(cfPrefixBytes, prefix...)
	iter, err := p.db.NewIter(&pebble.IterOptions{
		LowerBound: fullPrefix,
		UpperBound: prefixUpperBound(fullPrefix),
	})
	if err != nil {
		return nil, err
	}

	iter.First()
	return &Iterator{iter: iter, prefix: fullPrefix, cfPrefix: cfPrefixBytes}, nil
}

// prefixUpperBound returns the upper bound for prefix iteration
func prefixUpperBound(prefix []byte) []byte {
	if len(prefix) == 0 {
		return nil
	}
	upper := make([]byte, len(prefix))
	copy(upper, prefix)
	for i := len(upper) - 1; i >= 0; i-- {
		if upper[i] < 0xff {
			upper[i]++
			return upper[:i+1]
		}
	}
	return nil
}

// Iterator methods

// Valid returns true if the iterator is positioned at a valid key
func (i *Iterator) Valid() bool {
	return i.iter.Valid()
}

// Next advances the iterator to the next key
func (i *Iterator) Next() bool {
	return i.iter.Next()
}

// Key returns the current key (without the column family prefix)
func (i *Iterator) Key() []byte {
	key := i.iter.Key()
	// Strip only the column family prefix, keep the user prefix
	if len(key) > len(i.cfPrefix) && bytes.HasPrefix(key, i.cfPrefix) {
		return key[len(i.cfPrefix):]
	}
	return key
}

// Value returns the current value
func (i *Iterator) Value() []byte {
	return i.iter.Value()
}

// Close closes the iterator
func (i *Iterator) Close() error {
	return i.iter.Close()
}

// Alias for backward compatibility
type RocksDB = PebbleDB

// NewRocksDB creates a new PebbleDB instance (alias for backward compatibility)
func NewRocksDB(path string) (*PebbleDB, error) {
	return NewPebbleDB(path)
}
