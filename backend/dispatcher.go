package backend

import (
	"encoding/json"
	. "github.com/codepr/narwhal/agent/"
	"log"
	"net/http"
	"net/url"
	"path"
	"time"
)

type Dispatcher struct {
	commitQueue       string
	runners           []RunnerProxy
	heartbeatInterval time.Duration
}

func (d *Dispatcher) probeRunner(proxyChan <-chan *RunnerProxy, stopChan <-chan interface{}) {
	for {
		select {
		case proxy := <-proxyChan:
			url, _ := url.Parse(proxy.Url)
			urlPath := path.Join(url.Path, proxy.HealthPath)
			res, err := http.Get(urlPath)
			if err != nil {
				proxy.Alive = false
				continue
			}
			if res.StatusCode == 200 {
				proxy.Alive = true
			}
		case <-stopChan:
			break
		}
	}
}

func (d *Dispatcher) Consume() {
	mq := NewAmqpQueue("amqp://guest:guest@localhost:5672/", "commits")
	events := make(chan []byte)
	proxies := make(chan *RunnerProxy)
	stop := make(chan interface{})

	// Create a pool of healthcheck goroutines
	for range d.runners {
		go d.probeRunner(proxies, stop)
	}

	// Spawn a goroutine to periodically heartbeat on the healthcheck endpoints
	go func() {
		for {
			for _, runner := range d.runners {
				proxies <- &runner
			}
			time.Sleep(d.heartbeatInterval * time.Millisecond)
		}
	}()

	for _, runner := range d.runners {
		go func(runner *RunnerProxy) {
			for {
				event := <-events
				var commit Commit
				err := json.Unmarshal(event, &commit)
				if err != nil {
					log.Println("Error decoding commit event")
				} else {
					// push job to runner through runnerproxy
				}
			}
		}(&runner)
	}

	mq.Consume(events)
}
