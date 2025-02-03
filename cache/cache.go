package cache

import (
	"container/list"
	"sync"

	ibytes "github.com/cosmos/iavl/internal/bytes"
)

// Node represents a node eligible for caching.
type Node interface {
	GetKey() []byte
}

// Cache is an in-memory structure to persist nodes for quick access.
// Please see lruCache for more details about why we need a custom
// cache implementation.
type Cache interface {
	// Adds node to cache. If full and had to remove the oldest element,
	// returns the oldest, otherwise nil.
	// CONTRACT: node can never be nil. Otherwise, cache panics.
	Add(node Node) Node

	// Returns Node for the key, if exists. nil otherwise.
	Get(key []byte) Node

	// Has returns true if node with key exists in cache, false otherwise.
	Has(key []byte) bool

	// Remove removes node with key from cache. The removed node is returned.
	// if not in cache, return nil.
	Remove(key []byte) Node

	// Len returns the cache length.
	Len() int
}

// lruCache is an LRU cache implementation.
// The motivation for using a custom cache implementation is to
// allow for a custom max policy.
//
// Currently, the cache maximum is implemented in terms of the
// number of nodes which is not intuitive to configure.
// Instead, we are planning to add a byte maximum.
// The alternative implementations do not allow for
// customization and the ability to estimate the byte
// size of the cache.
type lruCache struct {
	//dict            map[string]*list.Element // FastNode cache.
	dict            sync.Map
	maxElementCount int        // FastNode the maximum number of nodes in the cache.
	ll              *list.List // LRU queue of cache elements. Used for deletion.
	mu              sync.Mutex
}

var _ Cache = (*lruCache)(nil)

func New(maxElementCount int) Cache {
	return &lruCache{
		maxElementCount: maxElementCount,
		ll:              list.New(),
	}
}

func (c *lruCache) Add(node Node) Node {
	key := string(node.GetKey())

	c.mu.Lock()
	defer c.mu.Unlock()
	if e, exists := c.dict.Load(key); exists {
		ele := e.(*list.Element)
		c.ll.MoveToFront(ele)
		old := ele.Value
		ele.Value = node
		return old.(Node)
	}

	ele := c.ll.PushFront(node)
	c.dict.Store(key, ele)

	if c.ll.Len() > c.maxElementCount {
		oldest := c.ll.Back()
		return c.remove(oldest)
	}
	return nil
}

func (c *lruCache) Get(key []byte) Node {
	if e, hit := c.dict.Load(string(key)); hit {
		c.mu.Lock()
		defer c.mu.Unlock()
		ele := e.(*list.Element)
		c.ll.MoveToFront(ele)
		return ele.Value.(Node)
	}
	return nil
}

func (c *lruCache) Has(key []byte) bool {
	_, exists := c.dict.Load(string(key))
	return exists
}

func (c *lruCache) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.ll.Len()
}

func (c *lruCache) Remove(key []byte) Node {
	keyS := string(key)
	if e, exists := c.dict.Load(keyS); exists {
		elem := e.(*list.Element)
		c.mu.Lock()
		defer c.mu.Unlock()
		return c.removeWithKey(elem, keyS)
	}
	return nil
}

func (c *lruCache) remove(e *list.Element) Node {
	removed := c.ll.Remove(e).(Node)
	c.dict.Delete(ibytes.UnsafeBytesToStr(removed.GetKey()))
	return removed
}

func (c *lruCache) removeWithKey(e *list.Element, key string) Node {
	removed := c.ll.Remove(e).(Node)
	c.dict.Delete(key)
	return removed
}
