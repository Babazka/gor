package replay

import (
	"net"
	"log"
)

type TcpPool struct {
	pool chan int
}

func NewTcpPool(size int) *TcpPool {
	p := &TcpPool{}
	p.pool = make(chan int, size)
	for i := 0; i < size; i++ {
		p.pool <- i
	}
	return p
}

func (p *TcpPool) Get() int {
	return <-p.pool
}

func (p *TcpPool) Connect(addr string) (net.Conn, error) {
	p.Get()
	//conn, err := net.DialTimeout("tcp", addr, 3 * time.Second)
	remote_addr, err := net.ResolveTCPAddr("tcp", addr)
	if err != nil {
		log.Fatal(err)
	}
	laddr, err := net.ResolveTCPAddr("tcp", Settings.ThisSideAddress + ":0")
	if err != nil {
		log.Fatal(err)
	}
	//conn, err := net.DialTCP("tcp", addr, 3 * time.Second)

	conn, err := net.DialTCP("tcp", laddr, remote_addr)
	return conn, err
}

func (p *TcpPool) ReturnToPool(conn net.Conn) {
	p.pool <- 0
}
