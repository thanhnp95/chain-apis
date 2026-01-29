package storage

import (
	"fmt"

	"github.com/thanhnp/chain-apis/internal/models"
)

// ChainStores holds all stores for a single chain
type ChainStores struct {
	DB           *PebbleDB
	BlockStore   *BlockStore
	TxStore      *TxStore
	VinStore     *VinStore
	VoutStore    *VoutStore
	AddressStore *AddressStore
	SyncStore    *SyncStore
}

// NewChainStores creates all stores for a chain using the given database
func NewChainStores(db *PebbleDB) *ChainStores {
	return &ChainStores{
		DB:           db,
		BlockStore:   NewBlockStore(db),
		TxStore:      NewTxStore(db),
		VinStore:     NewVinStore(db),
		VoutStore:    NewVoutStore(db),
		AddressStore: NewAddressStore(db),
		SyncStore:    NewSyncStore(db),
	}
}

// Close closes the database
func (cs *ChainStores) Close() error {
	return cs.DB.Close()
}

// MultiChainBlockStore wraps multiple chain-specific block stores
type MultiChainBlockStore struct {
	stores map[string]*BlockStore
}

// NewMultiChainBlockStore creates a new multi-chain block store
func NewMultiChainBlockStore() *MultiChainBlockStore {
	return &MultiChainBlockStore{
		stores: make(map[string]*BlockStore),
	}
}

// RegisterChain registers a block store for a chain
func (m *MultiChainBlockStore) RegisterChain(chain string, store *BlockStore) {
	m.stores[chain] = store
}

// getStore returns the store for the given chain
func (m *MultiChainBlockStore) getStore(chain string) (*BlockStore, error) {
	store, ok := m.stores[chain]
	if !ok {
		return nil, fmt.Errorf("chain not registered: %s", chain)
	}
	return store, nil
}

// Save stores a block
func (m *MultiChainBlockStore) Save(block *models.Block) error {
	store, err := m.getStore(block.Chain)
	if err != nil {
		return err
	}
	return store.Save(block)
}

// GetByHash retrieves a block by hash
func (m *MultiChainBlockStore) GetByHash(chain, hash string) (*models.Block, error) {
	store, err := m.getStore(chain)
	if err != nil {
		return nil, err
	}
	return store.GetByHash(chain, hash)
}

// GetByHeight retrieves a block by height
func (m *MultiChainBlockStore) GetByHeight(chain string, height int64) (*models.Block, error) {
	store, err := m.getStore(chain)
	if err != nil {
		return nil, err
	}
	return store.GetByHeight(chain, height)
}

// GetLatest retrieves the latest block
func (m *MultiChainBlockStore) GetLatest(chain string) (*models.Block, error) {
	store, err := m.getStore(chain)
	if err != nil {
		return nil, err
	}
	return store.GetLatest(chain)
}

// Delete removes a block
func (m *MultiChainBlockStore) Delete(chain, hash string, height int64) error {
	store, err := m.getStore(chain)
	if err != nil {
		return err
	}
	return store.Delete(chain, hash, height)
}

// MultiChainTxStore wraps multiple chain-specific transaction stores
type MultiChainTxStore struct {
	stores map[string]*TxStore
}

// NewMultiChainTxStore creates a new multi-chain transaction store
func NewMultiChainTxStore() *MultiChainTxStore {
	return &MultiChainTxStore{
		stores: make(map[string]*TxStore),
	}
}

// RegisterChain registers a transaction store for a chain
func (m *MultiChainTxStore) RegisterChain(chain string, store *TxStore) {
	m.stores[chain] = store
}

// getStore returns the store for the given chain
func (m *MultiChainTxStore) getStore(chain string) (*TxStore, error) {
	store, ok := m.stores[chain]
	if !ok {
		return nil, fmt.Errorf("chain not registered: %s", chain)
	}
	return store, nil
}

