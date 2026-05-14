// Package addresslookup provides Address Lookup Table (ALT) support for Solana transactions.
// ALT reduces transaction size by storing frequently used addresses in a lookup table.
package addresslookup

import (
	"context"
	"encoding/binary"
	"fmt"
	"sync"

	"github.com/gagliardetto/solana-go"
	addresslookuptable "github.com/gagliardetto/solana-go/programs/address-lookup-table"
	"github.com/gagliardetto/solana-go/rpc"
)

var addressLookupTableProgramID = solana.MustPublicKeyFromBase58("AddressLookupTab1e1111111111111111111111111")

// AddressLookupTableAccount represents an address lookup table account
type AddressLookupTableAccount struct {
	Key       solana.PublicKey
	Addresses []solana.PublicKey
}

// AddressLookupTableCache caches address lookup tables to avoid repeated RPC calls
type AddressLookupTableCache struct {
	mu    sync.RWMutex
	cache map[string]*AddressLookupTableAccount
}

// NewAddressLookupTableCache creates a new address lookup table cache
func NewAddressLookupTableCache() *AddressLookupTableCache {
	return &AddressLookupTableCache{
		cache: make(map[string]*AddressLookupTableAccount),
	}
}

// FetchAddressLookupTableAccount fetches an address lookup table account from the blockchain
func FetchAddressLookupTableAccount(
	ctx context.Context,
	client *rpc.Client,
	lookupTableAddress solana.PublicKey,
	commitment rpc.CommitmentType,
) (*AddressLookupTableAccount, error) {
	if commitment == "" {
		commitment = rpc.CommitmentConfirmed
	}

	accountInfo, err := client.GetAccountInfo(ctx, lookupTableAddress)
	if err != nil {
		return nil, fmt.Errorf("failed to get account info: %w", err)
	}

	if accountInfo == nil || accountInfo.Value == nil {
		return nil, nil
	}

	data := accountInfo.Value.Data.GetBinary()
	if len(data) < 56 {
		return nil, fmt.Errorf("invalid lookup table data size: %d", len(data))
	}

	state, err := addresslookuptable.DecodeAddressLookupTableState(data)
	if err != nil {
		return nil, fmt.Errorf("failed to decode lookup table: %w", err)
	}

	return &AddressLookupTableAccount{
		Key:       lookupTableAddress,
		Addresses: []solana.PublicKey(state.Addresses),
	}, nil
}

// GetLookupTable gets lookup table from cache or fetches from RPC
func (c *AddressLookupTableCache) GetLookupTable(
	ctx context.Context,
	client *rpc.Client,
	lookupTableAddress solana.PublicKey,
) (*AddressLookupTableAccount, error) {
	key := lookupTableAddress.String()

	c.mu.RLock()
	if cached, ok := c.cache[key]; ok {
		c.mu.RUnlock()
		return cached, nil
	}
	c.mu.RUnlock()

	lookupTable, err := FetchAddressLookupTableAccount(ctx, client, lookupTableAddress, "")
	if err != nil {
		return nil, err
	}

	if lookupTable != nil {
		c.mu.Lock()
		c.cache[key] = lookupTable
		c.mu.Unlock()
	}

	return lookupTable, nil
}

// Clear clears the cache
func (c *AddressLookupTableCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cache = make(map[string]*AddressLookupTableAccount)
}

// Remove removes a specific lookup table from cache
func (c *AddressLookupTableCache) Remove(lookupTableAddress solana.PublicKey) {
	key := lookupTableAddress.String()
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.cache, key)
}

// CompileLookupTableAddresses compiles addresses from lookup tables for versioned transactions
func CompileLookupTableAddresses(
	lookupTables []*AddressLookupTableAccount,
	requiredAddresses []solana.PublicKey,
) ([][]uint16, error) {
	// Maps to track address indices in lookup tables
	writableIndices := make([][]uint16, len(lookupTables))
	// Track which addresses are found
	foundWritable := make(map[solana.PublicKey]bool)

	for _, addr := range requiredAddresses {
		for tableIdx, table := range lookupTables {
			for addrIdx, tableAddr := range table.Addresses {
				if tableAddr.Equals(addr) {
					if !foundWritable[addr] {
						writableIndices[tableIdx] = append(writableIndices[tableIdx], uint16(addrIdx))
						foundWritable[addr] = true
					}
				}
			}
		}
	}

	return writableIndices, nil
}

// LookupTableBuilder helps build address lookup tables for transactions
type LookupTableBuilder struct {
	addresses map[string]bool // track unique addresses
}

// NewLookupTableBuilder creates a new lookup table builder
func NewLookupTableBuilder() *LookupTableBuilder {
	return &LookupTableBuilder{
		addresses: make(map[string]bool),
	}
}

// AddAddress adds an address to the builder
func (b *LookupTableBuilder) AddAddress(addr solana.PublicKey) {
	b.addresses[addr.String()] = true
}

// AddAddresses adds multiple addresses to the builder
func (b *LookupTableBuilder) AddAddresses(addresses ...solana.PublicKey) {
	for _, addr := range addresses {
		b.addresses[addr.String()] = true
	}
}

// GetAddresses returns all collected addresses
func (b *LookupTableBuilder) GetAddresses() []solana.PublicKey {
	addresses := make([]solana.PublicKey, 0, len(b.addresses))
	for addrStr := range b.addresses {
		addr := solana.MustPublicKeyFromBase58(addrStr)
		addresses = append(addresses, addr)
	}
	return addresses
}

// Count returns the number of unique addresses
func (b *LookupTableBuilder) Count() int {
	return len(b.addresses)
}

// Clear clears all addresses
func (b *LookupTableBuilder) Clear() {
	b.addresses = make(map[string]bool)
}

// DeriveLookupTableAddress derives the address of an address lookup table
func DeriveLookupTableAddress(
	authority solana.PublicKey,
	nonce uint64,
) (solana.PublicKey, uint8, error) {
	nonceBytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(nonceBytes, nonce)

	seeds := [][]byte{
		[]byte("lookup-table"),
		nonceBytes[:],
		authority[:],
	}

	return solana.FindProgramAddress(seeds, addressLookupTableProgramID)
}
