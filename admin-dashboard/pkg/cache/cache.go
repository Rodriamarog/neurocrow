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

type Cache struct {
	messages   map[string]cacheEntry[[]models.Message]
	profiles   map[string]cacheEntry[string]
	mu         sync.RWMutex
	defaultTTL time.Duration
}

type cacheEntry[T any] struct {
	value      T
	expiration time.Time
}

func New() *Cache {
	return &Cache{
		messages:   make(map[string]cacheEntry[[]models.Message]),
		profiles:   make(map[string]cacheEntry[string]),
		defaultTTL: 5 * time.Minute,
	}
}

func (c *Cache) GetMessages(threadID string) ([]models.Message, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, exists := c.messages[threadID]
	if !exists || time.Now().After(entry.expiration) {
		return nil, ErrNotFound
	}
	return entry.value, nil
}

func (c *Cache) SetMessages(threadID string, messages []models.Message) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.messages[threadID] = cacheEntry[[]models.Message]{
		value:      messages,
		expiration: time.Now().Add(c.defaultTTL),
	}
}

func (c *Cache) GetProfile(threadID string) (string, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, exists := c.profiles[threadID]
	if !exists || time.Now().After(entry.expiration) {
		return "", ErrNotFound
	}
	return entry.value, nil
}

func (c *Cache) SetProfile(threadID string, profileURL string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.profiles[threadID] = cacheEntry[string]{
		value:      profileURL,
		expiration: time.Now().Add(c.defaultTTL),
	}
}

func (c *Cache) InvalidateProfile(threadID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.profiles, threadID)
}
