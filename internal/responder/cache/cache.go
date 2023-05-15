package cache

import (
	"encoding/json"
	"log"
)

type Cache struct {
	// Array of CacheItems
	CacheItems []CacheItem `json:"cacheItems"`
}

type CacheItem struct {
	Type      string `json:"type"`
	Permanent bool   `json:"permanent"`
	Content   string `json:"content"`
}

type IDCacheItem struct {
	ID   int       `json:"id"`
	Item CacheItem `json:"item"`
}

func NewCache() *Cache {
	return &Cache{CacheItems: []CacheItem{}}
}

func (c *Cache) AddItem(cacheItem CacheItem) {
	c.CacheItems = append(c.CacheItems, cacheItem)
}

func (c *Cache) RemoveItem(index int) {
	if c.CacheItems[index].Permanent {
		log.Printf("Tried to remove permanent cache item at index %d", index)
		return
	} else {
		// Remove the item at the given index.
		c.CacheItems = append(c.CacheItems[:index], c.CacheItems[index+1:]...)
	}
}

func (c *Cache) Clear() {
	c.CacheItems = []CacheItem{}
}

func (c *Cache) RenderForPrompt() string {
	// Transform the Cache into a slice of IDCacheItems
	idCacheItems := make([]IDCacheItem, len(c.CacheItems))
	for i, item := range c.CacheItems {
		idCacheItems[i] = IDCacheItem{
			ID:   i,
			Item: item,
		}
	}

	jsonCache, err := json.Marshal(idCacheItems)
	if err != nil {
		log.Fatal(err)
	}

	return string(jsonCache)
}
