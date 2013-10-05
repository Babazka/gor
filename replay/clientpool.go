// A pool of HTTP clients, intended to support keep-alive connections to a single upstream server
//
// Use pool.GetClient() to get a usable http.Client and a sync.Mutex, which can be *optionally* used
// to prevent other goroutines from reusing this Client.
// The pool itself does *not* call Lock() or Unlock() on the mutex.
//
// http.Client itself is goroutine-safe: when used by multiple goroutines, it will create
// new TCP connections. A limit on the number of *idle* connections (MaxIdleConnsPerHost) can be imposed,
// but not on the total number, so http.Client cannot act as a fixed-size pool; thus the necessity of this module.


package replay

import (
	"net/http"
	"sync"
	"errors"
)

type ClientPool struct {
	sync.Mutex
	size int
	round_robin_index int
	clients []*http.Client
	locks []sync.Mutex
}

func NewClientPool(size int) *ClientPool {
	var pool ClientPool
	pool.size = size
	pool.round_robin_index = 0
	pool.clients = make([]*http.Client, size, size)
	pool.locks = make([]sync.Mutex, size, size)

	checkRedirects := func (req *http.Request, via []*http.Request) error {
		if len(via) >= 0 {
			return errors.New("stopped after a redirect")
		}
		return nil
	}

	for i, _ := range(pool.clients) {
		pool.clients[i] = &http.Client{
			CheckRedirect: checkRedirects,
		}
	}
	return &pool
}

// Returns a client to use (according to round-robin balancing)
func (self *ClientPool) GetClient() (*http.Client, *sync.Mutex) {
	self.Lock()
	defer self.Unlock()
	client_number := self.round_robin_index
	self.round_robin_index = (self.round_robin_index + 1) % self.size
	return self.clients[client_number], &self.locks[client_number]
}
