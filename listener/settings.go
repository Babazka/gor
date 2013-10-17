package listener

import (
	"flag"
	"os"
	"strconv"
	"strings"
)

const (
	defaultPort    = 80
	defaultAddress = "0.0.0.0"

	defaultReplayAddress = "localhost:28020"

	defaultPoolSize = 0

	defaultDevice = "lo"
)

// ListenerSettings contain all the needed configuration for setting up the listener
type ListenerSettings struct {
	Filter string
	Address string

	ReplayAddress string
	ReplayAddressRaw string

	RecordFile string

	ReplayLimit int
	PacketLimit int

	Multiply int

	DieOnConnectionError bool

	Verbose bool
	Dgram bool
	NoReassembly bool
	NoReplay bool

	PoolSize int

	Device string
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

	/*flag.IntVar(&Settings.Port, "p", defaultPort, "Specify the http server port whose traffic you want to capture")*/
	/*flag.StringVar(&Settings.Address, "ip", defaultAddress, "Specify IP address to listen")*/

	flag.StringVar(&Settings.Device, "i", defaultDevice, "Specify network interface to listen to")

	flag.StringVar(&Settings.RecordFile, "record-file", "", "file to store sniffed traffic in")

	flag.StringVar(&Settings.Filter, "pcap-filter", "tcp dst port 80", "Specify libpcap filter")

	//replayAddress := flag.String("r", defaultReplayAddress, "Address of replay server.")
	//Settings.ReplayServer(*replayAddress)

	flag.IntVar(&Settings.Multiply, "multiply", 1, "traffic multiplier")

	flag.StringVar(&Settings.ReplayAddressRaw, "r", defaultReplayAddress, "Address of replay server.")

	flag.BoolVar(&Settings.Verbose, "verbose", false, "Log requests")

	flag.BoolVar(&Settings.Dgram, "dgram", false, "use datagram socket")

	flag.BoolVar(&Settings.NoReplay, "noreplay", false, "do not send requests to replay server")

	flag.BoolVar(&Settings.DieOnConnectionError, "die-if-replay-server-is-unreachable", false, "die if replay server is unreachable")

	flag.BoolVar(&Settings.NoReassembly, "no-reasm", false, "Do not reassemble TCP streams, catch only segments with payload starting with GET / POST")

	flag.IntVar(&Settings.PoolSize, "pool-size", defaultPoolSize, "Size of persistent connection pool (default: no pool)")

	flag.IntVar(&Settings.PacketLimit, "packet-limit", 0, "Limiting parsed network packets per second")
}
