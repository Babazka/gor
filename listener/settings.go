package listener

import (
	"flag"
	"os"
	"strconv"
	"github.com/Babazka/gor/replay"
	"strings"
)

const (
	defaultPort    = 80
	defaultAddress = "0.0.0.0"

	defaultReplayAddress = "localhost:28020"

	defaultPoolSize = 0

	defaultClientPoolSize = 0

	defaultDevice = "lo"
)

// ListenerSettings contain all the needed configuration for setting up the listener
type ListenerSettings struct {
	Port    int
	Address string

	ReplayAddress string
	ReplayAddressRaw string

	ReplayLimit int

	Verbose bool

	PoolSize int

	Device string

	ForwardAddress string

	ClientPoolSize int
}

var Settings ListenerSettings = ListenerSettings{}

// ReplayServer generates ReplayLimit and ReplayAddress settings out of the replayAddress
func (s *ListenerSettings) ReplayServer(replayAddress string) {
	host_info := strings.Split(replayAddress, "|")

	if len(host_info) > 1 {
		s.ReplayLimit, _ = strconv.Atoi(host_info[1])
	}

	s.ReplayAddress = host_info[0]
}

func init() {
	if len(os.Args) < 2 || os.Args[1] != "listen" {
		return
	}

	flag.IntVar(&Settings.Port, "p", defaultPort, "Specify the http server port whose traffic you want to capture")
	/*flag.StringVar(&Settings.Address, "ip", defaultAddress, "Specify IP address to listen")*/

	flag.StringVar(&Settings.Device, "i", defaultDevice, "Specify network interface to listen to")

	//replayAddress := flag.String("r", defaultReplayAddress, "Address of replay server.")
	//Settings.ReplayServer(*replayAddress)

	flag.StringVar(&Settings.ReplayAddressRaw, "r", defaultReplayAddress, "Address of replay server.")

	flag.BoolVar(&Settings.Verbose, "verbose", false, "Log requests")

	flag.IntVar(&Settings.PoolSize, "pool-size", defaultPoolSize, "Size of persistent connection pool (default: no pool)")

	flag.StringVar(&replay.Settings.ForwardAddress, "rf", "", "http address to forward traffic.\n\tYou can limit requests per second by adding `|num` after address.\n\tIf you have multiple addresses with different limits. For example: http://staging.example.com|100,http://dev.example.com|10")

	flag.IntVar(&replay.Settings.ClientPoolSize, "r-client-pool-size", defaultClientPoolSize, "size of a pool of connections to forward servers (default: no pool, open a connection per request).\n\tUsing a pool allows the usage of HTTP keep-alive when possible.")
}
