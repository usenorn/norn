package forge

import (
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

type Credentials struct {
	mu     sync.Mutex
	minted map[uuid.UUID]entity.SCMCredential
}

func NewCredentials() *Credentials {
	return &Credentials{minted: make(map[uuid.UUID]entity.SCMCredential)}
}

func (c *Credentials) Get(key uuid.UUID, now time.Time) (string, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	held, found := c.minted[key]
	if !found {
		return "", false
	}

	if !held.Usable(now) {
		delete(c.minted, key)

		return "", false
	}

	return held.Token, true
}

func (c *Credentials) Put(key uuid.UUID, credential entity.SCMCredential) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.minted[key] = credential
}
