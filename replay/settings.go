package replay

import (
	"flag"
	"os"
	"strconv"
	"strings"
)

// ForwardHost where to forward requests
type ForwardHost struct {
	Url   string
	Limit int

	Stat *RequestStat

	Clients *ClientPool
}

// ReplaySettings ListenerSettings contain all the needed configuration for setting up the replay
type ReplaySettings struct {
	Host string

	Address        string
	Unix        string
	ForwardAddress string

	Cpuprofile string

	ReplayFile string

	Verbose bool

	PersistentConnections bool

	ClientPoolSize int
}

var Settings ReplaySettings = ReplaySettings{}

// ForwardedHosts implements forwardAddress syntax support for multiple hosts (coma separated), and rate limiting by specifing "|maxRps" after host name.
//
//    -f "host1,http://host2|10,host3"
//
func (r *ReplaySettings) ForwardedHosts() (hosts []*ForwardHost) {
	hosts = make([]*ForwardHost, 0, 10)

	for _, address := range strings.Split(r.ForwardAddress, ",") {
		host_info := strings.Split(address, "|")

		if strings.Index(host_info[0], "http") == -1 {
			host_info[0] = "http://" + host_info[0]
		}

		host := &ForwardHost{Url: host_info[0]}
		host.Stat = NewRequestStats(host)

		if r.ClientPoolSize > 0 {
			host.Clients = NewClientPool(r.ClientPoolSize)
		}

		if len(host_info) > 1 {
			host.Limit, _ = strconv.Atoi(host_info[1])
		}

		hosts = append(hosts, host)
	}

	return
}

func init() {
	if len(os.Args) < 2 || os.Args[1] != "replay" {
		return
	}

	const (
		defaultHost = "0.0.0.0:28020"

		defaultForwardAddress = "http://localhost:8080"

		defaultClientPoolSize = 0
	)

	flag.StringVar(&Settings.Address, "ip", defaultHost, "ip addresses and port (or unix socket) to listen on")

	flag.StringVar(&Settings.ReplayFile, "replay-file", "", "file with recorded traffic to replay (instead of listening on a socket) ")

	flag.StringVar(&Settings.Cpuprofile, "cpuprofile", "", "file to record cpuprofile data")

	flag.StringVar(&Settings.ForwardAddress, "f", defaultForwardAddress, "http address to forward traffic.\n\tYou can limit requests per second by adding `|num` after address.\n\tIf you have multiple addresses with different limits. For example: http://staging.example.com|100,http://dev.example.com|10")

	flag.BoolVar(&Settings.Verbose, "verbose", false, "Log requests")

	flag.BoolVar(&Settings.PersistentConnections, "persistent-connections", false, "Set this option to use together with connection pool in listen servers")

	flag.IntVar(&Settings.ClientPoolSize, "client-pool-size", defaultClientPoolSize, "size of a pool of connections to forward servers (default: no pool, open a connection per request).\n\tUsing a pool allows the usage of HTTP keep-alive when possible.")
}
