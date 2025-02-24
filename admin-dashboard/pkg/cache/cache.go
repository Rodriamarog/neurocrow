package cache

import (
	"admin-dashboard/models"
	"fmt"
	"sync"
	"time"
)

var (
	ErrNotFound = fmt.Errorf("not found in cache")
)

type CacheItem struct {
	Value      interface{}
	Expiration time.Time
}

type Cache struct {
	items map[string]CacheItem
	mu    sync.RWMutex
}

func New() *Cache {
	return &Cache{
		items: make(map[string]CacheItem),
	}
}

func (c *Cache) Get(key string) (interface{}, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	item, exists := c.items[key]
	if !exists {
		return nil, false
	}

	if time.Now().After(item.Expiration) {
		delete(c.items, key)
		return nil, false
	}

	return item.Value, true
}

func (c *Cache) SetWithTTL(key string, value interface{}, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.items[key] = CacheItem{
		Value:      value,
		Expiration: time.Now().Add(ttl),
	}
}

func (c *Cache) GetMessages(threadID string) ([]models.Message, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, exists := c.items[threadID]
	if !exists || time.Now().After(entry.Expiration) {
		return nil, ErrNotFound
	}
	return entry.Value.([]models.Message), nil
}

func (c *Cache) SetMessages(threadID string, messages []models.Message) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.items[threadID] = CacheItem{
		Value:      messages,
		Expiration: time.Now().Add(5 * time.Minute),
	}
}

func (c *Cache) GetProfile(threadID string) (string, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, exists := c.items[threadID]
	if !exists || time.Now().After(entry.Expiration) {
		return "", ErrNotFound
	}
	return entry.Value.(string), nil
}

func (c *Cache) SetProfile(threadID string, profileURL string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.items[threadID] = CacheItem{
		Value:      profileURL,
		Expiration: time.Now().Add(5 * time.Minute),
	}
}

func (c *Cache) InvalidateProfile(threadID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.items, threadID)
}
