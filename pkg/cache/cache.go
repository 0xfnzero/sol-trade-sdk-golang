package cache

import (
	"sync"
	"sync/atomic"
	"time"
)

// ===== LRU Cache Implementation =====

// Entry represents a cache entry
type Entry struct {
	Key        string
	Value      interface{}
	Expiration int64
	Prev, Next *Entry
}

// LRUCache is a thread-safe LRU cache with TTL support
type LRUCache struct {
	mu        sync.RWMutex
	entries   map[string]*Entry
	head      *Entry
	tail      *Entry
	maxSize   int
	ttl       time.Duration
	hits      int64
	misses    int64
	evictions int64
}

// NewLRUCache creates a new LRU cache
func NewLRUCache(maxSize int, ttl time.Duration) *LRUCache {
	cache := &LRUCache{
		entries: make(map[string]*Entry),
		maxSize: maxSize,
		ttl:     ttl,
	}
	return cache
}

// Get retrieves a value from the cache
func (c *LRUCache) Get(key string) (interface{}, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	entry, ok := c.entries[key]
	if !ok {
		atomic.AddInt64(&c.misses, 1)
		return nil, false
	}

	// Check expiration
	if time.Now().UnixNano() > entry.Expiration {
		c.removeEntry(entry)
		atomic.AddInt64(&c.misses, 1)
		return nil, false
	}

	// Move to front
	c.moveToFront(entry)
	atomic.AddInt64(&c.hits, 1)
	return entry.Value, true
}

// Set stores a value in the cache
func (c *LRUCache) Set(key string, value interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now().UnixNano()
	expiration := now + int64(c.ttl)

	// Check if key exists
	if entry, ok := c.entries[key]; ok {
		entry.Value = value
		entry.Expiration = expiration
		c.moveToFront(entry)
		return
	}

	// Create new entry
	entry := &Entry{
		Key:        key,
		Value:      value,
		Expiration: expiration,
	}
	c.entries[key] = entry
	c.addToFront(entry)

	// Evict if necessary
	if len(c.entries) > c.maxSize {
		c.evict()
	}
}

// Delete removes a key from the cache
func (c *LRUCache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if entry, ok := c.entries[key]; ok {
		c.removeEntry(entry)
	}
}

// Clear clears all entries from the cache
func (c *LRUCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries = make(map[string]*Entry)
	c.head = nil
	c.tail = nil
}

// Stats returns cache statistics
func (c *LRUCache) Stats() (hits, misses, evictions int64, size int) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.hits, c.misses, c.evictions, len(c.entries)
}

// HitRate returns the cache hit rate
func (c *LRUCache) HitRate() float64 {
	hits := atomic.LoadInt64(&c.hits)
	misses := atomic.LoadInt64(&c.misses)
	total := hits + misses
	if total == 0 {
		return 0
	}
	return float64(hits) / float64(total)
}

func (c *LRUCache) moveToFront(entry *Entry) {
	if c.head == entry {
		return
	}
	c.removeEntry(entry)
	c.addToFront(entry)
}

func (c *LRUCache) addToFront(entry *Entry) {
	entry.Prev = nil
	entry.Next = c.head
	if c.head != nil {
		c.head.Prev = entry
	}
	c.head = entry
	if c.tail == nil {
		c.tail = entry
	}
}

func (c *LRUCache) removeEntry(entry *Entry) {
	if entry.Prev != nil {
		entry.Prev.Next = entry.Next
	} else {
		c.head = entry.Next
	}
	if entry.Next != nil {
		entry.Next.Prev = entry.Prev
	} else {
		c.tail = entry.Prev
	}
	delete(c.entries, entry.Key)
}

func (c *LRUCache) evict() {
	if c.tail != nil {
		c.removeEntry(c.tail)
		atomic.AddInt64(&c.evictions, 1)
	}
}

// ===== TTL Cache Implementation =====

