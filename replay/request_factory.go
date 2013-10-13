package replay

import (
	"errors"
	"net/http"
	"net/url"
	"io/ioutil"
	"sync"
	"time"
	"log"

	"github.com/Babazka/gor/statsd"
)

// HttpResponse contains a host, a http request,
// a http response and an error
type HttpResponse struct {
	host *ForwardHost
	req  *http.Request
	resp *http.Response
	err  error
}


// RequestFactory processes requests
//
// Basic workflow:
//
// 1. When request added via Add() it get pushed to `responses` chan
// 2. handleRequest() listen for `responses` chan and decide where request should be forwarded, and apply rate-limit if needed
// 3. sendRequest() forwards request and returns response info to `responses` chan
// 4. handleRequest() listen for `response` channel and updates stats
type RequestFactory struct {
	c_responses chan *HttpResponse
	c_requests  chan *http.Request
	hosts []*ForwardHost
	dropped_requests int
}

// NewRequestFactory returns a RequestFactory pointer
// One created, it starts listening for incoming requests: requests channel
func NewRequestFactory() (factory *RequestFactory) {
	factory = &RequestFactory{}
	factory.c_responses = make(chan *HttpResponse, 1)
	factory.c_requests = make(chan *http.Request, Settings.BacklogSize)

	factory.hosts = Settings.ForwardedHosts()

	factory.dropped_requests = 0

	if Settings.UseWorkers {
		log.Printf("starting workers")
		for i := 0; i < Settings.ClientPoolSize; i++ {
			go factory.Worker(i)
		}
	} else {
		log.Printf("starting single handleRequests loop")
		go factory.handleRequests()
	}

	go factory.BacklogMonitor()

	return
}

func (f *RequestFactory) BacklogMonitor() {
	tick := time.Tick(1 * time.Second)
	for {
		<-tick
		dropped := f.dropped_requests
		// race condition, but fuck it, it's statistics
		f.dropped_requests = 0
		log.Printf("Backlog size %d; dropped %d reqs last second", len(f.c_requests), dropped)
		statsd.C.Inc("dropped", dropped, 1.0)
		statsd.C.Inc("backlog", len(f.c_requests), 1.0)
	}
}

// customCheckRedirect disables redirects https://github.com/buger/gor/pull/15
func customCheckRedirect(req *http.Request, via []*http.Request) error {
	if len(via) >= 0 {
		return errors.New("stopped after 2 redirects")
	}
	return nil
}

func (f *RequestFactory) sendRequest(host *ForwardHost, request *http.Request) {
	f.sendRequestUsingSpecificConnection(host, request, -1)
}

// sendRequest forwards http request to a given host
func (f *RequestFactory) sendRequestUsingSpecificConnection(host *ForwardHost, request *http.Request, connection_i int) {
	var client *http.Client

	if host.Clients != nil {
		if connection_i == -1 {
			var client_lock *sync.Mutex
			client, client_lock = host.Clients.GetClient()
			client_lock.Lock() // Prevent other goroutines from using this connection.
							   // http.Client is goroutine-safe, but in its own way:
							   // when used by 2+ goroutines, it will create new TCP
							   // connections, not block.
							   // We do not want this (that's why we created our own pool
							   // in the first place).
			defer client_lock.Unlock()
		} else {
			// no need for locking, each worker gets its own connection
			client = host.Clients.GetIthClient(connection_i)
		}

		// reuse a connection
		// original sniffed request may contain Connection: close, we do not want it
		request.Header.Del("Connection") // default for HTTP/1.1 assumed keep-alive
	} else {
		client = &http.Client{
			CheckRedirect: customCheckRedirect,
		}
	}

	// Change HOST of original request
	URL := host.Url + request.URL.Path + "?" + request.URL.RawQuery

	request.RequestURI = ""
	request.URL, _ = url.ParseRequestURI(URL)

	Debug("Sending request:", host.Url, request)

	resp, err := client.Do(request)

	if err == nil {
		defer resp.Body.Close()
		if host.Clients != nil {
			// we must read the response completely to make keep-alive work
			// (partial reads don't seem to work, contrary to the documentation)
			ioutil.ReadAll(resp.Body)
		}
	} else {
		Debug("Request error:", err)
	}

	//f.c_responses <- &HttpResponse{host, request, resp, err}
}

func (f *RequestFactory) Worker(worker_id int) {
	hosts := f.hosts

	tick := time.Tick(1 * time.Second)

    rps := 0

	for {
		select {
		case <-tick:
            log.Printf("Worker %d average RPS: %d", worker_id, rps / 1)
			statsd.C.Inc("worker_output", rps, 1.0)

            rps = 0
		case req := <-f.c_requests:
            rps += 1
			if len(hosts) < 1 {
				continue
			}
			host := hosts[0]
			// Ensure that we have actual stats for given timestamp
			host.Stat.Touch()

			if host.Limit == 0 || host.Stat.Count < host.Limit {
				// Increment Stat.Count
				host.Stat.IncReq()

				f.sendRequestUsingSpecificConnection(host, req, worker_id)
			}
		case resp := <-f.c_responses:
			// Increment returned http code stats, and elapsed time
			resp.host.Stat.IncResp(resp)
		}
	}
}

// handleRequests and their responses
func (f *RequestFactory) handleRequests() {
	hosts := f.hosts

	tick := time.Tick(1 * time.Second)

    rps := 0

	for {
		select {
		case <-tick:
            log.Printf("RF average RPS: %d, backlog len %d", rps / 1, len(f.c_requests))
            rps = 0
		case req := <-f.c_requests:
            rps += 1
			if len(hosts) < 1 {
				continue
			}
			host := hosts[0]
			// Ensure that we have actual stats for given timestamp
			host.Stat.Touch()

			if host.Limit == 0 || host.Stat.Count < host.Limit {
				// Increment Stat.Count
				host.Stat.IncReq()

				go f.sendRequest(host, req)
			}
		case resp := <-f.c_responses:
			// Increment returned http code stats, and elapsed time
			resp.host.Stat.IncResp(resp)
		}
	}
}

// Add request to channel for further processing
func (f *RequestFactory) Add(request *http.Request) {
	if len(f.c_requests) > Settings.BacklogSize - 100 {
		f.dropped_requests += 1
	} else {
		f.c_requests <- request
	}
}

/*
func (f *RequestFactory) Add(request *http.Request) {
	if len(f.hosts) < 1 {
		return
	}
	host := f.hosts[0]
	go f.sendRequest(host, request)
}
*/
