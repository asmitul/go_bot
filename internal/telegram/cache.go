package telegram

import (
	"fmt"
	"sync"
	"time"

	paymentservice "go_bot/internal/payment/service"
	"go_bot/internal/telegram/models"
)

type orderLookupCacheEntry struct {
	order   *paymentservice.Order
	found   bool
	expires time.Time
}

type orderLookupCache struct {
	mu     sync.RWMutex
	ttl    time.Duration
	values map[string]orderLookupCacheEntry
}

func newOrderLookupCache(ttl time.Duration) *orderLookupCache {
	if ttl <= 0 {
		return nil
	}
	return &orderLookupCache{
		ttl:    ttl,
		values: make(map[string]orderLookupCacheEntry),
	}
}

func (c *orderLookupCache) buildKey(merchantID int64, order string) string {
	return fmt.Sprintf("%d:%s", merchantID, order)
}

func (c *orderLookupCache) Get(merchantID int64, order string) (*paymentservice.Order, bool, bool) {
	if c == nil {
		return nil, false, false
	}

	key := c.buildKey(merchantID, order)

	c.mu.RLock()
	entry, ok := c.values[key]
	c.mu.RUnlock()

	if !ok {
		return nil, false, false
	}

	if time.Now().After(entry.expires) {
		c.mu.Lock()
		delete(c.values, key)
		c.mu.Unlock()
		return nil, false, false
	}

	return entry.order, entry.found, true
}

func (c *orderLookupCache) Set(merchantID int64, order string, value *paymentservice.Order, found bool) {
	if c == nil {
		return
	}

	key := c.buildKey(merchantID, order)

	c.mu.Lock()
	c.values[key] = orderLookupCacheEntry{
		order:   value,
		found:   found,
		expires: time.Now().Add(c.ttl),
	}
	c.mu.Unlock()
}

type groupCacheEntry struct {
	group   *models.Group
	expires time.Time
}

type groupCache struct {
	mu     sync.RWMutex
	ttl    time.Duration
	values map[int64]groupCacheEntry
}

func newGroupCache(ttl time.Duration) *groupCache {
	if ttl <= 0 {
		return nil
	}
	return &groupCache{
		ttl:    ttl,
		values: make(map[int64]groupCacheEntry),
	}
}

func (c *groupCache) Get(chatID int64) (*models.Group, bool) {
	if c == nil {
		return nil, false
	}

	c.mu.RLock()
	entry, ok := c.values[chatID]
	c.mu.RUnlock()

	if !ok {
		return nil, false
	}

	if time.Now().After(entry.expires) {
		c.mu.Lock()
		delete(c.values, chatID)
		c.mu.Unlock()
		return nil, false
	}

	return entry.group, true
}

func (c *groupCache) Set(chatID int64, group *models.Group) {
	if c == nil {
		return
	}

	c.mu.Lock()
	c.values[chatID] = groupCacheEntry{
		group:   group,
		expires: time.Now().Add(c.ttl),
	}
	c.mu.Unlock()
}
