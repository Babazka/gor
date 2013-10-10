package replay

import (
	"errors"
	"net/http"
	"net/url"
	"io/ioutil"
	"time"
	"log"
)

const (
	WORKER_CHAN_SIZE = 4000
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
	requests chan *http.Request
}

// NewRequestFactory returns a RequestFactory pointer
// One created, it starts listening for incoming requests: requests channel
func NewRequestFactory() (factory *RequestFactory) {
	factory = &RequestFactory{}
	/*factory.c_responses = make(chan *HttpResponse, 1)*/
	/*factory.c_requests = make(chan *http.Request, 100)*/
	n_workers := Settings.ClientPoolSize
	if n_workers == 0 {
		n_workers = 1
	}

	factory.requests = make(chan *http.Request, WORKER_CHAN_SIZE)

	for i := 0; i < n_workers; i++ {
		go factory.Worker(i, factory.requests)
	}

	/*go factory.handleRequests()*/

	return
}

// Add request to channel for further processing
func (f *RequestFactory) Add(request *http.Request) {
	f.requests <- request

}

func (f *RequestFactory) Worker(wnum int, requests chan *http.Request) {
	hosts := Settings.ForwardedHosts()

	client := &http.Client{
		CheckRedirect: customCheckRedirect,
	}

	tick := time.Tick(10 * time.Second)

    rps := 0

	for {
		select {
		case <-tick:
            log.Printf("W%d average RPS: %d, qlen %d", wnum, rps / 10, len(requests))
            rps = 0
		case request := <-requests:
			rps += 1
			// Change HOST of original request
			if len(hosts) < 1 {
				continue
			}
			host := hosts[0]
			/*
			// Ensure that we have actual stats for given timestamp
			host.Stat.Touch()

			if !(host.Limit == 0 || host.Stat.Count < host.Limit) {
				continue
			}

			// Increment Stat.Count
			host.Stat.IncReq()
			*/
			URL := host.Url + request.URL.Path + "?" + request.URL.RawQuery

			request.RequestURI = ""
			request.URL, _ = url.ParseRequestURI(URL)

			Debug("Sending request:", host.Url, request)

			resp, err := client.Do(request)

			if err == nil {
				if host.Clients != nil {
					// we must read the response completely to make keep-alive work
					// (partial reads don't seem to work, contrary to the documentation)
					ioutil.ReadAll(resp.Body)
				}
				resp.Body.Close()
			} else {
				Debug("Request error:", err)
			}
		}
	}
}

// customCheckRedirect disables redirects https://github.com/buger/gor/pull/15
func customCheckRedirect(req *http.Request, via []*http.Request) error {
	if len(via) >= 0 {
		return errors.New("stopped after 2 redirects")
	}
	return nil
}

/*
// sendRequest forwards http request to a given host
func (f *RequestFactory) sendRequest(host *ForwardHost, request *http.Request) {
	var client *http.Client

	if host.Clients != nil {
		// reuse a connection
		var client_lock *sync.Mutex
		client, client_lock = host.Clients.GetClient()
		client_lock.Lock() // Prevent other goroutines from using this connection.
						   // http.Client is goroutine-safe, but in its own way:
						   // when used by 2+ goroutines, it will create new TCP
						   // connections, not block.
						   // We do not want this (that's why we created our own pool
						   // in the first place).
		defer client_lock.Unlock()
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
*/

// handleRequests and their responses
/*
func (f *RequestFactory) handleRequests() {
	hosts := Settings.ForwardedHosts()

	tick := time.Tick(10 * time.Second)

    rps := 0

	for {
		select {
		case <-tick:
            log.Printf("RF average RPS: %d", rps / 10)
            rps = 0
		case req := <-f.c_requests:
            rps += 1
			for _, host := range hosts {
				// Ensure that we have actual stats for given timestamp
				host.Stat.Touch()

				if host.Limit == 0 || host.Stat.Count < host.Limit {
					// Increment Stat.Count
					host.Stat.IncReq()

					go f.sendRequest(host, req)
				}
			}
		case resp := <-f.c_responses:
			// Increment returned http code stats, and elapsed time
			resp.host.Stat.IncResp(resp)
		}
	}
}
*/

