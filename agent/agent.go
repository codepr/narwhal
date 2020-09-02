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

package agent

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"

	. "github.com/codepr/narwhal/backend"
	. "github.com/codepr/narwhal/internal"
)

type Agent struct {
	server      *http.Server
	commitQueue string
}

func NewAgent(commitQueue string) *Agent {
	return &Agent{
		server:      nil,
		commitQueue: commitQueue,
	}
}

func (a *Agent) Run() {

	// For now we just store our subscribers into a slice
	logger := log.New(os.Stdout, "agent: ", log.LstdFlags)
	logger.Println("Agent is starting...")

	mq := NewAmqpQueue("amqp://guest:guest@localhost:5672/", "commits")

	events := make(chan Commit)

	go func() {
		for {
			event := <-events
			payload, err := json.Marshal(event)
			if err != nil {
				logger.Println("Error encoding event")
				continue
			}
			if err := mq.Produce(payload); err != nil {
				logger.Println("Error producing event to queue")
			}
		}
	}()

	// Setup 2 HTTP routes
	router := http.NewServeMux()
	router.Handle("/health", healthCheckHandler())
	router.Handle("/commit", commitHandler(events))

	server := &http.Server{
		Addr:         ":9797",
		Handler:      logging(logger)(router),
		ErrorLog:     logger,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  15 * time.Second,
	}

	done := make(chan bool)
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt)

	// Setup a graceful shutdown goroutine waiting for a CTRL+C signal
	go func() {
		<-quit
		logger.Println("Agent is shutting down...")

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		server.SetKeepAlivesEnabled(false)
		if err := server.Shutdown(ctx); err != nil {
			logger.Fatalf("Could not gracefully shutdown the agent: %v\n", err)
		}
		close(done)
	}()

	logger.Println("Agent is ready to handle requests at :9797")
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Fatalf("Could not listen on :9797: %v\n", err)
	}

	<-done
	logger.Println("Agent stopped")
}
