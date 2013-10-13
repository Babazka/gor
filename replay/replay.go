// Replay server receive requests objects from Listeners and forward it to given address.
// Basic usage:
//
//     gor replay -f http://staging.server
//
//
// Rate limiting
//
// It can be useful if you want forward only part of production traffic, not to overload staging environment. You can specify desired request per second using "|" operator after server address:
//
//     # staging.server not get more than 10 requests per second
//     gor replay -f "http://staging.server|10"
//
//
// Forward to multiple addresses
//
// Just separate addresses by coma:
//    gor replay -f "http://staging.server|10,http://dev.server|20"
//
//
//  For more help run:
//
//     gor replay -h
//
package replay

import (
	"bufio"
	"time"
	"bytes"
	"strings"
	"io"
	"log"
	"net"
	"net/http"
	"encoding/gob"
	"runtime/pprof"
	"os/signal"
	"os"
)

const bufSize = 4096

// Debug enables logging only if "--verbose" flag passed
func Debug(v ...interface{}) {
	if Settings.Verbose {
		log.Println(v...)
	}
}

// ParseRequest in []byte returns a http request or an error
func ParseRequest(data []byte) (request *http.Request, err error) {
	buf := bytes.NewBuffer(data)
	reader := bufio.NewReader(buf)

	request, err = http.ReadRequest(reader)
	return
}

// Run acts as `main` function of replay
// Replay server listen to UDP traffic from Listeners
// Each request processed by RequestFactory
func Run() {
	if Settings.Cpuprofile != "" {
		pf, err := os.Create(Settings.Cpuprofile)
		if err != nil {
			log.Fatal(err)
		}
		pprof.StartCPUProfile(pf)
		defer pprof.StopCPUProfile()
	}

	if Settings.ReplayFile != "" {
		go RunFile()
	} else {
		go RunListener()
	}

	sigchan := make(chan os.Signal, 1)
	signal.Notify(sigchan, os.Interrupt, os.Kill)
	sig := <-sigchan
	log.Println("Received signal ", sig)
}

func RunListener() {
	family := "tcp"
	if strings.HasPrefix(Settings.Address, "/") {
		family = "unix"
	}
	listener, err := net.Listen(family, Settings.Address)

	log.Println("Starting replay server at ", family, ":", Settings.Address)

	if err != nil {
		log.Fatal("Can't start:", err)
	}

	for _, host := range Settings.ForwardedHosts() {
		log.Println("Forwarding requests to:", host.Url, "limit:", host.Limit)
	}

	requestFactory := NewRequestFactory()

	for {
		conn, err := listener.Accept()

		if err != nil {
			log.Println("Error while Accept()", err)
			continue
		}

		if Settings.PersistentConnections {
			go handlePersistentConnection(conn, requestFactory)
		} else {
			go handleConnection(conn, requestFactory)
		}
	}

}

func RunFile() {
	f, err := os.Open(Settings.ReplayFile)

	if err != nil {
		log.Fatal("cannot open replay file: ", err)
	}

	rf := NewRequestFactory()

	currentTime := time.Now().UnixNano()
	currentRPS := 0

	decoder := gob.NewDecoder(f)
	i := 0
	for {
		var msg []byte
		err = decoder.Decode(&msg)
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatal("replay file decoding error: ", err)
		}
		i += 1

        if (time.Now().UnixNano() - currentTime) > time.Second.Nanoseconds() {
            currentTime = time.Now().UnixNano()
            log.Printf("Input RPS: %d", currentRPS)
            currentRPS = 0
        }
        currentRPS += 1

		go func() {
			if request, err := ParseRequest(msg); err != nil {
				Debug("Error while parsing request", err, msg)
			} else {
				Debug("Adding request", request)
				rf.Add(request)
			}
		}()
	}
	log.Println("read a total of ", i, "requests from file")
}

func handleConnection(conn net.Conn, rf *RequestFactory) error {
	defer conn.Close()

	var read = true
	var response []byte
	var buf []byte

	buf = make([]byte, bufSize)

	for read {
		n, err := conn.Read(buf)

		switch err {
		case io.EOF:
			read = false
		case nil:
			response = append(response, buf[:n]...)
			if n < bufSize {
				read = false
			}
		default:
			read = false
		}
	}

	go func() {
		if request, err := ParseRequest(response); err != nil {
			Debug("Error while parsing request", err, response)
		} else {
			Debug("Adding request", request)

			rf.Add(request)
		}
	}()

	return nil
}

func handlePersistentConnection(conn net.Conn, rf *RequestFactory) error {
	defer conn.Close()

	var buf []byte

	decoder := gob.NewDecoder(conn)

	currentTime := time.Now().UnixNano()
	currentRPS := 0


	for {
		err := decoder.Decode(&buf)
		if err == io.EOF {
			return nil
		} else if err != nil {
			Debug("Gob decode error: %s", err)
			return err
		}

        if (time.Now().UnixNano() - currentTime) > time.Second.Nanoseconds() {
            currentTime = time.Now().UnixNano()
            log.Printf("Input RPS: %d", currentRPS)
            currentRPS = 0
        }
        currentRPS += 1

		go func() {
			if request, err := ParseRequest(buf); err != nil {
				Debug("Error while parsing request", err, buf)
			} else {
				Debug("Adding request", request)

				rf.Add(request)
			}
		}()
	}

	return nil
}
