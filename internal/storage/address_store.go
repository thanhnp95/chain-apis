package storage

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/thanhnp/chain-apis/internal/models"
)

// AddressStore handles address storage operations
type AddressStore struct {
	db *RocksDB
}

// NewAddressStore creates a new AddressStore
func NewAddressStore(db *RocksDB) *AddressStore {
	return &AddressStore{db: db}
}

// addressKey creates a key for the addresses column family
func addressKey(chain, address string) []byte {
	return []byte(fmt.Sprintf("%s:%s", chain, address))
}

// addressVinKey creates a key for the address_vins column family
func addressVinKey(chain, address, txid string, index int) []byte {
	return []byte(fmt.Sprintf("%s:%s:%s:%06d", chain, address, txid, index))
}

// addressVoutKey creates a key for the address_vouts column family
func addressVoutKey(chain, address, txid string, index int) []byte {
	return []byte(fmt.Sprintf("%s:%s:%s:%06d", chain, address, txid, index))
}

// addressPrefix creates a prefix for all entries of an address
func addressPrefix(chain, address string) []byte {
	return []byte(fmt.Sprintf("%s:%s:", chain, address))
}

// Save stores an address in the database
func (s *AddressStore) Save(addr *models.Address) error {
	data, err := json.Marshal(addr)
	if err != nil {
		return fmt.Errorf("failed to marshal address: %w", err)
	}

	return s.db.Put(CFAddresses, addressKey(addr.Chain, addr.Address), data)
}

// Get retrieves an address by its value
func (s *AddressStore) Get(chain, address string) (*models.Address, error) {
	data, err := s.db.Get(CFAddresses, addressKey(chain, address))
	if err != nil {
		return nil, err
	}
	if data == nil {
		return nil, nil
	}

	var addr models.Address
	if err := json.Unmarshal(data, &addr); err != nil {
		return nil, fmt.Errorf("failed to unmarshal address: %w", err)
	}
	return &addr, nil
}

// GetOrCreate retrieves an address or creates a new one if it doesn't exist
func (s *AddressStore) GetOrCreate(chain, address string) (*models.Address, error) {
	addr, err := s.Get(chain, address)
	if err != nil {
		return nil, err
	}
	if addr != nil {
		return addr, nil
	}

	return &models.Address{
		Address: address,
		Chain:   chain,
	}, nil
}

// AddVinReference adds a vin reference to an address
func (s *AddressStore) AddVinReference(chain, address, txid string, index int) error {
	key := addressVinKey(chain, address, txid, index)
	ref := fmt.Sprintf("%s:%d", txid, index)
	return s.db.Put(CFAddressVins, key, []byte(ref))
}

// AddVoutReference adds a vout reference to an address
func (s *AddressStore) AddVoutReference(chain, address, txid string, index int) error {
	key := addressVoutKey(chain, address, txid, index)
	ref := fmt.Sprintf("%s:%d", txid, index)
	return s.db.Put(CFAddressVouts, key, []byte(ref))
}

// RemoveVinReference removes a vin reference from an address
func (s *AddressStore) RemoveVinReference(chain, address, txid string, index int) error {
	return s.db.Delete(CFAddressVins, addressVinKey(chain, address, txid, index))
}

// RemoveVoutReference removes a vout reference from an address
func (s *AddressStore) RemoveVoutReference(chain, address, txid string, index int) error {
	return s.db.Delete(CFAddressVouts, addressVoutKey(chain, address, txid, index))
}

// GetVinReferences retrieves all vin references for an address
func (s *AddressStore) GetVinReferences(chain, address string) ([]string, error) {
	prefix := addressPrefix(chain, address)
	iter, err := s.db.NewPrefixIterator(CFAddressVins, prefix)
	if err != nil {
		return nil, err
	}
	defer iter.Close()

	var refs []string
	for ; iter.Valid(); iter.Next() {
		key := iter.Key()
		if !bytes.HasPrefix(key, prefix) {
			break
		}

		value := iter.Value()
		refs = append(refs, string(value))
	}

	return refs, nil
}

// GetVoutReferences retrieves all vout references for an address
func (s *AddressStore) GetVoutReferences(chain, address string) ([]string, error) {
	prefix := addressPrefix(chain, address)
	iter, err := s.db.NewPrefixIterator(CFAddressVouts, prefix)
	if err != nil {
		return nil, err
	}
	defer iter.Close()

	var refs []string
	for ; iter.Valid(); iter.Next() {
		key := iter.Key()
		if !bytes.HasPrefix(key, prefix) {
			break
		}

		value := iter.Value()
		refs = append(refs, string(value))
	}

	return refs, nil
}

// UpdateBalance updates an address balance based on received/sent amounts
func (s *AddressStore) UpdateBalance(chain, address string, received, sent int64, incrementTxCount bool) error {
	addr, err := s.GetOrCreate(chain, address)
	if err != nil {
		return err
	}

	addr.TotalReceived += received
	addr.TotalSent += sent
	addr.Balance = addr.TotalReceived - addr.TotalSent
	if incrementTxCount {
		addr.TxCount++
	}

	return s.Save(addr)
}

// Delete removes an address from the database
func (s *AddressStore) Delete(chain, address string) error {
	return s.db.Delete(CFAddresses, addressKey(chain, address))
}
