package replay

import (
	"net"
	"time"
)

type TcpPool struct {
	pool chan *net.Conn
}

func NewTcpPool(size int) *TcpPool {
	p := &TcpPool{}
	p.pool = make(chan *net.Conn, size)
	for i := 0; i < size; i++ {
		p.pool <- nil
	}
	return p
}

func (p *TcpPool) Get() *net.Conn {
	return <-p.pool
}

func (p *TcpPool) Connect(addr string) (net.Conn, error) {
	p.Get()
	conn, err := net.DialTimeout("tcp", addr, 3 * time.Second)
	return conn, err
}

func (p *TcpPool) ReturnToPool(conn net.Conn) {
	p.pool <- nil
}