// Save stores a transaction
func (m *MultiChainTxStore) Save(tx *models.Transaction) error {
	store, err := m.getStore(tx.Chain)
	if err != nil {
		return err
	}
	return store.Save(tx)
}

// SaveBatch stores multiple transactions
func (m *MultiChainTxStore) SaveBatch(txs []*models.Transaction) error {
	if len(txs) == 0 {
		return nil
	}
	store, err := m.getStore(txs[0].Chain)
	if err != nil {
		return err
	}
	return store.SaveBatch(txs)
}

// Get retrieves a transaction by ID
func (m *MultiChainTxStore) Get(chain, txid string) (*models.Transaction, error) {
	store, err := m.getStore(chain)
	if err != nil {
		return nil, err
	}
	return store.Get(chain, txid)
}

// GetByBlock retrieves all transactions in a block
func (m *MultiChainTxStore) GetByBlock(chain, blockHash string) ([]*models.Transaction, error) {
	store, err := m.getStore(chain)
	if err != nil {
		return nil, err
	}
	return store.GetByBlock(chain, blockHash)
}

// Delete removes a transaction
func (m *MultiChainTxStore) Delete(chain, txid string) error {
	store, err := m.getStore(chain)
	if err != nil {
		return err
	}
	return store.Delete(chain, txid)
}

// MultiChainVinStore wraps multiple chain-specific vin stores
type MultiChainVinStore struct {
	stores map[string]*VinStore
}

// NewMultiChainVinStore creates a new multi-chain vin store
func NewMultiChainVinStore() *MultiChainVinStore {
	return &MultiChainVinStore{
		stores: make(map[string]*VinStore),
	}
}

// RegisterChain registers a vin store for a chain
func (m *MultiChainVinStore) RegisterChain(chain string, store *VinStore) {
	m.stores[chain] = store
}

// getStore returns the store for the given chain
func (m *MultiChainVinStore) getStore(chain string) (*VinStore, error) {
	store, ok := m.stores[chain]
	if !ok {
		return nil, fmt.Errorf("chain not registered: %s", chain)
	}
	return store, nil
}

// Save stores a vin
func (m *MultiChainVinStore) Save(vin *models.Vin) error {
	store, err := m.getStore(vin.Chain)
	if err != nil {
		return err
	}
	return store.Save(vin)
}

// SaveBatch stores multiple vins
func (m *MultiChainVinStore) SaveBatch(vins []*models.Vin) error {
	if len(vins) == 0 {
		return nil
	}
	store, err := m.getStore(vins[0].Chain)
	if err != nil {
		return err
	}
	return store.SaveBatch(vins)
}

// Get retrieves a vin by transaction ID and index
func (m *MultiChainVinStore) Get(chain, txid string, index int) (*models.Vin, error) {
	store, err := m.getStore(chain)
	if err != nil {
		return nil, err
	}
	return store.Get(chain, txid, index)
}

// GetByTx retrieves all vins for a transaction
func (m *MultiChainVinStore) GetByTx(chain, txid string) ([]*models.Vin, error) {
	store, err := m.getStore(chain)
	if err != nil {
		return nil, err
	}
	return store.GetByTx(chain, txid)
}

// Delete removes a vin
func (m *MultiChainVinStore) Delete(chain, txid string, index int) error {
	store, err := m.getStore(chain)
	if err != nil {
		return err
	}
	return store.Delete(chain, txid, index)
}

// DeleteByTx deletes all vins for a transaction
func (m *MultiChainVinStore) DeleteByTx(chain, txid string) error {
	store, err := m.getStore(chain)
	if err != nil {
		return err
	}
	return store.DeleteByTx(chain, txid)
}

// NewPrefixIterator creates an iterator for vins with the given prefix
func (m *MultiChainVinStore) NewPrefixIterator(chain string, prefix []byte) (*VinIterator, error) {
	store, err := m.getStore(chain)
	if err != nil {
		return nil, err
	}
	return store.NewPrefixIterator(prefix)
}

