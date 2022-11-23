package cache

import (
	"time"

	policylru "github.com/gogama/policy-lru"
	"github.com/gogama/reee-evolution/daemon"
)

type Policy struct {
	MaxCount int
	MaxSize  uint64
	MaxAge   time.Duration
}

type policy struct {
	Policy
	size uint64
}

type value struct {
	msg      *daemon.Message
	size     uint64
	birthday time.Time
}

type Cache struct {
	policy policy
	lru    *policylru.Cache[string, value]
}

func New(p Policy) daemon.MessageCache {
	cache := &Cache{
		policy: policy{
			Policy: p,
		},
	}
	cache.lru = policylru.NewWithHandler[string, value](&cache.policy, &cache.policy)
	return cache
}

func (c *Cache) Get(cacheKey string) *daemon.Message {
	if v, ok := c.lru.Get(cacheKey); ok {
		return v.msg
	}
	return nil
}

func (c *Cache) Put(cacheKey string, msg *daemon.Message, size uint64) {
	c.lru.Add(cacheKey, value{msg, size, time.Now()})
}

func (p *policy) Evict(_ string, v value, n int) bool {
	if p.MaxCount > 0 && n > p.MaxCount {
		return true
	} else if p.MaxSize > 0 && p.size > p.MaxSize {
		return true
	} else if p.MaxAge > 0 && time.Since(v.birthday) > p.MaxAge {
		return true
	}
	return false
}

func (p *policy) Added(_ string, old, new value, _ bool) {
	p.size -= old.size
	p.size += new.size
}

func (p *policy) Removed(_ string, v value) {
	p.size -= v.size
}
