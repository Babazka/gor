// Listener capture TCP traffic using RAW SOCKETS.
// Note: it requires sudo or root access.
//
// Right now it supports only HTTP
package listener

import (
	"bufio"
	"bytes"
	"strings"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"time"
	"encoding/gob"

	"github.com/Babazka/gor/statsd"
)

// Debug enables logging only if "--verbose" flag passed
func Debug(v ...interface{}) {
	if Settings.Verbose {
		log.Println(v...)
	}
}

// ReplayServer returns a connection to the replay server and error if some
func ReplayServer() (conn net.Conn, err error) {
	// Connection to replay server
	//conn, err = net.Dial("tcp", Settings.ReplayAddress)
	//conn, err = net.Dial("unix", Settings.ReplayAddress)
	if Settings.Dgram {
		conn, err = net.Dial("unixgram", Settings.ReplayAddress)
	} else {
		conn, err = net.Dial("unix", Settings.ReplayAddress)
	}

	if err != nil {
		log.Println("Connection error ", err, Settings.ReplayAddress)
	}

	return
}

// ReplayServer returns a connection to the replay server and error if some
func ReplayServerWithNumber(index int) (conn net.Conn, err error) {
	// Connection to replay server
	//conn, err = net.Dial("tcp", Settings.ReplayAddress)
	address := fmt.Sprintf("%s.%d", Settings.ReplayAddress, index)
	if Settings.Dgram {
		conn, err = net.Dial("unixgram", address)
	} else {
		conn, err = net.Dial("unix", address)
	}

	if err != nil {
		log.Println("Connection error ", err, Settings.ReplayAddress)
	}

	return
}

// Run acts as `main` function of a listener
func Run() {
	if os.Getuid() != 0 {
		fmt.Println("Please start the listener as root or sudo!")
		fmt.Println("This is required since listener sniff traffic on given port.")
		os.Exit(1)
	}

	Settings.ReplayServer(Settings.ReplayAddressRaw)

	fmt.Println("Listening for HTTP traffic on ", Settings.Device, "with filter '", Settings.Filter, "'")
	fmt.Println("Forwarding requests to replay server:", Settings.ReplayAddress, "Limit:", Settings.ReplayLimit)

	sigchan := make(chan os.Signal, 1)
	signal.Notify(sigchan, os.Interrupt, os.Kill)

	go RunLoop()
	s := <-sigchan
	log.Println("Got signal: ", s)
}

func RunLoop() {
	// Sniffing traffic from given device
	listener := RAWTCPListen(Settings.Device)

	var connection_pool *ConnectionPool
	if Settings.PoolSize > 0 {
		connection_pool = NewConnectionPool(Settings.PoolSize)
	}

	currentTime := time.Now().UnixNano()
	currentRPS := 0

	var pkdump *os.File
	var gobcoder *gob.Encoder
	var err error

	if Settings.RecordFile != "" {
		pkdump, err = os.Create(Settings.RecordFile)
		if err != nil {
			log.Fatal("Cannot create record file:", err)
		}

		defer pkdump.Close()
		gobcoder = gob.NewEncoder(pkdump)
	}

	for {
		// Receiving TCPMessage object
		m := listener.Receive()

		if Settings.ReplayLimit != 0 {
			if (time.Now().UnixNano() - currentTime) > time.Second.Nanoseconds() {
				currentTime = time.Now().UnixNano()
				unread := listener.UnreadCount()
				log.Printf("Output RPS: %d, unread %d", currentRPS, unread)
				statsd.C.Inc("output", currentRPS, 1.0)
				statsd.C.Inc("unread", unread, 1.0)
				currentRPS = 0
			}

			if currentRPS >= Settings.ReplayLimit {
				continue
			}

			currentRPS++
		}

		if pkdump != nil {
			gobcoder.Encode(m.Bytes())
		}

		if Settings.Verbose {
			lines := strings.Split(string(m.Bytes()), "\r\n")
			log.Printf(lines[0])
		}

		if Settings.NoReplay {
			continue
		}

		if connection_pool == nil {
			go sendMessage(m)
		} else {
			//go sendMessageViaConnectionPool(m, connection_pool)
			sendMessageViaConnectionPool(m, connection_pool)
		}
	}
}

func sendMessageViaConnectionPool(m *TCPMessage, pool *ConnectionPool) {
	err := pool.SendUsingSomeConnection(m.Bytes())
	if err != nil {
		Debug("Error while sending requests", err)
		if Settings.DieOnConnectionError {
			log.Fatal("Dying on connection error: ", err)
		}
	}
}

func sendMessage(m *TCPMessage) {
	conn, err := ReplayServer()

	if err != nil {
		log.Println("Failed to send message. Replay server not respond.")
		return
	} else {
		defer conn.Close()
	}

	// For debugging purpose
	// Usually request parsing happens in replay part
	if Settings.Verbose {
		buf := bytes.NewBuffer(m.Bytes())
		reader := bufio.NewReader(buf)

		request, err := http.ReadRequest(reader)

		if err != nil {
			Debug("Error while parsing request:", err, string(m.Bytes()))
		} else {
			request.ParseMultipartForm(32 << 20)
			Debug("Forwarding request:", request)
		}
	}

	_, err = conn.Write(m.Bytes())

	if err != nil {
		log.Println("Error while sending requests", err)
	}
}