// MultiChainVoutStore wraps multiple chain-specific vout stores
type MultiChainVoutStore struct {
	stores map[string]*VoutStore
}

// NewMultiChainVoutStore creates a new multi-chain vout store
func NewMultiChainVoutStore() *MultiChainVoutStore {
	return &MultiChainVoutStore{
		stores: make(map[string]*VoutStore),
	}
}

// RegisterChain registers a vout store for a chain
func (m *MultiChainVoutStore) RegisterChain(chain string, store *VoutStore) {
	m.stores[chain] = store
}

// getStore returns the store for the given chain
func (m *MultiChainVoutStore) getStore(chain string) (*VoutStore, error) {
	store, ok := m.stores[chain]
	if !ok {
		return nil, fmt.Errorf("chain not registered: %s", chain)
	}
	return store, nil
}

// Save stores a vout
func (m *MultiChainVoutStore) Save(vout *models.Vout) error {
	store, err := m.getStore(vout.Chain)
	if err != nil {
		return err
	}
	return store.Save(vout)
}

// SaveBatch stores multiple vouts
func (m *MultiChainVoutStore) SaveBatch(vouts []*models.Vout) error {
	if len(vouts) == 0 {
		return nil
	}
	store, err := m.getStore(vouts[0].Chain)
	if err != nil {
		return err
	}
	return store.SaveBatch(vouts)
}

// Get retrieves a vout by transaction ID and index
func (m *MultiChainVoutStore) Get(chain, txid string, index int) (*models.Vout, error) {
	store, err := m.getStore(chain)
	if err != nil {
		return nil, err
	}
	return store.Get(chain, txid, index)
}

// GetByTx retrieves all vouts for a transaction
func (m *MultiChainVoutStore) GetByTx(chain, txid string) ([]*models.Vout, error) {
	store, err := m.getStore(chain)
	if err != nil {
		return nil, err
	}
	return store.GetByTx(chain, txid)
}

// Delete removes a vout
func (m *MultiChainVoutStore) Delete(chain, txid string, index int) error {
	store, err := m.getStore(chain)
	if err != nil {
		return err
	}
	return store.Delete(chain, txid, index)
}

// DeleteByTx deletes all vouts for a transaction
func (m *MultiChainVoutStore) DeleteByTx(chain, txid string) error {
	store, err := m.getStore(chain)
	if err != nil {
		return err
	}
	return store.DeleteByTx(chain, txid)
}

// MarkSpent marks a vout as spent
func (m *MultiChainVoutStore) MarkSpent(chain, txid string, index int, spentByTxID string, spentByVin int) error {
	store, err := m.getStore(chain)
	if err != nil {
		return err
	}
	return store.MarkSpent(chain, txid, index, spentByTxID, spentByVin)
}

// MarkUnspent marks a vout as unspent
func (m *MultiChainVoutStore) MarkUnspent(chain, txid string, index int) error {
	store, err := m.getStore(chain)
	if err != nil {
		return err
	}
	return store.MarkUnspent(chain, txid, index)
}

// BulkMarkSpent marks all spent vouts for a chain
func (m *MultiChainVoutStore) BulkMarkSpent(chain string, vinStore *VinStore) (int, error) {
	store, err := m.getStore(chain)
	if err != nil {
		return 0, err
	}
	return store.BulkMarkSpent(chain, vinStore)
}

// MultiChainAddressStore wraps multiple chain-specific address stores
type MultiChainAddressStore struct {
	stores map[string]*AddressStore
}

// NewMultiChainAddressStore creates a new multi-chain address store
func NewMultiChainAddressStore() *MultiChainAddressStore {
	return &MultiChainAddressStore{
		stores: make(map[string]*AddressStore),
	}
}

// RegisterChain registers an address store for a chain
func (m *MultiChainAddressStore) RegisterChain(chain string, store *AddressStore) {
	m.stores[chain] = store
}

