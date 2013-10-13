// Gor is simple http traffic replication tool written in Go. Its main goal to replay traffic from production servers to staging and dev environments.
// Now you can test your code on real user sessions in an automated and repeatable fashion.
//
// Gor consists of 2 parts: listener and replay servers.
// Listener catch http traffic from given port in real-time and send it to replay server via UDP. Replay server forwards traffic to given address.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"runtime/pprof"
	"time"

	"github.com/Babazka/gor/listener"
	"github.com/Babazka/gor/replay"
	"github.com/Babazka/gor/statsd"
)

const (
	VERSION = "0.3.5"
)

var (
	mode       string
	cpuprofile = flag.String("gen-cpuprofile", "", "write cpu profile to file")
	memprofile = flag.String("memprofile", "", "write memory profile to this file")

	statsd_address = flag.String("statsd", "", "host:port of statsd")
	statsd_prefix = flag.String("statsd-prefix", "", "additional statsd prefix (placed after the default which is 'gor.hostname.mode')")
)

func main() {
	defer func() {
		if r := recover(); r != nil {
			if _, ok := r.(error); !ok {
				fmt.Errorf("pkg: %v", r)
			}
		}
	}()

	fmt.Println("Version:", VERSION)

	if len(os.Args) > 1 {
		mode = os.Args[1]
	}

	if mode != "listen" && mode != "replay" {
		fmt.Println("Usage: \n\tgor listen -h\n\tgor replay -h")
		return
	}

	// Remove mode attr
	os.Args = append(os.Args[:1], os.Args[2:]...)

	flag.Parse()

	if *cpuprofile != "" {
		f, err := os.Create(*cpuprofile)
		if err != nil {
			log.Fatal(err)
		}
		pprof.StartCPUProfile(f)

		time.AfterFunc(60*time.Second, func() {
			pprof.StopCPUProfile()
			f.Close()
			log.Println("Stop profiling after 60 seconds")
		})
	}

	if *memprofile != "" {
		f, err := os.Create(*memprofile)
		if err != nil {
			log.Fatal(err)
		}
		time.AfterFunc(60*time.Second, func() {
			pprof.WriteHeapProfile(f)
			f.Close()
		})
	}

	if *statsd_address != "" {
		prefix := makeStatsdPrefix(mode)
		client, err := statsd.Dial(*statsd_address, prefix)
		if err != nil {
			log.Fatal("While connecting to statsd: ", err)
		}
		statsd.C = client
	}

	switch mode {
	case "listen":
		listener.Run()
	case "replay":
		replay.Run()
	}
}

func makeStatsdPrefix(mode string) string {
	prefix := "gor."
	hostname, err := os.Hostname()
	if err != nil {
		log.Fatal("Cannot get hostname: ", err)
	}
	prefix = prefix + hostname + "." + mode
	if *statsd_prefix != "" {
		prefix = prefix + "-" + *statsd_prefix
	}
	return prefix
}
