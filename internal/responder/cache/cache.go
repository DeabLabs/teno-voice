package cache

import (
	"encoding/json"
	"log"
)

type Cache struct {
	CacheItems []CacheItem `json:"cacheItems"`
}

type CacheItem struct {
	Name    string `json:"name"`
	Content string `json:"content"`
}

func NewCache() *Cache {
	return &Cache{CacheItems: []CacheItem{}}
}

func (c *Cache) AddOrUpdateItem(newItem CacheItem) {
	for i, item := range c.CacheItems {
		if item.Name == newItem.Name {
			c.CacheItems[i].Content = newItem.Content
			return
		}
	}
	c.CacheItems = append(c.CacheItems, newItem)
}

func (c *Cache) RemoveItem(index int) {
	c.CacheItems = append(c.CacheItems[:index], c.CacheItems[index+1:]...)
}

func (c *Cache) Clear() {
	c.CacheItems = []CacheItem{}
}

func (c *Cache) RenderForPrompt() string {
	// Transform the Cache into a slice of CacheItems
	cacheItems := make([]CacheItem, len(c.CacheItems))
	for i, item := range c.CacheItems {
		// Copy only the Name and Content fields to the new slice
		cacheItems[i] = CacheItem{
			Name:    item.Name,
			Content: item.Content,
		}
	}

	jsonCache, err := json.Marshal(cacheItems)
	if err != nil {
		log.Fatal(err)
	}

	return string(jsonCache)
}
