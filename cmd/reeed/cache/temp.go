package cache

import (
	"github.com/gogama/reee-evolution/daemon"
)

type TempCache struct {
}

func (c *TempCache) Get(key string) *daemon.Message {
	return nil
}

func (c *TempCache) Put(key string, msg *daemon.Message, size int64) {
	return
}
