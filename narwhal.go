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

package main

import (
	"flag"
	"github.com/codepr/narwhal/runner"
	core "github.com/codepr/narwhal/server"
	"log"
	"os"
)

var (
	addr, dispatcherUrl string
	serverType          int
)

func main() {
	flag.StringVar(&addr, "addr", ":28919", "Server listening address")
	flag.IntVar(&serverType, "type", core.Dispatcher,
		"Server type, can be either 0 (Dispatcher) or 1 (Runner)")
	flag.StringVar(&dispatcherUrl, "dispatcher",
		"http://localhost:28919/runner", "Dispatcher URL")
	flag.Parse()

	if serverType < 0 || serverType > 1 {
		log.Fatal("Server type not supported")
	}

	var prefix string = "[dispatcher] "
	if serverType == core.TestRunner {
		prefix = "[runner] "
	}
	var server core.Server
	logger := log.New(os.Stdout, prefix, log.LstdFlags)
	if serverType == core.Dispatcher {
		runnerPool := runner.NewRunnerRegistry(logger)
		server = core.NewDispatcherServer(addr, logger, runnerPool)
	} else {
		server = core.NewRunnerServer(addr, dispatcherUrl)
	}

	if err := core.RunServer(server); err != nil {
		logger.Fatal(err)
	}
}
