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

package core

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

const (
	Dispatcher = iota
	TestRunner = iota
)

type Server interface {
	// Start the server
	Run() error
}

type DispatcherServer struct {
	server              *http.Server
	runnerPool          RunnerPool
	healthcheck_ch      chan bool
	healthcheck_timeout time.Duration
}

type RunnerServer struct {
	server     *http.Server
	runnerPool RunnerPool
}

func dispatcherNewRouter(r RunnerPool) *http.ServeMux {
	router := http.NewServeMux()
	router.Handle("/runner", handleTestRunner(r))
	router.Handle("/commit", handleCommit(r))
	return router
}

// Factory function, return a Server instance based on serverType argument
func NewServer(addr string, l *log.Logger,
	r RunnerPool, ts time.Duration, serverType int) Server {
	switch serverType {
	case Dispatcher:
		return &DispatcherServer{
			server: &http.Server{
				Addr:           addr,
				Handler:        logReq(l)(dispatcherNewRouter(r)),
				ErrorLog:       l,
				ReadTimeout:    5 * time.Second,
				WriteTimeout:   10 * time.Second,
				IdleTimeout:    30 * time.Second,
				MaxHeaderBytes: 1 << 20,
			},
			runnerPool:          r,
			healthcheck_timeout: ts,
			healthcheck_ch:      make(chan bool),
		}
	case TestRunner:
		return &RunnerServer{
			server: &http.Server{
				Addr:           addr,
				Handler:        logReq(l)(dispatcherNewRouter(r)),
				ErrorLog:       l,
				ReadTimeout:    5 * time.Second,
				WriteTimeout:   10 * time.Second,
				IdleTimeout:    30 * time.Second,
				MaxHeaderBytes: 1 << 20,
			},
			runnerPool: r,
		}
	}
	return nil
}

func (s *DispatcherServer) Run() error {
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

func (s *RunnerServer) Run() error {
	done := make(chan bool)
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-quit
		s.server.ErrorLog.Println("Shutdown")
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

	s.server.ErrorLog.Println("Listening on", s.server.Addr)
	if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		s.server.ErrorLog.Println("Unable to bind on", s.server.Addr)
	}

	<-done
	return nil
}

func (s *DispatcherServer) runnersHealthcheck() {
	ticker := time.NewTicker(s.healthcheck_timeout * time.Second)
	for {
		select {
		case <-s.healthcheck_ch:
			ticker.Stop()
			return
		case <-ticker.C:
			s.runnerPool.HealthCheck()
		}
	}
}
