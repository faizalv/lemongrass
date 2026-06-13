package usecase

import (
	"container/list"
	"sync"
	"time"
)

const cacheTTL = 60 * time.Second

type cacheEntry struct {
	key       string
	projectID int64
	data      string
	size      int
	expiresAt time.Time
	elem      *list.Element
}

type lruCache struct {
	mu        sync.Mutex
	entries   map[string]*cacheEntry
	lru       *list.List
	totalSize int
	maxSize   int
}

func newLRUCache(maxSize int) *lruCache {
	return &lruCache{
		entries: make(map[string]*cacheEntry),
		lru:     list.New(),
		maxSize: maxSize,
	}
}

func (c *lruCache) get(key string) (string, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.entries[key]
	if !ok {
		return "", false
	}
	if time.Now().After(e.expiresAt) {
		c.lru.Remove(e.elem)
		delete(c.entries, e.key)
		c.totalSize -= e.size
		return "", false
	}
	c.lru.MoveToFront(e.elem)
	return e.data, true
}

func (c *lruCache) set(key string, projectID int64, data string) {
	size := len(data)
	c.mu.Lock()
	defer c.mu.Unlock()
	if e, ok := c.entries[key]; ok {
		c.lru.Remove(e.elem)
		delete(c.entries, e.key)
		c.totalSize -= e.size
	}
	for c.totalSize+size > c.maxSize && c.lru.Len() > 0 {
		oldest := c.lru.Back()
		if oldest == nil {
			break
		}
		old := oldest.Value.(*cacheEntry)
		c.lru.Remove(old.elem)
		delete(c.entries, old.key)
		c.totalSize -= old.size
	}
	e := &cacheEntry{
		key:       key,
		projectID: projectID,
		data:      data,
		size:      size,
		expiresAt: time.Now().Add(cacheTTL),
	}
	e.elem = c.lru.PushFront(e)
	c.entries[key] = e
	c.totalSize += size
}

func (c *lruCache) invalidateProject(projectID int64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for k, e := range c.entries {
		if e.projectID == projectID {
			c.lru.Remove(e.elem)
			delete(c.entries, k)
			c.totalSize -= e.size
		}
	}
}

type sfCall struct {
	wg  sync.WaitGroup
	val string
}

type singleflightGroup struct {
	mu sync.Mutex
	m  map[string]*sfCall
}

func (g *singleflightGroup) Do(key string, fn func() string) string {
	g.mu.Lock()
	if g.m == nil {
		g.m = make(map[string]*sfCall)
	}
	if c, ok := g.m[key]; ok {
		g.mu.Unlock()
		c.wg.Wait()
		return c.val
	}
	c := &sfCall{}
	c.wg.Add(1)
	g.m[key] = c
	g.mu.Unlock()

	c.val = fn()
	c.wg.Done()

	g.mu.Lock()
	delete(g.m, key)
	g.mu.Unlock()
	return c.val
}
