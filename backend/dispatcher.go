// BSD 2-Clause License
//
// Copyright (c) 2020, Andrea Giacomo Baldan
// All rights reserved.
//
// Redistribution and use in source and binary forms, with or without
// modification, are permitted provided that the following conditions are met:
//
// * Redistributions of source code must retain the above copyright notice, this
//   list of conditions and the following disclaimer.
//
// * Redistributions in binary form must reproduce the above copyright notice,
//   this list of conditions and the following disclaimer in the documentation
//   and/or other materials provided with the distribution.
//
// THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS"
// AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE
// IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE ARE
// DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT HOLDER OR CONTRIBUTORS BE LIABLE
// FOR ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR CONSEQUENTIAL
// DAMAGES (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR
// SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER
// CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY,
// OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE
// OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.

package backend

import (
	"encoding/json"
	. "github.com/codepr/narwhal/middleware"
	"log"
	"net/rpc"
	"time"
)

type Dispatcher struct {
	commitQueue       string
	runners           []RunnerProxy
	heartbeatInterval time.Duration
}

func NewDispatcher(commitQueue string, interval time.Duration, runners []RunnerProxy) *Dispatcher {
	return &Dispatcher{commitQueue, runners, interval}
}

func (d *Dispatcher) probeRunner(proxyChan <-chan *RunnerProxy, stopChan <-chan interface{}) {
	for {
		select {
		case proxy := <-proxyChan:
			var req HeartBeatRequest
			var res HeartBeatResponse
			proxy.RpcClient.Call("Runner.HeartBeat", req, &res)
		case <-stopChan:
			break
		}
	}
}

func (d *Dispatcher) Consume() error {
	mq := NewAmqpQueue("amqp://guest:guest@localhost:5672/", "commits")
	events := make(chan []byte)
	proxies := make(chan *RunnerProxy)
	stop := make(chan interface{})

	// Create a pool of healthcheck goroutines
	for i, runner := range d.runners {
		if client, err := rpc.Dial("tcp", runner.Addr); err != nil {
			log.Printf("Unable to dial runner %s", runner.Addr)
		} else {
			d.runners[i].RpcClient = client
		}
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

	return mq.Consume(events)
}
