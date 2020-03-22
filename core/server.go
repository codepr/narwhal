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

// Server contains factory functions to create and run server components,
// currently 2 types of server are supported:
//
// - Dispatcher: register runners and accept commits and forward them to an
//               alive runner for processing tests and other instructions, only
//               if not already processed before (e.g. only newest commits
//               are elegible for processing)
// - TestRunner: run a pool of containers and accepts commits from the
//				 dispatcher, its responsibility is to handle execution of tests
//				 and other instructions, crashes and timeouts are to be
//				 expected
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
	// Start the server, listening on a host:port tuple
	Run() error
}

type DispatcherServer struct {
	// server is a pointer to a builtin library http.Server, listen on a
	// host:port tuple and expose some REST APIs
	server *http.Server

	// runnerPool is a pool of RunnerPool type, specifically a DispatcherServer
	// exptects them to be of type TestRunnerPool, representing remote servers
	// located by an URL
	runnerPool RunnerPool

	// healthcheck_ch is a channel that acts as a helm to managing a periodic
	// check on health status of each runner
	healthcheck_ch chan bool

	// healthcheck_timeout define the ticks for the healthcheck calls of each
	// runner
	healthcheck_timeout time.Duration
}

type RunnerServer struct {
	// server is a pointer to a builtin library http.Server, listen on a
	// host:port tuple and expose some REST APIs
	server *http.Server

	// runnerPool is a pool of RunnerPool type, specifically a DispatcherServer
	// exptects them to be of type ContainerRunnerPool, representing a pool of
	// docker containers to run tests inside. Meant to be pre-allocated.
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
