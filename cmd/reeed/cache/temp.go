package cache

import "github.com/jhillyerd/enmime"

type TempCache struct {
}

func (c *TempCache) Get(key string) *enmime.Envelope {
	return nil
}

func (c *TempCache) Put(key string, env *enmime.Envelope, size int64) {
	return
}
