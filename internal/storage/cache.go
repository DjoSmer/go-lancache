package storage

import (
	"log/slog"
	"sync"
	"time"
)

type cacheItem struct {
	value     Storage
	createdAt time.Time
	expiresAt time.Time
	node      *cacheNode
}
type cacheNode struct {
	Key       string
	Prev      *cacheNode
	Next      *cacheNode
	expiresAt time.Time
}

type StorageCache struct {
	mu         sync.Mutex
	items      map[string]*cacheItem
	first      *cacheNode
	last       *cacheNode
	ttl        time.Duration
	stopWorker chan struct{}
	OnEvict    func(Storage)
}

var defaultItemsSize = 10000

func NewStorageCache(ttl time.Duration) *StorageCache {
	cache := &StorageCache{
		items:      make(map[string]*cacheItem, defaultItemsSize),
		ttl:        ttl,
		stopWorker: make(chan struct{}),
	}

	return cache
}

func (c *StorageCache) EvictAll() {
	c.mu.Lock()

	items := c.items
	c.items = make(map[string]*cacheItem, defaultItemsSize)
	c.first = nil
	c.last = nil

	c.mu.Unlock()

	for _, item := range items {
		c.OnEvict(item.value)
	}
}

func (c *StorageCache) Get(key string) (Storage, bool) {
	log := c.log("Get")

	c.mu.Lock()
	defer c.mu.Unlock()

	item, ok := c.items[key]
	if ok {
		c.upItem(key, item)
		log.Debug("Get: " + key)
		return item.value, ok
	}

	return Storage{}, ok
}

func (c *StorageCache) Set(key string, storage Storage) {
	log := c.log("Set")

	c.mu.Lock()
	defer c.mu.Unlock()

	item, ok := c.items[key]
	if !ok {
		item = &cacheItem{
			createdAt: time.Now(),
		}
	}

	item.value = storage
	log.Debug("Set: " + key)

	c.upItem(key, item)
	c.items[key] = item
}

func (c *StorageCache) Update(key string, updateFn func(s *Storage) *Storage) *Storage {
	log := c.log("Update")

	c.mu.Lock()
	defer c.mu.Unlock()

	item, ok := c.items[key]
	if !ok {
		log.Debug("Not found in cache")
		return nil
	}

	oldStorage := item.value
	storage := item.value

	newStorage := updateFn(&storage)
	if newStorage != nil {
		log.Debug("Updated", slog.Any("old", oldStorage), slog.Any("new", newStorage))
		item.value = *newStorage
		c.upItem(key, item)

		return newStorage
	}

	return &storage
}

func (c *StorageCache) startNodeWorker() {
	log := c.log("startNodeWorker")

	for {
		select {
		case <-c.stopWorker:
			return
		default:
		}

		c.mu.Lock()
		var node cacheNode

		if c.first != nil {
			node = *c.first
		} else {
			node = cacheNode{
				Key:       "skip",
				expiresAt: time.Now().Add(c.ttl),
			}
		}
		c.mu.Unlock()

		log.Debug("Node wait: "+node.Key, slog.Duration("d", time.Until(node.expiresAt)+(100*time.Millisecond)))

		if node.expiresAt.After(time.Now()) {
			select {
			case <-c.stopWorker:
				return
			case <-time.After(time.Until(node.expiresAt) + (100 * time.Millisecond)):
			}
		}

		if node.Key == "skip" {
			continue
		}

		log.Debug("Node continue: " + node.Key)

		c.mu.Lock()
		if c.first == nil || c.first.Key != node.Key {
			c.mu.Unlock()
			continue
		}

		if c.first == c.last {
			c.last = nil
		}

		c.first = c.first.Next
		if c.first != nil {
			c.first.Prev = nil
		}

		toEvict := false
		item, ok := c.items[node.Key]
		if ok && item.expiresAt.Compare(node.expiresAt) != 1 {
			log.Debug("Node delete: " + node.Key)

			if item.value.Dirty && c.OnEvict != nil {
				toEvict = true
			}

			delete(c.items, node.Key)
		}

		c.mu.Unlock()

		if toEvict {
			go c.OnEvict(item.value)
		}
	}
}

func (c *StorageCache) Start() {
	go c.startNodeWorker()
}

func (c *StorageCache) Stop() {
	close(c.stopWorker)
}

func (c *StorageCache) upItem(key string, item *cacheItem) {
	item.expiresAt = time.Now().Add(c.ttl)

	if item.node == nil {
		item.node = &cacheNode{
			Key:       key,
			expiresAt: item.expiresAt,
			Prev:      c.last,
		}
	}
	item.node.expiresAt = item.expiresAt

	if item.node == c.last {
		return
	}

	if c.first == item.node {
		c.first = item.node.Next
	}

	if c.first == nil {
		c.first = item.node
	}

	if item.node.Prev != nil {
		item.node.Prev.Next = item.node.Next
	}

	if item.node.Next != nil {
		item.node.Next.Prev = item.node.Prev
		item.node.Next = nil
	}

	item.node.Prev = c.last

	if c.last != nil {
		c.last.Next = item.node
	}

	c.last = item.node
	c.last.Next = nil
}

func (c *StorageCache) log(name string) *slog.Logger {
	op := "StorageCache." + name
	return slog.Default().With(slog.String("op", op))
}
