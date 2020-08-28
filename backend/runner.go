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
	"log"
	"net"
	"net/rpc"
)

type RunnerRequest struct {
	Commit Commit
}

type RunnerResponse struct {
	Response string
}

type HeartBeatRequest struct{}

type HeartBeatResponse struct {
	Alive bool
}

type Runner struct{}

func (r *Runner) HeartBeat(req HeartBeatRequest, res *HeartBeatResponse) error {
	res.Alive = true
	return nil
}

func (r *Runner) RunCommitJob(req RunnerRequest, res *RunnerResponse) error {
	res.Response = "OK"
	return nil
}

func StartRunner(addr string) error {
	quit := make(chan interface{})
	done := make(chan interface{})
	listener, err := net.Listen("tcp", addr)
	runnerProxy := &Runner{}
	rpcServer := rpc.NewServer()

	// Publish Runner proxy object
	rpcServer.RegisterName("Runner", runnerProxy)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("Listening on %v\n", listener.Addr())

	// Wait for incoming connections
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				select {
				case <-quit:
					listener.Close()
					close(done)
					return
				default:
					log.Fatal(err)
				}
			}
			log.Print("Connection accepted")
			go func() {
				rpcServer.ServeConn(conn)
			}()
		}
	}()

	<-done
	return nil
}
