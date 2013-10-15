// A pool of persistent connections to the replay server.
//
// The pool contains several connections, one per 'slot'. The workflow is as follows:
//
//	 1. We select a slot with pool.NextConnectionIndex(), obtaining slot's index;
//	 2. By calling pool.AcquireConnection(index) we lock the slot and get a net.Conn object
//	 3. We do something with the connection, and finally...
//	 4. We call pool.ReleaseConnection(index) to unlock the slot.
//
// The index which is used in steps 2 and 4 is the same which is obtained in the step 1.
//
// Steps 2-4 can be replaced with the helper function SendUsingSpecificConnection(index, []byte);
// and the entire workflow can be replaced with the helper function SendUsingSomeConnection([]byte).
//
// Bytes are send as arrays serialized with Gob.
// The replay server must be launched with --persistent-connections option.
//
// The pool is goroutine-safe, i.e. it can be used from several goroutines concurrently.

package listener

import (
	"net"
	"sync"
	"encoding/gob"
)

type ConnectionPool struct {
	sync.Mutex
	size int
	round_robin_index int
	connections []net.Conn
	locks []sync.Mutex
}

func NewConnectionPool(size int) *ConnectionPool {
	var pool ConnectionPool
	pool.size = size
	pool.round_robin_index = 0
	pool.connections = make([]net.Conn, size, size)
	pool.locks = make([]sync.Mutex, size, size)
	return &pool
}

// Returns next connection index to use (according to round-robin balancing)
func (self *ConnectionPool) NextConnectionIndex() int {
	self.Lock()
	defer self.Unlock()
	conn_number := self.round_robin_index
	self.round_robin_index = (self.round_robin_index + 1) % self.size
	return conn_number
}

// Recreates a connection to replay server at pool slot `index`
func (self *ConnectionPool) EstablishConnection(index int) (net.Conn, error) {
	if self.connections[index] != nil {
		self.connections[index].Close()
		self.connections[index] = nil
	}
	/*conn, err := ReplayServer()*/
	conn, err := ReplayServerWithNumber(index)
	Debug("Dialing replay server; pool connection #", index)
	if err != nil {
		return nil, err
	}
	self.connections[index] = conn
	return conn, nil
}

// Returns a ready-to-use connection at pool slot `index`, recreating it if necessary.
// Also locks this pool slot.
// You must unlock the slot manually (with ReleaseConnection),
// even if AcquireConnection returns error.
func (self *ConnectionPool) AcquireConnection(index int) (net.Conn, error) {
	self.locks[index].Lock()
	if self.connections[index] == nil {
		_, err := self.EstablishConnection(index)
		if err != nil {
			return nil, err
		}
	}
	return self.connections[index], nil
}

// Unlocks the specified pool slot.
func (self *ConnectionPool) ReleaseConnection(index int) {
	self.locks[index].Unlock()
}

// Send bytes (serialized with gob) via the connection at pool slot `index`
// Reestablishes connection if necessary.
func (self *ConnectionPool) SendUsingSpecificConnection(pool_index int, bytes []byte) error {
	conn, err := self.AcquireConnection(pool_index)
	defer self.ReleaseConnection(pool_index)

	if err != nil {
		return err
	}

	var encoder *gob.Encoder

	encoder = gob.NewEncoder(conn)
	if Settings.Dgram {
		_, err = conn.Write(bytes)
	} else {
		err = encoder.Encode(bytes)
	}

	if err == nil {
		return nil
	}
	Debug("Old connection is broken, trying to reopen. Error: ", err)
	conn.Close()
	conn, err = self.EstablishConnection(pool_index)
	if err != nil {
		self.connections[pool_index] = nil
		return err
	}

	encoder = gob.NewEncoder(conn)

	if Settings.Dgram {
		_, err = conn.Write(bytes)
	} else {
		err = encoder.Encode(bytes)
	}

	if err != nil {
		conn.Close()
		self.connections[pool_index] = nil
		return err
	}
	return nil
}

func (self *ConnectionPool) SendUsingSomeConnection(bytes []byte) error {
	index := self.NextConnectionIndex()
	return self.SendUsingSpecificConnection(index, bytes)
}
