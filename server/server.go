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
package server

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/codepr/narwhal/runner"
	"log"
	"net"
	"net/http"
	"net/rpc"
	"os"
	"os/signal"
	"syscall"
	"time"
)

const (
	Dispatcher = iota
	TestRunner
)

type Server interface {
	// Start the server, listening on a host:port tuple
	Run() error
}

func RunServer(s Server) error {
	return s.Run()
}

type DispatcherServer struct {
	// server is a pointer to a builtin library http.Server, listen on a
	// host:port tuple and expose some REST APIs
	server *http.Server
	// Registry just tracks and manage the runner units, each one representing
	// remote servers located by an URL
	//registry *RunnerRegistry
}

type RunnerServer struct {
	addr          string
	dispatcherUrl string
	quit          chan interface{}
	// RPC server ref, act as the transport layer
	rpcServer *rpc.Server
}

func newDispatcherRouter(r *runner.RunnerRegistry) *http.ServeMux {
	router := http.NewServeMux()
	router.Handle("/runner", handleDispatcherRunner(r))
	router.Handle("/commit", handleDispatcherCommit(r))
	return router
}

// Factory function, return a Server instance based on serverType argument
func NewDispatcherServer(addr string, l *log.Logger,
	r *runner.RunnerRegistry) *DispatcherServer {
	return &DispatcherServer{
		server: &http.Server{
			Addr:           addr,
			Handler:        logReq(l)(newDispatcherRouter(r)),
			ErrorLog:       l,
			ReadTimeout:    5 * time.Second,
			WriteTimeout:   10 * time.Second,
			IdleTimeout:    30 * time.Second,
			MaxHeaderBytes: 1 << 20,
		},
	}
}

func NewRunnerServer(addr, dispatcherUrl string) *RunnerServer {
	return &RunnerServer{
		addr:          addr,
		dispatcherUrl: dispatcherUrl,
		rpcServer:     rpc.NewServer(),
	}
}

func (s *DispatcherServer) Run() error {
	done := make(chan bool)
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	// s.registry.Start()

	go func() {
		<-quit
		s.server.ErrorLog.Println("Shutdown")
		// Stop push pushCommit goroutine
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

func (s *RunnerServer) Run() error {
	done := make(chan interface{})
	listener, err := net.Listen("tcp", s.addr)
	runnerProxy := &runner.RunnerProxy{Addr: listener.Addr().String()}
	s.rpcServer.RegisterName("RunnerProxy", runnerProxy)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("Listening on %v\n", listener.Addr())

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				select {
				case <-s.quit:
					listener.Close()
					close(done)
					return
				default:
					log.Fatal(err)
				}
			}
			log.Print("Connection accepted")
			go func() {
				s.rpcServer.ServeConn(conn)
			}()
		}
	}()

	// Register to a dispatcher
	registerBody, err := json.Marshal(map[string]string{
		"addr": "127.0.0.1:28918",
	})
	if err != nil {
		log.Println(err)
	}
	resp, err := http.Post(s.dispatcherUrl,
		"application/json", bytes.NewBuffer(registerBody))
	if err != nil {
		log.Println(err)
	}
	resp.Body.Close()
	<-done
	return nil
}
