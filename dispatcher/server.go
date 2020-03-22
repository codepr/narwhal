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

package dispatcher

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

type Server struct {
	server              *http.Server
	runnerPool          *TestRunnerPool
	healthcheck_ch      chan bool
	healthcheck_timeout time.Duration
}

func newRouter(p *TestRunnerPool) *http.ServeMux {
	router := http.NewServeMux()
	router.Handle("/testrunner", handleTestRunner(p))
	router.Handle("/commit", handleCommit(p))
	return router
}

func NewServer(addr string, l *log.Logger,
	p *TestRunnerPool, ts time.Duration) *Server {
	return &Server{
		server: &http.Server{
			Addr:           addr,
			Handler:        logReq(l)(newRouter(p)),
			ErrorLog:       l,
			ReadTimeout:    5 * time.Second,
			WriteTimeout:   10 * time.Second,
			IdleTimeout:    30 * time.Second,
			MaxHeaderBytes: 1 << 20,
		},
		runnerPool:          p,
		healthcheck_timeout: ts,
		healthcheck_ch:      make(chan bool),
	}
}

func (s *Server) Run() error {
	done := make(chan bool)
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-quit
		s.server.ErrorLog.Println("Shutdown")
		// Stop the healthcheck goroutine
		s.healthcheck_ch <- true
		// Stop push pushCommit goroutine
		s.runnerPool.Stop()
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		s.server.SetKeepAlivesEnabled(false)
		if err := s.server.Shutdown(ctx); err != nil {
			s.server.ErrorLog.Fatal("Could not shutdown the server")
		}
		close(done)
	}()

	// Start healthcheck goroutine
	go s.runnersHealthcheck()

	s.server.ErrorLog.Println("Listening on", s.server.Addr)
	if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		s.server.ErrorLog.Println("Unable to bind on", s.server.Addr)
	}

	<-done
	return nil
}

func (s *Server) runnersHealthcheck() {
	ticker := time.NewTicker(s.healthcheck_timeout * time.Second)
	for {
		select {
		case <-s.healthcheck_ch:
			ticker.Stop()
			return
		case <-ticker.C:
			for _, t := range s.runnerPool.runners {
				res, err := http.Get(t.URL + "/health")
				if err != nil || res.StatusCode != 200 {
					t.Alive = false
				}
			}
		}
	}
}