// TTLCache is a simple TTL cache with background cleanup
type TTLCache struct {
	mu      sync.RWMutex
	entries map[string]*tttEntry
	ttl     time.Duration
	hits    int64
	misses  int64
}

type tttEntry struct {
	value      interface{}
	expiration int64
}

// NewTTLCache creates a new TTL cache
func NewTTLCache(ttl time.Duration) *TTLCache {
	return &TTLCache{
		entries: make(map[string]*tttEntry),
		ttl:     ttl,
	}
}

// Get retrieves a value from the cache
func (c *TTLCache) Get(key string) (interface{}, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, ok := c.entries[key]
	if !ok {
		atomic.AddInt64(&c.misses, 1)
		return nil, false
	}

	if time.Now().UnixNano() > entry.expiration {
		atomic.AddInt64(&c.misses, 1)
		return nil, false
	}

	atomic.AddInt64(&c.hits, 1)
	return entry.value, true
}

// Set stores a value in the cache
func (c *TTLCache) Set(key string, value interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries[key] = &tttEntry{
		value:      value,
		expiration: time.Now().UnixNano() + int64(c.ttl),
	}
}

// Delete removes a key from the cache
func (c *TTLCache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.entries, key)
}

// Cleanup removes expired entries
func (c *TTLCache) Cleanup() {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now().UnixNano()
	for key, entry := range c.entries {
		if now > entry.expiration {
			delete(c.entries, key)
		}
	}
}

// ===== Sharded Cache for High Concurrency =====

// ShardedCache distributes keys across multiple shards for better concurrency
type ShardedCache struct {
	shards []*LRUCache
	mask   uint64
}

// NewShardedCache creates a new sharded cache
func NewShardedCache(shards, maxSizePerShard int, ttl time.Duration) *ShardedCache {
	c := &ShardedCache{
		shards: make([]*LRUCache, shards),
		mask:   uint64(shards - 1),
	}
	for i := 0; i < shards; i++ {
		c.shards[i] = NewLRUCache(maxSizePerShard, ttl)
	}
	return c
}

// Get retrieves a value from the cache
func (c *ShardedCache) Get(key string) (interface{}, bool) {
	return c.getShard(key).Get(key)
}

// Set stores a value in the cache
func (c *ShardedCache) Set(key string, value interface{}) {
	c.getShard(key).Set(key, value)
}

// Delete removes a key from the cache
func (c *ShardedCache) Delete(key string) {
	c.getShard(key).Delete(key)
}

// Stats returns aggregated statistics
func (c *ShardedCache) Stats() (hits, misses, evictions int64, size int) {
	for _, shard := range c.shards {
		h, m, e, s := shard.Stats()
		hits += h
		misses += m
		evictions += e
		size += s
	}
	return
}

func (c *ShardedCache) getShard(key string) *LRUCache {
	// FNV-1a hash
	var hash uint64
	for _, c := range key {
		hash ^= uint64(c)
		hash *= 16777619
	}
	return c.shards[hash&c.mask]
}

// ===== Atomic Value Cache =====

// AtomicCache provides lock-free reads for cached values
type AtomicCache struct {
	value atomic.Value
	ttl   time.Duration
	mu    sync.Mutex
}

type atomicEntry struct {
	value      interface{}
	expiration int64
}

// NewAtomicCache creates a new atomic cache
func NewAtomicCache(ttl time.Duration) *AtomicCache {
	return &AtomicCache{ttl: ttl}
}

// Get retrieves the cached value
func (c *AtomicCache) Get() (interface{}, bool) {
	v := c.value.Load()
	if v == nil {
		return nil, false
	}
	entry := v.(*atomicEntry)
	if time.Now().UnixNano() > entry.expiration {
		return nil, false
	}
	return entry.value, true
}

// Set stores a value
func (c *AtomicCache) Set(value interface{}) {
	c.value.Store(&atomicEntry{
		value:      value,
		expiration: time.Now().UnixNano() + int64(c.ttl),
	})
}