// getStore returns the store for the given chain
func (m *MultiChainAddressStore) getStore(chain string) (*AddressStore, error) {
	store, ok := m.stores[chain]
	if !ok {
		return nil, fmt.Errorf("chain not registered: %s", chain)
	}
	return store, nil
}

// Get retrieves an address
func (m *MultiChainAddressStore) Get(chain, address string) (*models.Address, error) {
	store, err := m.getStore(chain)
	if err != nil {
		return nil, err
	}
	return store.Get(chain, address)
}

// UpdateBalance updates the balance for an address
func (m *MultiChainAddressStore) UpdateBalance(chain, address string, received, sent int64, incrementTx bool) error {
	store, err := m.getStore(chain)
	if err != nil {
		return err
	}
	return store.UpdateBalance(chain, address, received, sent, incrementTx)
}

// AddVinReference adds a vin reference to an address
func (m *MultiChainAddressStore) AddVinReference(chain, address, txid string, vinIndex int) error {
	store, err := m.getStore(chain)
	if err != nil {
		return err
	}
	return store.AddVinReference(chain, address, txid, vinIndex)
}

// AddVoutReference adds a vout reference to an address
func (m *MultiChainAddressStore) AddVoutReference(chain, address, txid string, voutIndex int) error {
	store, err := m.getStore(chain)
	if err != nil {
		return err
	}
	return store.AddVoutReference(chain, address, txid, voutIndex)
}

// RemoveVinReference removes a vin reference from an address
func (m *MultiChainAddressStore) RemoveVinReference(chain, address, txid string, vinIndex int) error {
	store, err := m.getStore(chain)
	if err != nil {
		return err
	}
	return store.RemoveVinReference(chain, address, txid, vinIndex)
}

// RemoveVoutReference removes a vout reference from an address
func (m *MultiChainAddressStore) RemoveVoutReference(chain, address, txid string, voutIndex int) error {
	store, err := m.getStore(chain)
	if err != nil {
		return err
	}
	return store.RemoveVoutReference(chain, address, txid, voutIndex)
}

// GetVinReferences retrieves all vin references for an address
func (m *MultiChainAddressStore) GetVinReferences(chain, address string) ([]string, error) {
	store, err := m.getStore(chain)
	if err != nil {
		return nil, err
	}
	return store.GetVinReferences(chain, address)
}

// GetVoutReferences retrieves all vout references for an address
func (m *MultiChainAddressStore) GetVoutReferences(chain, address string) ([]string, error) {
	store, err := m.getStore(chain)
	if err != nil {
		return nil, err
	}
	return store.GetVoutReferences(chain, address)
}

// MultiChainSyncStore wraps multiple chain-specific sync stores
type MultiChainSyncStore struct {
	stores map[string]*SyncStore
}

// NewMultiChainSyncStore creates a new multi-chain sync store
func NewMultiChainSyncStore() *MultiChainSyncStore {
	return &MultiChainSyncStore{
		stores: make(map[string]*SyncStore),
	}
}

// RegisterChain registers a sync store for a chain
func (m *MultiChainSyncStore) RegisterChain(chain string, store *SyncStore) {
	m.stores[chain] = store
}

// getStore returns the store for the given chain
func (m *MultiChainSyncStore) getStore(chain string) (*SyncStore, error) {
	store, ok := m.stores[chain]
	if !ok {
		return nil, fmt.Errorf("chain not registered: %s", chain)
	}
	return store, nil
}

// GetSyncedHeight retrieves the synced height for a chain
func (m *MultiChainSyncStore) GetSyncedHeight(chain string) (int64, error) {
	store, err := m.getStore(chain)
	if err != nil {
		return 0, err
	}
	return store.GetSyncedHeight(chain)
}

// SetSyncedHeight sets the synced height for a chain
func (m *MultiChainSyncStore) SetSyncedHeight(chain string, height int64) error {
	store, err := m.getStore(chain)
	if err != nil {
		return err
	}
	return store.SetSyncedHeight(chain, height)
}
